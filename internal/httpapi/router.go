package httpapi

import (
	"net/http"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func (h *Handler) Router() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger(h.log))
	router.Use(middleware.Recoverer(h.log))
	router.Use(middleware.SecurityHeaders)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   h.cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.BodyLimit(h.cfg.HTTP.MaxBodyBytes))

	authLimiter := middleware.NewRateLimiter(h.cfg.Auth.RateLimitPerMinute, time.Minute)

	router.Get("/healthz", h.Health)
	router.Get("/readyz", h.Ready)

	router.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.With(middleware.RateLimit(authLimiter)).Post("/register", h.Register)
			auth.With(middleware.RateLimit(authLimiter)).Post("/login", h.Login)
			auth.With(middleware.RateLimit(authLimiter)).Post("/forgot-password", h.ForgotPassword)
			auth.With(middleware.RateLimit(authLimiter)).Post("/reset-password", h.ResetPassword)
			auth.With(middleware.RateLimit(authLimiter)).Post("/resend-verification", h.ResendVerification)
			auth.Post("/refresh", h.Refresh)
			auth.Post("/logout", h.Logout)
			auth.Post("/verify-email", h.VerifyEmail)
			auth.Post("/oauth/exchange", h.OAuthExchange)
			auth.Get("/oauth/{provider}/start", h.OAuthStart)
			auth.Get("/oauth/{provider}/url", h.OAuthURL)
			auth.Get("/oauth/{provider}/callback", h.OAuthCallback)

			auth.Group(func(protected chi.Router) {
				protected.Use(middleware.RequireAuth(h.sessions, h.users))
				protected.Get("/me", h.Me)
				protected.Patch("/me", h.UpdateMe)
				protected.Post("/change-password", h.ChangePassword)
			})
		})

		api.Get("/plans", h.Plans)
		api.Get("/billing/plans", h.Plans)

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(h.sessions, h.users))

			protected.Get("/api-keys", h.ListAPIKeys)
			protected.Post("/api-keys", h.CreateAPIKey)
			protected.Delete("/api-keys/{id}", h.RevokeAPIKey)

			protected.Get("/usage", h.UsageSummary)
			protected.Get("/usage/today", h.UsageToday)
			protected.Get("/usage/activity", h.UsageActivity)

			protected.Get("/billing", h.BillingOverview)
			protected.Post("/billing/checkout", h.Checkout)
			protected.Get("/billing/orders/{id}", h.OrderStatus)
			protected.Post("/billing/orders/{id}/mock-complete", h.CompleteMockPayment)
			protected.Post("/billing/cancel", h.CancelRenewal)
			protected.Post("/billing/downgrade", h.Downgrade)
		})

		api.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.RequireAuth(h.sessions, h.users))
			admin.Use(middleware.RequireRole(domain.RoleAdmin))
			admin.Get("/stats", h.AdminStats)
			admin.Get("/users", h.AdminUsers)
			admin.Patch("/users/{id}/role", h.AdminUpdateRole)
			admin.Patch("/users/{id}/plan", h.AdminUpdatePlan)
			admin.Get("/orders", h.AdminOrders)
			admin.Get("/usage", h.AdminUsage)
		})

		api.Post("/webhooks/sepay", h.SePayWebhook)
	})

	llmHandler := middleware.RequireAPIKey(h.keys)(http.HandlerFunc(h.llm.ServeHTTP))
	router.Handle("/v1", llmHandler)
	router.Handle("/v1/*", llmHandler)

	router.NotFound(h.NotFound)
	router.MethodNotAllowed(h.MethodNotAllowed)
	return router
}
