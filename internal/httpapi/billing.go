package httpapi

import (
	"net/http"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/go-chi/chi/v5"
)

type checkoutRequest struct {
	PlanID string `json:"plan_id"`
}

func (h *Handler) Plans(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.cfg.Plans.List())
}

func (h *Handler) BillingOverview(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	overview, err := h.billing.Overview(r.Context(), user)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	var body checkoutRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	planID, ok := h.cfg.Plans.Parse(body.PlanID)
	if !ok {
		writeError(w, r, domain.Invalid("invalid_plan", "Gói dịch vụ không hợp lệ."))
		return
	}
	checkout, err := h.billing.Checkout(r.Context(), user, planID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, checkout)
}

func (h *Handler) OrderStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	order, err := h.billing.OrderStatus(r.Context(), user, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) CompleteMockPayment(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	order, err := h.billing.CompleteMockPayment(r.Context(), user, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *Handler) CancelRenewal(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	if err := h.billing.CancelRenewal(r.Context(), user); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Đã hủy gia hạn. Gói vẫn có hiệu lực đến hết kỳ đã thanh toán.",
	})
}

func (h *Handler) Downgrade(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	if err := h.billing.Downgrade(r.Context(), user); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Đã chuyển về gói Free. Các API key hiện có vẫn hoạt động.",
	})
}
