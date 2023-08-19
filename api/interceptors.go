package api

import (
	"context"
	"fmt"
	"runtime"

	"github.com/bufbuild/connect-go"
)

// Returns the user agent string for the CLI.
func Agent() string {
	return fmt.Sprintf("github.com/depot/depot-go/%s/%s", runtime.GOOS, runtime.GOARCH)
}

func WithUserAgent() connect.ClientOption {
	return connect.WithInterceptors(&agentInterceptor{Agent()})
}

type agentInterceptor struct {
	agent string
}

func (i *agentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("User-Agent", i.agent)
		return next(ctx, req)
	}
}

func (i *agentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("User-Agent", i.agent)
		return conn
	}
}

func (i *agentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
