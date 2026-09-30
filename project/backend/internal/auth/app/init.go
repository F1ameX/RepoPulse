package app

import (
	"context"
	"net/http"

	httpadapter "github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/in/http"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/httpserver"
)

func newHTTPServer(ctx context.Context, cfg Config) *http.Server {
	return httpserver.New(ctx, cfg, httpadapter.NewHandler())
}
