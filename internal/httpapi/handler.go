package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/middleware"
	"github.com/deepsi/agent-service/internal/service"
)

// Deps is everything the HTTP layer needs. Keeping it explicit makes the
// dependency graph visible in app.New and keeps handlers testable.
type Deps struct {
	Config   *config.Config
	Logger   *slog.Logger
	Auth     *service.AuthService
	OAuth    *service.OAuthService
	Sessions *service.SessionService
	Keys     *service.APIKeyService
	Usage    *service.UsageService
	Billing  *service.BillingService
	Webhook  *service.WebhookService

	Users    domain.UserRepository
	APIKeys  domain.APIKeyRepository
	Payments domain.PaymentRepository
	UsageLog domain.UsageRepository
	Ready    func(context.Context) error
}

type Handler struct {
	cfg      *config.Config
	log      *slog.Logger
	auth     *service.AuthService
	oauth    *service.OAuthService
	sessions *service.SessionService
	keys     *service.APIKeyService
	usage    *service.UsageService
	billing  *service.BillingService
	webhook  *service.WebhookService
	llm      *LLMProxy

	users    domain.UserRepository
	apiKeys  domain.APIKeyRepository
	payments domain.PaymentRepository
	usageLog domain.UsageRepository
	ready    func(context.Context) error
}

func New(deps Deps) *Handler {
	h := &Handler{
		cfg:      deps.Config,
		log:      deps.Logger,
		auth:     deps.Auth,
		oauth:    deps.OAuth,
		sessions: deps.Sessions,
		keys:     deps.Keys,
		usage:    deps.Usage,
		billing:  deps.Billing,
		webhook:  deps.Webhook,
		users:    deps.Users,
		apiKeys:  deps.APIKeys,
		payments: deps.Payments,
		usageLog: deps.UsageLog,
		ready:    deps.Ready,
	}
	h.llm = NewLLMProxy(deps.Config, deps.Logger, deps.Usage)
	return h
}

func currentUser(r *http.Request) (*domain.User, bool) {
	return currentUserFromContext(r.Context())
}

func currentUserFromContext(ctx context.Context) (*domain.User, bool) {
	return userFromContext(ctx)
}

func sessionMeta(r *http.Request) service.SessionMeta {
	return service.SessionMeta{
		UserAgent: r.UserAgent(),
		IP:        middleware.ClientIP(r),
	}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeRawJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": h.cfg.App.Name,
		"version": h.cfg.App.Version,
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.ready == nil {
		writeRawJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if err := h.ready(r.Context()); err != nil {
		h.log.Error("readiness check failed", "error", err)
		writeRawJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
		})
		return
	}
	writeRawJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, domain.NotFound("not_found", "Không tìm thấy endpoint này."))
}

func (h *Handler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, domain.NewError(http.StatusMethodNotAllowed, "method_not_allowed", "Phương thức HTTP không được hỗ trợ.", nil))
}
