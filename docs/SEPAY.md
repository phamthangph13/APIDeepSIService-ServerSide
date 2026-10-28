# SePay integration

The server supports two ways to collect money and one webhook to confirm it:

1. **SePay Payment Gateway checkout** — redirect the user to SePay's hosted checkout.
2. **Bank-transfer QR fallback** — show the user a VietQR code and bank details, then wait for the webhook.
3. **Webhook** — SePay calls `POST /api/v1/webhooks/sepay` when a bank transaction arrives.

All credentials, URLs, bank details, and the order TTL are environment variables. See `.env.example`.

## Webhook setup

Configure the webhook URL in the SePay dashboard:

```
https://api.service.deepsi.space/api/v1/webhooks/sepay
```

SePay supports two authentication modes. Pick one with `SEPAY_WEBHOOK_AUTH_MODE`.

### API key mode

```env
SEPAY_WEBHOOK_AUTH_MODE=api_key
SEPAY_WEBHOOK_API_KEY=your-sepay-webhook-api-key
```

SePay sends:

```http
Authorization: Apikey your-sepay-webhook-api-key
```

The comparison is constant-time. An empty key rejects every request.

### HMAC mode

```env
SEPAY_WEBHOOK_AUTH_MODE=hmac
SEPAY_WEBHOOK_SECRET=your-sepay-hmac-secret
```

SePay sends:

```http
X-SePay-Timestamp: 1720000000
X-SePay-Signature: sha256=<hex>
```

The signature is HMAC-SHA256 over:

```
<timestamp>.<raw request body>
```

The server also rejects timestamps older or newer than five minutes to prevent replay. The signature comparison is constant-time.

### No auth (development only)

```env
SEPAY_WEBHOOK_AUTH_MODE=none
```

Only allowed when `APP_ENV` is not `production`.

## Payload

The server accepts SePay's bank-webhook payload. Relevant fields:

```json
{
  "id": 92704,
  "gateway": "Vietcombank",
  "transactionDate": "2026-10-28 10:12:30",
  "accountNumber": "0123456789",
  "code": "DSI1A2B3C4D",
  "content": "DSI1A2B3C4D chuyen tien",
  "transferType": "in",
  "description": "...",
  "transferAmount": 1499000,
  "accumulated": 0,
  "referenceCode": "FT26123456789"
}
```

Processing rules:

- `transferType` must be `in`; outgoing transfers are acknowledged and ignored.
- The payment code is taken from `code` first, then extracted from `content`/`description` with the pattern `DSI[A-Z0-9]{8}`.
- Orders are matched by `payment_code`.
- The order must be `pending`, not expired, and the transfer amount must be at least the order amount.
- `id` is unique in `webhook_events`, so retries cannot apply a payment twice.
- If processing fails, the event is marked `failed` and the endpoint returns `500`, so SePay retries. A retry is allowed to run again.
- Accepted deliveries return exactly:

```json
{ "success": true }
```

## Payment code and order lifecycle

Each checkout creates a `payment_orders` document:

| Field | Meaning |
| --- | --- |
| `invoice_number` | Human-readable invoice, e.g. `DSI-20261028-AB12CD` |
| `payment_code` | Transfer content, e.g. `DSI1A2B3C4D` |
| `amount_vnd` | Amount in VND |
| `status` | `pending`, `paid`, `expired`, `cancelled`, `failed` |
| `expires_at` | `PAYMENT_ORDER_TTL` after creation |
| `paid_at` | Set when the webhook confirms payment |

When a payment succeeds, the user's plan is activated for one month. If they renew the same plan before it expires, the new month is added to the existing expiry.

## Payment Gateway checkout

```env
SEPAY_MERCHANT_ID=your-merchant-id
SEPAY_CHECKOUT_SECRET=your-checkout-secret
SEPAY_CHECKOUT_URL=https://pay.sepay.vn/v1/checkout/init
SEPAY_SUCCESS_URL=https://service.deepsi.space/dashboard/billing?status=success
SEPAY_ERROR_URL=https://service.deepsi.space/dashboard/billing?status=error
SEPAY_CANCEL_URL=https://service.deepsi.space/dashboard/billing?status=cancel
SEPAY_PAYMENT_METHOD=
```

`POST /api/v1/billing/checkout` returns `mode: "gateway"` and a form:

```json
{
  "action": "https://pay.sepay.vn/v1/checkout/init",
  "method": "POST",
  "fields": {
    "order_amount": "1499000",
    "merchant": "your-merchant-id",
    "currency": "VND",
    "operation": "PURCHASE",
    "order_description": "DeepSI Agent - Plus - DSI-20261028-AB12CD",
    "order_invoice_number": "DSI-20261028-AB12CD",
    "customer_id": "usr_...",
    "success_url": "https://service.deepsi.space/dashboard/billing?status=success&order_id=ord_...",
    "error_url": "https://service.deepsi.space/dashboard/billing?status=error&order_id=ord_...",
    "cancel_url": "https://service.deepsi.space/dashboard/billing?status=cancel&order_id=ord_...",
    "signature": "base64-hmac"
  }
}
```

The signature is HMAC-SHA256 with `SEPAY_CHECKOUT_SECRET` over the fields in this fixed order, joined by commas, skipping empty fields:

```
order_amount,merchant,currency,operation,order_description,order_invoice_number,customer_id,payment_method,success_url,error_url,cancel_url
```

Example canonical string:

```
order_amount=1499000,merchant=MERCHANT,currency=VND,operation=PURCHASE,order_description=...,order_invoice_number=...,customer_id=...,success_url=...,error_url=...,cancel_url=...
```

The frontend should render a hidden HTML form and submit it. Do not send the signature from the browser manually; use the fields returned by the API.

## Bank-transfer fallback

If gateway credentials are absent but `SEPAY_BANK_ACCOUNT` is set, checkout returns `mode: "bank_transfer"`:

```env
SEPAY_BANK_CODE=VCB
SEPAY_BANK_NAME=Vietcombank
SEPAY_BANK_ACCOUNT=0123456789
SEPAY_BANK_HOLDER=CONG TY DEEPSI
SEPAY_QR_BASE_URL=https://qr.sepay.vn/img
```

The response contains `bank_transfer.qr_url`, which encodes the account, bank, amount, and payment code. The user transfers the exact amount with the payment code in the description. The same webhook confirms it.

## Local testing

Enable mock payments:

```env
PAYMENT_MOCK_ENABLED=true
```

`POST /api/v1/billing/checkout` then returns `mode: "mock"`. Complete it with:

```bash
curl -X POST http://localhost:8080/api/v1/billing/orders/ord_.../mock-complete \
  -H "Authorization: Bearer <access_token>"
```

To test the webhook without SePay, use `SEPAY_WEBHOOK_AUTH_MODE=none` in development and post a payload:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/sepay \
  -H "Content-Type: application/json" \
  -d '{
    "id": 123456,
    "transferType": "in",
    "transferAmount": 1499000,
    "code": "DSI1A2B3C4D",
    "content": "DSI1A2B3C4D"
  }'
```

Create an order first so the payment code exists. A successful response is `{"success":true}`.

## Idempotency and retries

- `webhook_events.sepay_id` has a unique index.
- A duplicate event is acknowledged without re-applying the payment.
- If an event previously failed, a retry is allowed to process it again.
- If plan activation fails after the order is marked paid, the order is returned to `pending` so the retry can finish.
