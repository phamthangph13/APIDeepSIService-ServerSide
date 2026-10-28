package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/httpapi"
	"github.com/deepsi/agent-service/internal/platform/mailer"
	platformmongo "github.com/deepsi/agent-service/internal/platform/mongo"
	"github.com/deepsi/agent-service/internal/platform/security"
	mongostore "github.com/deepsi/agent-service/internal/repository/mongo"
	"github.com/deepsi/agent-service/internal/service"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type App struct {
	cfg        *config.Config
	log        *slog.Logger
	server     *http.Server
	db         *mongo.Database
	users      domain.UserRepository
	billing    *service.BillingService
	disconnect func(context.Context) error
	handler    http.Handler
	cancelJobs context.CancelFunc
}

func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	db, disconnect, err := platformmongo.Connect(ctx, cfg.Mongo.URI, cfg.Mongo.Database, cfg.Mongo.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	store := mongostore.NewStore(db)
	if err := store.EnsureIndexes(ctx, db); err != nil {
		_ = disconnect(context.Background())
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}

	mailerClient := mailer.NewSMTPMailer(cfg.SMTP, log)
	emailService := service.NewEmailService(cfg, mailerClient, log)
	sessions := service.NewSessionService(cfg, store.Tokens, store.Users)
	authService := service.NewAuthService(cfg, store.Users, store.Tokens, sessions, emailService, log)
	oauthService := service.NewOAuthService(cfg, store.Users, sessions, log)
	keyService := service.NewAPIKeyService(cfg, store.APIKeys, store.Users)
	usageService := service.NewUsageService(cfg, store.Usage)
	sepayService := service.NewSePayService(cfg)
	billingService := service.NewBillingService(cfg, store.Users, store.Payments, usageService, emailService, sepayService, log)
	webhookService := service.NewWebhookService(cfg, store.Webhooks, billingService, sepayService, log)

	handler := httpapi.New(httpapi.Deps{
		Config:   cfg,
		Logger:   log,
		Auth:     authService,
		OAuth:    oauthService,
		Sessions: sessions,
		Keys:     keyService,
		Usage:    usageService,
		Billing:  billingService,
		Webhook:  webhookService,
		Users:    store.Users,
		APIKeys:  store.APIKeys,
		Payments: store.Payments,
		UsageLog: store.Usage,
		Ready: func(ctx context.Context) error {
			checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			return db.Client().Ping(checkCtx, readpref.Primary())
		},
	})

	app := &App{
		cfg:        cfg,
		log:        log,
		db:         db,
		users:      store.Users,
		billing:    billingService,
		disconnect: disconnect,
		handler:    handler.Router(),
	}
	app.server = &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           app.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	if err := app.seed(ctx, store); err != nil {
		_ = disconnect(context.Background())
		return nil, fmt.Errorf("seed data: %w", err)
	}
	app.startJobs()

	return app, nil
}

func (a *App) Handler() http.Handler { return a.handler }

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("http server listening",
			"addr", a.cfg.HTTP.Addr,
			"env", a.cfg.App.Env,
			"frontend", a.cfg.App.FrontendURL,
			"plans", a.cfg.PlanSummary(),
			"llm_enabled", a.cfg.LLM.Enabled,
			"payment_mode", a.billing.PaymentMode(),
			"sepay_webhook_auth", a.cfg.SePay.WebhookAuthMode,
		)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if a.cancelJobs != nil {
			a.cancelJobs()
		}
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) Close(ctx context.Context) {
	if a.cancelJobs != nil {
		a.cancelJobs()
	}
	if a.disconnect != nil {
		_ = a.disconnect(ctx)
	}
}

func (a *App) startJobs() {
	jobsCtx, cancel := context.WithCancel(context.Background())
	a.cancelJobs = cancel
	go func() {
		a.expirePlans(jobsCtx)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-jobsCtx.Done():
				return
			case <-ticker.C:
				a.expirePlans(jobsCtx)
			}
		}
	}()
}

func (a *App) expirePlans(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	updated, err := a.users.ExpirePlans(timeoutCtx, time.Now())
	if err != nil {
		a.log.Error("expire plans failed", "error", err)
		return
	}
	if updated > 0 {
		a.log.Info("expired paid plans", "count", updated)
	}
}

func (a *App) seed(ctx context.Context, store *mongostore.Store) error {
	if a.cfg.Seed.AdminEmail != "" && a.cfg.Seed.AdminPassword != "" {
		if err := seedUser(ctx, a.cfg, store.Users, seedInput{
			Email:    a.cfg.Seed.AdminEmail,
			Password: a.cfg.Seed.AdminPassword,
			Name:     a.cfg.Seed.AdminName,
			Role:     domain.RoleAdmin,
			Plan:     domain.PlanFree,
		}); err != nil {
			return err
		}
	}
	return nil
}

type seedInput struct {
	Email    string
	Password string
	Name     string
	Role     string
	Plan     domain.PlanID
}

func seedUser(ctx context.Context, cfg *config.Config, users domain.UserRepository, input seedInput) error {
	existing, err := users.FindByEmail(ctx, input.Email)
	if err == nil {
		if existing.Role != input.Role {
			if setErr := users.SetRole(ctx, existing.ID, input.Role); setErr != nil {
				return setErr
			}
		}
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if err := security.ValidatePassword(input.Password); err != nil {
		return fmt.Errorf("seed password for %s: %w", input.Email, err)
	}
	hash, err := security.HashPassword(input.Password, cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	count, _ := users.Count(ctx)
	now := time.Now()
	user := &domain.User{
		ID:            "usr_" + uuid.NewString(),
		Name:          input.Name,
		Email:         input.Email,
		PasswordHash:  hash,
		Provider:      domain.ProviderPassword,
		Role:          input.Role,
		Plan:          input.Plan,
		EmailVerified: true,
		AvatarColor:   security.AvatarColor(int(count)),
	}
	if input.Plan == domain.PlanPlus || input.Plan == domain.PlanUltral {
		expiry := now.AddDate(0, 1, 0)
		user.PlanStartedAt = &now
		user.PlanExpiresAt = &expiry
	}
	if err := users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return nil
		}
		return err
	}
	return nil
}
