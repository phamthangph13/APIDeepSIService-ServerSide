package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/go-chi/chi/v5"
)

type oauthExchangeRequest struct {
	Code string `json:"code"`
}

// OAuthStart sends the browser to the provider. The frontend uses a normal
// navigation so the provider can show its own consent screen.
func (h *Handler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	target, err := h.oauth.StartURL(provider, r.URL.Query().Get("next"))
	if err != nil {
		h.redirectOAuthError(w, r, err)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// OAuthURL is the JSON variant used when a client wants to control navigation
// itself.
func (h *Handler) OAuthURL(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	target, err := h.oauth.StartURL(provider, r.URL.Query().Get("next"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": target})
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(chi.URLParam(r, "provider"))
	query := r.URL.Query()
	if providerError := query.Get("error"); providerError != "" {
		description := query.Get("error_description")
		if description == "" {
			description = providerError
		}
		h.redirectOAuthError(w, r, domain.NewError(http.StatusBadRequest, "oauth_denied", description, nil))
		return
	}
	target, err := h.oauth.HandleCallback(r.Context(), provider, query.Get("code"), query.Get("state"))
	if err != nil {
		h.redirectOAuthError(w, r, err)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) OAuthExchange(w http.ResponseWriter, r *http.Request) {
	var body oauthExchangeRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.oauth.ExchangeCode(r.Context(), body.Code, sessionMeta(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) redirectOAuthError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := domain.AsError(err)
	message := "Đăng nhập OAuth không thành công."
	if appErr != nil && appErr.Message != "" {
		message = appErr.Message
	}
	target := h.cfg.App.FrontendURL + "/login?oauth_error=" + url.QueryEscape(message)
	http.Redirect(w, r, target, http.StatusFound)
}
