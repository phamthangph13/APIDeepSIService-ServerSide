package httpapi

import (
	"context"
	"net/http"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/httpx"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	httpx.WriteJSON(w, status, data)
}

func writeRawJSON(w http.ResponseWriter, status int, data any) {
	httpx.WriteRawJSON(w, status, data)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteError(w, r, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	return httpx.DecodeJSON(w, r, target)
}

func userFromContext(ctx context.Context) (*domain.User, bool) {
	return httpx.User(ctx)
}

func requestID(ctx context.Context) string {
	id, _ := httpx.RequestID(ctx)
	return id
}
