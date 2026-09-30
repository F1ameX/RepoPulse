// Package httpadapter exposes the Auth Service's operational health endpoint.
package httpadapter

import (
	"net/http"

	httptransport "github.com/F1ameX/RepoPulse/project/backend/internal/pkg/adapter/in/http"
)

// NewHandler exposes process liveness only. Domain transports are not wired yet.
func NewHandler() http.Handler {
	return httptransport.NewHealthHandler()
}
