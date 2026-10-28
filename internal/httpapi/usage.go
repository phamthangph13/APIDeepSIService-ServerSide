package httpapi

import (
	"net/http"
	"strconv"

	"github.com/deepsi/agent-service/internal/domain"
)

func (h *Handler) UsageSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	rangeDays := parseInt(r.URL.Query().Get("range"), 30)
	if rangeDays != 7 && rangeDays != 30 && rangeDays != 90 {
		rangeDays = 30
	}
	summary, err := h.usage.Summary(r.Context(), user, rangeDays)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) UsageToday(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	summary, err := h.usage.Today(r.Context(), user)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) UsageActivity(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.Unauthorized("missing_token", "Bạn cần đăng nhập để tiếp tục."))
		return
	}
	limit := int64(parseInt(r.URL.Query().Get("limit"), 7))
	rows, err := h.usage.Activity(r.Context(), user, limit)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
