package actapro

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"go.artefactual.dev/tools/clientauth"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

type Client interface {
	gen.Invoker
}

func NewClient(
	config Config,
	httpClient *http.Client,
	tokenProvider clientauth.AccessTokenProvider,
) (Client, error) {
	if httpClient == nil {
		timeout := config.Timeout
		if timeout < 0 {
			return nil, fmt.Errorf("ACTApro.Timeout: value %s is less than 0", timeout)
		}

		httpClient = cleanhttp.DefaultPooledClient()
		httpClient.Timeout = timeout
	}

	return gen.NewClient(
		config.URL,
		securitySource{
			staticToken:   config.Token,
			tokenProvider: tokenProvider,
		},
		gen.WithClient(httpClient),
	)
}

type securitySource struct {
	staticToken   string
	tokenProvider clientauth.AccessTokenProvider
}

func (s securitySource) BearerAuth(ctx context.Context, _ gen.OperationName) (gen.BearerAuth, error) {
	if s.staticToken != "" {
		return gen.BearerAuth{Token: s.staticToken}, nil
	}
	if s.tokenProvider == nil {
		return gen.BearerAuth{}, errors.New("missing ACTApro token provider")
	}

	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return gen.BearerAuth{}, fmt.Errorf("failed to get access token: %v", err)
	}

	return gen.BearerAuth{Token: token}, nil
}
