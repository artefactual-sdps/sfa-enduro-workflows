package apis

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"go.artefactual.dev/tools/clientauth"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
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
			timeout = DefaultTimeout
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

func (s securitySource) Smart(ctx context.Context, _ gen.OperationName) (gen.Smart, error) {
	if s.staticToken != "" {
		return gen.Smart{Token: s.staticToken}, nil
	}
	if s.tokenProvider == nil {
		return gen.Smart{}, errors.New("missing APIS token provider")
	}

	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return gen.Smart{}, fmt.Errorf("failed to get access token: %v", err)
	}

	return gen.Smart{Token: token}, nil
}
