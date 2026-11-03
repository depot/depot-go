package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/depot/depot-go/api"
	corev1 "github.com/depot/depot-go/proto/depot/core/v1"
	"github.com/depot/depot-go/proto/depot/core/v1/corev1connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type billingServer struct {
	corev1connect.UnimplementedBillingServiceHandler
	t       *testing.T
	mu      sync.Mutex
	tokens  []string
	invoice *corev1.Invoice
}

func (s *billingServer) ListInvoices(_ context.Context, req *connect.Request[corev1.ListInvoicesRequest]) (*connect.Response[corev1.ListInvoicesResponse], error) {
	if req.Msg.PageSize != 2 {
		s.t.Errorf("page size = %d", req.Msg.PageSize)
	}
	s.mu.Lock()
	s.tokens = append(s.tokens, req.Msg.PageToken)
	s.mu.Unlock()
	if req.Msg.PageToken == "" {
		return connect.NewResponse(&corev1.ListInvoicesResponse{NextPageToken: "past-drafts"}), nil
	}
	if req.Msg.PageToken != "past-drafts" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Invalid page token"))
	}
	return connect.NewResponse(&corev1.ListInvoicesResponse{Invoices: []*corev1.Invoice{
		s.invoice, {Id: "in_previous", SubscriptionId: "sub_canceled"},
	}}), nil
}

func (s *billingServer) GetInvoice(_ context.Context, req *connect.Request[corev1.GetInvoiceRequest]) (*connect.Response[corev1.GetInvoiceResponse], error) {
	if req.Msg.InvoiceId != s.invoice.Id {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("Invoice not found"))
	}
	return connect.NewResponse(&corev1.GetInvoiceResponse{Invoice: s.invoice}), nil
}

func (s *billingServer) ListInvoiceLineItems(_ context.Context, req *connect.Request[corev1.ListInvoiceLineItemsRequest]) (*connect.Response[corev1.ListInvoiceLineItemsResponse], error) {
	if req.Msg.InvoiceId != s.invoice.Id || req.Msg.PageSize != 1 || req.Msg.PageToken != "line-page" {
		s.t.Errorf("unexpected line request: %v", req.Msg)
	}
	return connect.NewResponse(&corev1.ListInvoiceLineItemsResponse{
		LineItems:     []*corev1.InvoiceLineItem{{Id: "il_credit", Amount: -9007199254740993, Quantity: proto.Int64(0), UnitAmountDecimal: proto.String("0.0001")}},
		NextPageToken: "next-line",
	}), nil
}

func TestBillingClientInvoiceHistory(t *testing.T) {
	invoice := &corev1.Invoice{
		Id: "in_standalone", Status: "paid", Currency: "usd", Total: 9007199254740993,
		StartingBalance: -1500, SubtotalExcludingTax: proto.Int64(0),
		FinalizedAt:      timestamppb.New(time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)),
		HostedInvoiceUrl: proto.String("https://invoice.example.test/hosted"),
	}
	service := &billingServer{t: t, invoice: invoice}
	_, handler := corev1connect.NewBillingServiceHandler(service)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer organization-token" || r.Header.Get("User-Agent") == "" || r.Header.Get("X-Depot-Org") != "" {
			t.Error("missing authentication/user agent or unexpected organization override")
		}
		w.Header().Set("Cache-Control", "no-store")
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	t.Setenv("DEPOT_API_URL", server.URL)
	client := api.NewBillingClient()
	ctx := context.Background()

	first, err := client.ListInvoices(ctx, api.WithAuthentication(connect.NewRequest(&corev1.ListInvoicesRequest{PageSize: 2}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Msg.Invoices) != 0 || first.Msg.NextPageToken != "past-drafts" {
		t.Fatalf("unexpected empty page: %v", first.Msg)
	}
	second, err := client.ListInvoices(ctx, api.WithAuthentication(connect.NewRequest(&corev1.ListInvoicesRequest{PageSize: 2, PageToken: first.Msg.NextPageToken}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Msg.Invoices) != 2 || !proto.Equal(second.Msg.Invoices[0], invoice) || second.Msg.Invoices[1].SubscriptionId != "sub_canceled" || second.Msg.NextPageToken != "" {
		t.Fatalf("unexpected history: %v", second.Msg)
	}
	service.mu.Lock()
	tokens := append([]string(nil), service.tokens...)
	service.mu.Unlock()
	if !reflect.DeepEqual(tokens, []string{"", "past-drafts"}) {
		t.Fatalf("tokens = %v", service.tokens)
	}

	detail, err := client.GetInvoice(ctx, api.WithAuthentication(connect.NewRequest(&corev1.GetInvoiceRequest{InvoiceId: invoice.Id}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(detail.Msg.Invoice, invoice) || detail.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("invoice fields or response metadata changed: %v", detail.Msg)
	}
	lines, err := client.ListInvoiceLineItems(ctx, api.WithAuthentication(connect.NewRequest(&corev1.ListInvoiceLineItemsRequest{InvoiceId: invoice.Id, PageSize: 1, PageToken: "line-page"}), "organization-token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines.Msg.LineItems) != 1 || lines.Msg.LineItems[0].Amount != -9007199254740993 || lines.Msg.LineItems[0].Quantity == nil || lines.Msg.LineItems[0].GetQuantity() != 0 || lines.Msg.LineItems[0].GetUnitAmountDecimal() != "0.0001" || lines.Msg.NextPageToken != "next-line" {
		t.Fatalf("line item values changed: %v", lines.Msg)
	}
	_, err = client.GetInvoice(ctx, api.WithAuthentication(connect.NewRequest(&corev1.GetInvoiceRequest{InvoiceId: "in_foreign"}), "organization-token"))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	_, err = client.ListInvoices(ctx, api.WithAuthentication(connect.NewRequest(&corev1.ListInvoicesRequest{PageSize: 2, PageToken: "invalid"}), "organization-token"))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if calls.Load() != 6 {
		t.Fatalf("requests = %d; unexpected retry", calls.Load())
	}
}
