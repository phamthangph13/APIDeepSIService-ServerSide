package middleware

import (
	"net/http"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/httpx"
	"github.com/deepsi/agent-service/internal/service"
)

func RequireAuth(sessions *service.SessionService, users domain.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := httpx.BearerToken(r)
			if token == "" {
				httpx.WriteError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
				return
			}
			claims, err := sessions.ParseAccess(token)
			if err != nil {
				httpx.WriteError(w, r, domain.Unauthorized("invalid_token", "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại."))
				return
			}
			user, err := users.FindByID(r.Context(), claims.UserID)
			if err != nil {
				httpx.WriteError(w, r, domain.Unauthorized("user_not_found", "Tài khoản không tồn tại."))
				return
			}
			ctx := httpx.WithUser(r.Context(), user)
			ctx = httpx.WithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := httpx.User(r.Context())
			if !ok {
				httpx.WriteError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
				return
			}
			if user.Role != role {
				httpx.WriteError(w, r, domain.Forbidden("forbidden", "Bạn không có quyền truy cập khu vực này."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAPIKey(keys *service.APIKeyService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := httpx.BearerToken(r)
			if raw == "" {
				httpx.WriteRawJSON(w, http.StatusUnauthorized, openAIError("invalid_api_key", "Thiếu API key. Dùng header Authorization: Bearer dsi_..."))
				return
			}
			user, key, err := keys.Authenticate(r.Context(), raw)
			if err != nil {
				httpx.WriteRawJSON(w, http.StatusUnauthorized, openAIError("invalid_api_key", "API key không hợp lệ hoặc đã bị thu hồi."))
				return
			}
			ctx := httpx.WithUser(r.Context(), user)
			ctx = httpx.WithAPIKey(ctx, key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func openAIError(code, message string) map[string]any {
	return map[string]any{"error": map[string]any{
		"type":    "authentication_error",
		"code":    code,
		"message": message,
	}}
}
