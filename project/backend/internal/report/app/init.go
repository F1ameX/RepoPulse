package app

import (
	"context"
	"net/http"

	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/httpserver"
	httpadapter "github.com/F1ameX/RepoPulse/project/backend/internal/report/adapter/in/http"
)

func newHTTPServer(ctx context.Context, cfg Config) *http.Server {
	return httpserver.New(ctx, cfg, httpadapter.NewHandler())
}
