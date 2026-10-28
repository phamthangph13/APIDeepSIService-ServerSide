package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/deepsi/agent-service/internal/domain"
)

type contextKey string

const (
	userContextKey      contextKey = "auth_user"
	claimsContextKey    contextKey = "auth_claims"
	apiKeyContextKey    contextKey = "auth_api_key"
	requestIDContextKey contextKey = "request_id"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func WriteRawJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := domain.AsError(err)
	if appErr == nil {
		appErr = domain.Internal(errors.New("unknown error"))
	}
	requestID, _ := RequestID(r.Context())
	WriteRawJSON(w, appErr.Status, errorBody{Error: apiError{
		Code:      appErr.Code,
		Message:   appErr.Message,
		RequestID: requestID,
	}})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.Invalid("empty_body", "Nội dung yêu cầu không được để trống.")
		}
		return domain.Invalid("invalid_json", "Dữ liệu gửi lên không hợp lệ.")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.Invalid("invalid_json", "Chỉ được gửi một đối tượng JSON.")
	}
	return nil
}

func BearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func User(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(userContextKey).(*domain.User)
	return user, ok
}

func WithClaims(ctx context.Context, claims any) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

func Claims(ctx context.Context) any {
	return ctx.Value(claimsContextKey)
}

func WithAPIKey(ctx context.Context, key *domain.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyContextKey, key)
}

func APIKey(ctx context.Context) (*domain.APIKey, bool) {
	key, ok := ctx.Value(apiKeyContextKey).(*domain.APIKey)
	return key, ok
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, id)
}

func RequestID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey).(string)
	return id, ok
}
