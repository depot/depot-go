package actionspublic

import (
	"context"
)

type ActionsPublicProvider struct {
}

func NewActionsPublicProvider() *ActionsPublicProvider {
	return &ActionsPublicProvider{}
}

func (p *ActionsPublicProvider) Name() string {
	return "actions-public"
}

func (p *ActionsPublicProvider) RetrieveToken(ctx context.Context) (string, error) {
	token, err := RetrieveToken(ctx, "https://depot.dev")
	return token, err
}
