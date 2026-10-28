package httpapi

import (
	"net/http"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/go-chi/chi/v5"
)

type createAPIKeyRequest struct {
	Name          string   `json:"name"`
	Environment   string   `json:"environment"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays int      `json:"expires_in_days"`
}

func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	keys, err := h.keys.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	var body createAPIKeyRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	key, err := h.keys.Create(r.Context(), user, domain.CreateAPIKeyInput{
		Name:        body.Name,
		Environment: body.Environment,
		Scopes:      body.Scopes,
		ExpiresDays: body.ExpiresInDays,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	if err := h.keys.Revoke(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
