# API reference

Base URL: `http://localhost:8080/api/v1` (development) or `https://api.service.deepsi.space/api/v1` (production).

All authenticated endpoints expect:

```http
Authorization: Bearer <access_token>
```

Success responses are wrapped in `data`:

```json
{ "data": { "id": "usr_...", "email": "ban@congty.vn" } }
```

Errors:

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "Email hoặc mật khẩu không đúng.",
    "request_id": "req_..."
  }
}
```

## Auth

### Register

```http
POST /auth/register
Content-Type: application/json

{
  "name": "Nguyễn Minh Anh",
  "email": "ban@congty.vn",
  "password": "mat-khau-du-manh"
}
```

Response `201`:

```json
{
  "data": {
    "user": {
      "id": "usr_...",
      "name": "Nguyễn Minh Anh",
      "email": "ban@congty.vn",
      "provider": "password",
      "role": "client",
      "plan": "free",
      "email_verified": true,
      "avatar_color": "moss",
      "created_at": "2026-10-28T09:00:00Z",
      "updated_at": "2026-10-28T09:00:00Z"
    },
    "access_token": "eyJ...",
    "refresh_token": "opaque...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

If `REQUIRE_EMAIL_VERIFICATION=true`, no tokens are returned and `requires_email_verification` is `true`.

### Login

```http
POST /auth/login

{ "email": "ban@congty.vn", "password": "mat-khau-du-manh" }
```

Returns the same `AuthResult` shape as register.

### Refresh

```http
POST /auth/refresh

{ "refresh_token": "opaque..." }
```

Refresh tokens rotate: the old token is revoked and a new one is returned. A cookie (`deepsi_refresh`) is also set for browser clients.

### Logout

```http
POST /auth/logout

{ "refresh_token": "opaque..." }
```

Returns `204`.

### Forgot password

```http
POST /auth/forgot-password

{ "email": "ban@congty.vn" }
```

Always returns a generic message. In development, with `AUTH_EXPOSE_DEBUG_TOKENS=true` and no SMTP host, the response also contains `debug_token` and `reset_url`.

### Reset password

```http
POST /auth/reset-password

{ "token": "token-from-email", "password": "mat-khau-moi" }
```

Returns a new session (`AuthResult`). All old refresh tokens are revoked.

### Verify email

```http
POST /auth/verify-email

{ "token": "token-from-email" }
```

### Resend verification

```http
POST /auth/resend-verification

{ "email": "ban@congty.vn" }
```

### OAuth

Start (browser navigation):

```http
GET /auth/oauth/google/start?next=/dashboard
GET /auth/oauth/github/start?next=/dashboard
```

The provider redirects back to `GET /auth/oauth/{provider}/callback`, which redirects the browser to:

```
{FRONTEND_URL}/auth/callback?code=<one-time-code>&next=/dashboard
```

Exchange the one-time code:

```http
POST /auth/oauth/exchange

{ "code": "one-time-code" }
```

Returns `AuthResult`.

A JSON URL variant is available at `GET /auth/oauth/{provider}/url?next=/dashboard`.

### Profile

```http
GET /auth/me
PATCH /auth/me

{ "name": "Tên mới", "avatar_color": "clay" }
POST /auth/change-password

{ "current_password": "...", "new_password": "..." }
```

`change-password` returns a new session.

## API keys

```http
GET /api-keys
```

Response:

```json
{
  "data": [
    {
      "id": "key_...",
      "name": "Production · Website",
      "prefix": "dsi_live_7Hk2",
      "last4": "aK9e",
      "environment": "production",
      "scopes": ["chat", "embeddings"],
      "status": "active",
      "created_at": "2026-10-28T09:00:00Z",
      "last_used_at": "2026-10-28T10:00:00Z"
    }
  ]
}
```

Create:

```http
POST /api-keys

{
  "name": "Production · Website",
  "environment": "production",
  "scopes": ["chat", "embeddings"],
  "expires_in_days": 90
}
```

Response `201` includes the only plaintext copy of the key:

```json
{
  "data": {
    "id": "key_...",
    "key": "dsi_live_...",
    "...": "..."
  }
}
```

Revoke:

```http
DELETE /api-keys/{id}
```

Returns `204`. Revoked keys fail authentication immediately.

Scopes: `chat`, `embeddings`, `vision`, `models`.

## Usage

```http
GET /usage?range=30
GET /usage/today
GET /usage/activity?limit=7
```

`range` accepts `7`, `30`, or `90`. Summary fields:

```json
{
  "data": {
    "days": [
      { "date": "2026-10-01", "label": "01/10", "tokens": 1200, "requests": 4, "input": 800, "output": 400 }
    ],
    "total_tokens": 1200,
    "total_requests": 4,
    "avg_latency_ms": 320,
    "p95_latency_ms": 610,
    "error_rate": 0.0,
    "models": [
      { "id": "deepsi-m1", "name": "DeepSI-M1", "share": 1.0, "tokens": 1200, "requests": 4 }
    ],
    "quota": 300000,
    "used_ratio": 0.004,
    "input_tokens": 800,
    "output_tokens": 400
  }
}
```

## Billing

Plans (public):

```http
GET /plans
```

Overview:

```http
GET /billing
```

Returns current plan, period dates, usage, invoices, pending order, and payment configuration (`gateway`, `bank_transfer`, `mock`, or `none`).

Checkout:

```http
POST /billing/checkout

{ "plan_id": "plus" }
```

Response:

```json
{
  "data": {
    "order": {
      "id": "ord_...",
      "plan_id": "plus",
      "amount_vnd": 1499000,
      "currency": "VND",
      "invoice_number": "DSI-20261028-AB12CD",
      "payment_code": "DSI1A2B3C4D",
      "status": "pending",
      "expires_at": "2026-10-29T09:00:00Z"
    },
    "mode": "bank_transfer",
    "bank_transfer": {
      "bank_code": "VCB",
      "account_number": "0123456789",
      "account_holder": "CONG TY DEEPSI",
      "amount_vnd": 1499000,
      "content": "DSI1A2B3C4D",
      "qr_url": "https://qr.sepay.vn/img?..."
    }
  }
}
```

`mode` is one of:

- `gateway`: render a form with `action`, `method`, and `fields` (includes SePay signature), then submit it.
- `bank_transfer`: show the bank details/QR and poll the order.
- `mock`: local development only; call mock-complete.

Order status:

```http
GET /billing/orders/{id}
```

Mock payment (only with `PAYMENT_MOCK_ENABLED=true`):

```http
POST /billing/orders/{id}/mock-complete
```

Cancel renewal (keeps the plan until period end):

```http
POST /billing/cancel
```

Downgrade to Free immediately:

```http
POST /billing/downgrade
```

## Admin

Requires a user with `role: "admin"`.

```http
GET /admin/stats
GET /admin/users?q=&role=&plan=&limit=20&offset=0
PATCH /admin/users/{id}/role

{ "role": "admin" }

PATCH /admin/users/{id}/plan

{ "plan_id": "plus", "months": 1 }

GET /admin/orders?status=pending&limit=20&offset=0
GET /admin/usage?days=30
```

`GET /admin/users` items contain `user`, `key_count`, `tokens_30d`, and `last_seen`.

## Webhook

```http
POST /webhooks/sepay
```

See [SEPAY.md](SEPAY.md). This endpoint does not use the normal user auth middleware.

## OpenAI-compatible proxy

Base URL: `/v1` (not under `/api/v1`). Authenticate with a DeepSI API key:

```http
POST /v1/chat/completions
Authorization: Bearer dsi_live_...
Content-Type: application/json

{
  "model": "deepsi-m1",
  "messages": [{ "role": "user", "content": "Xin chào" }],
  "stream": false
}
```

Also available:

```http
GET /v1/models
POST /v1/embeddings
```

The proxy forwards other `/v1/*` paths as well. It checks scopes and quota, forwards streaming SSE, and records token usage when the upstream returns a `usage` object. For streaming, include `stream_options: { "include_usage": true }` if your upstream supports it so usage can be recorded exactly.

Errors use the OpenAI shape:

```json
{
  "error": {
    "message": "Đã hết hạn mức token.",
    "type": "insufficient_quota",
    "code": "monthly_quota_exhausted"
  }
}
```

## Health

```http
GET /healthz
GET /readyz
```

`readyz` pings MongoDB and returns `503` when it is unavailable.
