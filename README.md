# DeepSI Agent — API server

Go + MongoDB backend for [service.deepsi.space](https://service.deepsi.space). It provides email/password and Google/GitHub authentication, client and admin roles, API keys, usage metering, SePay billing, and an OpenAI-compatible LLM proxy.

The React frontend lives in a separate repository (`APIDeepSIService`). This repository is the server only.

## Features

- Email/password registration, login, email verification, password reset over SMTP
- Google and GitHub OAuth with signed state and one-time exchange codes
- JWT access tokens + rotating opaque refresh tokens
- `client` and `admin` roles
- API keys with environments, scopes, expiry, revocation, and HMAC-peppered hashes
- Per-plan daily/monthly token quotas and key limits
- Usage dashboard data: daily tokens, requests, latency p95, error rate, per-model breakdown
- OpenAI-compatible `/v1` proxy with streaming, quota enforcement, and usage logging
- SePay webhook (API key or HMAC-SHA256), Payment Gateway checkout, bank-transfer QR fallback
- Admin API for users, plans, orders, revenue, and usage
- Plan prices, quotas, key limits, and retention are environment-configurable
- Health and readiness endpoints, request IDs, structured logs, graceful shutdown

## Project layout

```
cmd/api/                  entrypoint
internal/app/             dependency wiring, seeding, background jobs
internal/config/          env loader + plan catalog
internal/domain/          entities, repository interfaces, errors
internal/httpx/           JSON responses, auth context helpers
internal/httpapi/         HTTP handlers, router, LLM proxy
internal/middleware/      auth, CORS, rate limiting, logging, recovery
internal/platform/        security, SMTP, MongoDB connection/indexes
internal/repository/mongo concrete repositories
internal/service/         auth, sessions, OAuth, keys, usage, billing, SePay, webhooks
docs/                     API and SePay integration notes
```

## Quick start

Requirements: Go 1.24+, MongoDB 6/7, and a `.env` file.

```bash
cp .env.example .env
# Fill JWT_SECRET, API_KEY_PEPPER, MONGO_URI, and any providers you use.

# Start MongoDB locally, or:
docker compose up -d mongo

go run ./cmd/api
```

The API listens on `http://localhost:8080` by default.

Check it:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

If `ADMIN_EMAIL`/`ADMIN_PASSWORD` are set, the admin account is created on first boot. Existing accounts are not overwritten.

### Local development without SMTP

Leave `SMTP_HOST` empty. Emails are written to the process log instead of being sent. Set `AUTH_EXPOSE_DEBUG_TOKENS=true` to include a `debug_token`/`reset_url` in the forgot-password response so you can complete the flow locally.

### Local development without SePay

Set `PAYMENT_MOCK_ENABLED=true`. Checkout returns `mode: "mock"` and the frontend can call:

```http
POST /api/v1/billing/orders/{order_id}/mock-complete
Authorization: Bearer <access_token>
```

This is disabled in production unless explicitly enabled.

## Environment

All values are documented in [`.env.example`](.env.example). Important groups:

| Group | Variables |
| --- | --- |
| App | `APP_ENV`, `APP_NAME`, `PORT`, `FRONTEND_URL`, `PUBLIC_API_URL`, `APP_TIMEZONE` |
| Mongo | `MONGO_URI`, `MONGO_DATABASE`, `MONGO_CONNECT_TIMEOUT` |
| Auth | `JWT_SECRET`, `API_KEY_PEPPER`, token TTLs, `BCRYPT_COST`, `REQUIRE_EMAIL_VERIFICATION` |
| SMTP | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_TLS_MODE` |
| OAuth | `GOOGLE_*`, `GITHUB_*` |
| SePay | `SEPAY_WEBHOOK_*`, `SEPAY_MERCHANT_ID`, `SEPAY_CHECKOUT_SECRET`, bank details |
| LLM | `LLM_ENABLED`, `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_TIMEOUT`, `LLM_MODELS` |
| Plans | `PLAN_<ID>_PRICE_VND`, `_MONTHLY_TOKENS`, `_DAILY_TOKENS`, `_KEY_LIMIT`, `_RETENTION_DAYS` |

`JWT_SECRET` and `API_KEY_PEPPER` are required in production and must be at least 32 characters. `API_KEY_PEPPER` should be different from `JWT_SECRET`.

### LLM-APIGATEWAY routing

Client applications call this service's documented `/v1` endpoints. This backend validates the DeepSI API key, scopes, and account quota, then forwards the request to the internal LLM-APIGATEWAY. Keep provider credentials and model-to-upstream mappings in the gateway only. When both services share a host, set `LLM_BASE_URL=http://127.0.0.1:8085/v1`; set this service's `LLM_API_KEY` to the same secret as the gateway's `LLM_GATEWAY_API_KEY`. The gateway must be reachable from the backend host. Production startup requires this route to be enabled and configured.

To enable email verification in production, configure `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, and `SMTP_TLS_MODE=starttls` (or `tls`) in the deployment's secret environment, then set `REQUIRE_EMAIL_VERIFICATION=true`. The service refuses to start if verification is required without SMTP, or if production SMTP is configured without TLS. Keep `AUTH_EXPOSE_DEBUG_TOKENS=false` in production. SMTP credentials must be supplied by the deployment owner; they are intentionally not stored in this repository.

## API overview

Base path: `/api/v1`. See [docs/API.md](docs/API.md) for request/response examples.

| Area | Endpoints |
| --- | --- |
| Auth | `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/forgot-password`, `/auth/reset-password`, `/auth/verify-email`, `/auth/resend-verification` |
| OAuth | `GET /auth/oauth/{google|github}/start`, `GET /auth/oauth/{provider}/callback`, `POST /auth/oauth/exchange` |
| Profile | `GET/PATCH /auth/me`, `POST /auth/change-password` |
| API keys | `GET/POST /api-keys`, `DELETE /api-keys/{id}` |
| Usage | `GET /usage?range=7|30|90`, `/usage/today`, `/usage/activity` |
| Billing | `GET /plans`, `GET /billing`, `POST /billing/checkout`, `GET /billing/orders/{id}`, `POST /billing/cancel`, `POST /billing/downgrade` |
| Admin | `GET /admin/stats`, `/admin/users`, `PATCH /admin/users/{id}/role`, `/admin/users/{id}/plan`, `/admin/orders`, `/admin/usage` |
| Webhook | `POST /webhooks/sepay` |
| LLM proxy | `GET /v1/models`, `POST /v1/chat/completions`, `POST /v1/embeddings` (and other `/v1/*` paths) |

Responses use a `{ "data": ... }` envelope, except the SePay webhook and the OpenAI-compatible `/v1` routes, which preserve their native shapes. Errors use:

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "Email hoặc mật khẩu không đúng.",
    "request_id": "req_..."
  }
}
```

## LLM proxy

The proxy sends requests only to the configured internal LLM-APIGATEWAY. Keep `LLM_BASE_URL` ending in `/v1`; the backend forwards the DeepSI API key's request to the gateway with the gateway's own bearer key. If it ends in `/v1`, the proxy strips the local `/v1` prefix before joining; if not, it adds `/v1`.

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer dsi_live_..." \
  -H "Content-Type: application/json" \
  -d '{"model":"deepsi-m1","messages":[{"role":"user","content":"Xin chào"}]}'
```

The proxy checks the API key's scopes (`chat`, `embeddings`, `vision`, `models`), enforces the owner's quota, forwards streaming responses, and records usage after each request. When `LLM_ENABLED=false`, `/v1/models` still returns the configured `LLM_MODELS` list so clients can discover models.

## SePay

See [docs/SEPAY.md](docs/SEPAY.md) for webhook setup, signature verification, checkout fields, and bank-transfer QR configuration. The webhook endpoint is `POST /api/v1/webhooks/sepay` and always returns `{"success": true}` for accepted events.

## Deployment

```bash
cp .env.example .env
# set production values
docker compose up --build -d
```

Set `APP_ENV=production`, `FRONTEND_URL`, `PUBLIC_API_URL`, `CORS_ALLOWED_ORIGINS`, `COOKIE_SECURE=true`, and the internal `LLM-APIGATEWAY` URL/key. Provider credentials belong only in the gateway environment. Put the API behind TLS (Caddy, Nginx, or a cloud load balancer). Use a managed MongoDB with authentication and backups.

## Security notes

- Passwords are bcrypt hashes. API keys are stored as SHA-256 HMAC hashes with a server-side pepper; plaintext is returned only once at creation.
- Refresh tokens are opaque, hashed at rest, rotated on use, and revoked on password reset/change.
- OAuth state is a short-lived signed JWT; the callback exchanges a one-time code for tokens, so access tokens never appear in the URL.
- Webhook authentication supports SePay's API key and HMAC-SHA256 modes. `none` is rejected in production.
- The in-memory rate limiter is per-process. Use Redis or an edge limiter when running multiple replicas.
- Usage logs are removed by a MongoDB TTL index based on each plan's retention days.
