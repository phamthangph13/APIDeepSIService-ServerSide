package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/service"
)

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type updateProfileRequest struct {
	Name        string `json:"name"`
	AvatarColor string `json:"avatar_color"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body registerRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.Register(r.Context(), service.RegisterInput{
		Name:     body.Name,
		Email:    body.Email,
		Password: body.Password,
	}, sessionMeta(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.Login(r.Context(), body.Email, body.Password, sessionMeta(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &body); err != nil {
			writeError(w, r, err)
			return
		}
	}
	raw := strings.TrimSpace(body.RefreshToken)
	if raw == "" {
		raw = refreshCookie(r)
	}
	result, err := h.auth.Refresh(r.Context(), raw, sessionMeta(r))
	if err != nil {
		h.clearRefreshCookie(w)
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if r.ContentLength != 0 {
		_ = decodeJSON(w, r, &body)
	}
	raw := strings.TrimSpace(body.RefreshToken)
	if raw == "" {
		raw = refreshCookie(r)
	}
	_ = h.auth.Logout(r.Context(), raw)
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	fresh, err := h.auth.Me(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, fresh)
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	var body updateProfileRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	updated, err := h.auth.UpdateProfile(r.Context(), user.ID, body.Name, body.AvatarColor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	var body changePasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.ChangePassword(r.Context(), user.ID, body.CurrentPassword, body.NewPassword, sessionMeta(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body forgotPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	debugToken, err := h.auth.ForgotPassword(r.Context(), body.Email)
	if err != nil {
		writeError(w, r, err)
		return
	}
	response := map[string]any{
		"message": "Nếu email này có tài khoản, chúng tôi đã gửi liên kết đặt lại mật khẩu. Kiểm tra hộp thư và thư mục spam.",
	}
	// Only populated when AUTH_EXPOSE_DEBUG_TOKENS=true and SMTP is not
	// configured, so local development can complete the flow without email.
	if debugToken != "" {
		response["debug_token"] = debugToken
		response["reset_url"] = h.cfg.App.FrontendURL + "/reset-password?token=" + url.QueryEscape(debugToken)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.ResetPassword(r.Context(), body.Token, body.Password, sessionMeta(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.attachRefreshCookie(w, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body verifyEmailRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.auth.VerifyEmail(r.Context(), body.Token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Email đã được xác nhận. Bạn có thể đăng nhập.",
		"user":    user,
	})
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var body forgotPasswordRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.auth.ResendVerification(r.Context(), body.Email); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Nếu email này chưa được xác nhận, chúng tôi đã gửi lại liên kết.",
	})
}

func (h *Handler) attachRefreshCookie(w http.ResponseWriter, result *service.AuthResult) {
	if result == nil || result.RefreshToken == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    result.RefreshToken,
		Path:     "/api/v1/auth",
		Domain:   h.cfg.Auth.CookieDomain,
		MaxAge:   int(h.cfg.Auth.RefreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.Auth.CookieSecure,
		SameSite: parseSameSite(h.cfg.Auth.CookieSameSite),
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		Domain:   h.cfg.Auth.CookieDomain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.cfg.Auth.CookieSecure,
		SameSite: parseSameSite(h.cfg.Auth.CookieSameSite),
	})
}

const refreshCookieName = "deepsi_refresh"

func refreshCookie(r *http.Request) string {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
