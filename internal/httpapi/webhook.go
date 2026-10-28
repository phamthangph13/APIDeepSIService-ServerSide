package httpapi

import (
	"io"
	"net/http"

	"github.com/deepsi/agent-service/internal/service"
)

// SePayWebhook receives bank-transfer notifications. SePay expects a 2xx
// response with {"success": true}; anything else is retried. Authentication is
// handled here rather than by the normal user middleware because SePay uses its
// own API key or HMAC signature.
func (h *Handler) SePayWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeRawJSON(w, http.StatusBadRequest, map[string]any{"success": false})
		return
	}
	headers := service.WebhookHeaders{
		Authorization: r.Header.Get("Authorization"),
		Timestamp:     r.Header.Get("X-SePay-Timestamp"),
		Signature:     r.Header.Get("X-SePay-Signature"),
	}
	if !h.webhook.Verify(headers, body) {
		h.log.Warn("sepay webhook rejected: invalid authentication",
			"ip", r.RemoteAddr,
			"request_id", requestID(r.Context()),
		)
		writeRawJSON(w, http.StatusUnauthorized, map[string]any{"success": false})
		return
	}
	if err := h.webhook.Process(r.Context(), body); err != nil {
		h.log.Error("sepay webhook processing failed", "error", err, "request_id", requestID(r.Context()))
		writeRawJSON(w, http.StatusInternalServerError, map[string]any{"success": false})
		return
	}
	writeRawJSON(w, http.StatusOK, map[string]any{"success": true})
}
