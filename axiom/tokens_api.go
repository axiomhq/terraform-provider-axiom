package axiom

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
)

// The types and calls below mirror the token API of axiom-go and add the
// labels organisation capability, which the API supports but axiom-go does
// not have yet. Each type embeds its axiom-go counterpart and replaces the
// organisation capabilities, so the JSON is otherwise the same. They can be
// replaced by the axiom-go ones once those include labels.

const tokensAPIPath = "/v2/tokens"

type tokenOrgCapabilities struct {
	axiom.OrganisationCapabilities
	Labels []axiom.Action `json:"labels,omitempty"`
}

type createTokenRequest struct {
	axiom.CreateTokenRequest
	OrganisationCapabilities tokenOrgCapabilities `json:"orgCapabilities"`
}

type apiToken struct {
	axiom.APIToken
	OrganisationCapabilities tokenOrgCapabilities `json:"orgCapabilities"`
}

type createTokenResponse struct {
	apiToken
	Token string `json:"token"`
}

type regenerateTokenRequest struct {
	ExistingTokenExpiresAt time.Time           `json:"existingTokenExpiresAt"`
	NewToken               *createTokenRequest `json:"newToken,omitempty"`
}

func createToken(ctx context.Context, client *axiom.Client, req createTokenRequest) (*createTokenResponse, error) {
	var res createTokenResponse
	if err := client.Call(ctx, http.MethodPost, tokensAPIPath, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func getToken(ctx context.Context, client *axiom.Client, id string) (*apiToken, error) {
	path, err := url.JoinPath(tokensAPIPath, id)
	if err != nil {
		return nil, err
	}

	var res apiToken
	if err := client.Call(ctx, http.MethodGet, path, nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func regenerateToken(ctx context.Context, client *axiom.Client, id string, req regenerateTokenRequest) (*createTokenResponse, error) {
	path, err := url.JoinPath(tokensAPIPath, id, "regenerate")
	if err != nil {
		return nil, err
	}

	var res createTokenResponse
	if err := client.Call(ctx, http.MethodPost, path, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
