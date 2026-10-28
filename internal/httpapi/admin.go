package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/go-chi/chi/v5"
)

type updateRoleRequest struct {
	Role string `json:"role"`
}

type updatePlanRequest struct {
	PlanID string `json:"plan_id"`
	Months int    `json:"months"`
}

func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(h.cfg.App.Timezone)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.cfg.App.Timezone)
	from30 := now.AddDate(0, 0, -30)

	users, err := h.users.Count(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	byPlan, err := h.users.CountByPlan(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	keys, err := h.apiKeys.CountAllActive(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	revenue, err := h.payments.Revenue(r.Context(), monthStart, now.Add(time.Minute))
	if err != nil {
		writeError(w, r, err)
		return
	}
	usage, err := h.usageLog.AdminTotals(r.Context(), from30, now)
	if err != nil {
		writeError(w, r, err)
		return
	}
	_, pending, err := h.payments.AdminListOrders(r.Context(), domain.PaymentStatusPending, 1, 0)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users_total":    users,
		"users_by_plan":  byPlan,
		"active_keys":    keys,
		"revenue_month":  revenue,
		"tokens_30d":     usage.Tokens,
		"requests_30d":   usage.Requests,
		"pending_orders": pending,
		"generated_at":   now,
		"currency":       "VND",
	})
}

func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := domain.UserFilter{
		Query:  strings.TrimSpace(query.Get("q")),
		Role:   strings.TrimSpace(query.Get("role")),
		Plan:   strings.TrimSpace(query.Get("plan")),
		Limit:  int64(parseInt(query.Get("limit"), 20)),
		Offset: int64(parseInt(query.Get("offset"), 0)),
	}
	users, total, err := h.users.List(r.Context(), filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	now := time.Now()
	from30 := now.AddDate(0, 0, -30)
	items := make([]domain.AdminUserListItem, 0, len(users))
	for _, user := range users {
		keyCount, _ := h.apiKeys.CountActive(r.Context(), user.ID)
		usage, _ := h.usageLog.Sum(r.Context(), user.ID, from30, now)
		lastSeen := user.UpdatedAt
		if user.LastLoginAt != nil {
			lastSeen = *user.LastLoginAt
		}
		items = append(items, domain.AdminUserListItem{
			User:      user.Public(),
			KeyCount:  keyCount,
			Tokens30d: usage.Tokens,
			LastSeen:  lastSeen,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *Handler) AdminUpdateRole(w http.ResponseWriter, r *http.Request) {
	actor, _ := currentUser(r)
	var body updateRoleRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	role := strings.ToLower(strings.TrimSpace(body.Role))
	if role != domain.RoleClient && role != domain.RoleAdmin {
		writeError(w, r, domain.Invalid("invalid_role", "Vai trò không hợp lệ."))
		return
	}
	targetID := chi.URLParam(r, "id")
	if actor != nil && actor.ID == targetID && role != domain.RoleAdmin {
		writeError(w, r, domain.Invalid("cannot_demote_self", "Bạn không thể tự hạ quyền quản trị của mình."))
		return
	}
	if err := h.users.SetRole(r.Context(), targetID, role); err != nil {
		writeError(w, r, err)
		return
	}
	user, err := h.users.FindByID(r.Context(), targetID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, user.Public())
}

func (h *Handler) AdminUpdatePlan(w http.ResponseWriter, r *http.Request) {
	var body updatePlanRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	planID, ok := h.cfg.Plans.Parse(body.PlanID)
	if !ok {
		writeError(w, r, domain.Invalid("invalid_plan", "Gói dịch vụ không hợp lệ."))
		return
	}
	targetID := chi.URLParam(r, "id")
	if planID == domain.PlanFree {
		if err := h.users.SetPlan(r.Context(), targetID, domain.PlanFree, nil, nil, false); err != nil {
			writeError(w, r, err)
			return
		}
	} else {
		months := body.Months
		if months <= 0 {
			months = 1
		}
		if months > 36 {
			months = 36
		}
		now := time.Now()
		start := now
		expiry := now.AddDate(0, months, 0)
		if existing, err := h.users.FindByID(r.Context(), targetID); err == nil &&
			existing.Plan == planID && existing.PlanExpiresAt != nil && existing.PlanExpiresAt.After(now) {
			expiry = existing.PlanExpiresAt.AddDate(0, months, 0)
		}
		if err := h.users.SetPlan(r.Context(), targetID, planID, &start, &expiry, false); err != nil {
			writeError(w, r, err)
			return
		}
	}
	user, err := h.users.FindByID(r.Context(), targetID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, user.Public())
}

func (h *Handler) AdminOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := int64(parseInt(query.Get("limit"), 20))
	offset := int64(parseInt(query.Get("offset"), 0))
	orders, total, err := h.payments.AdminListOrders(r.Context(), query.Get("status"), limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  orders,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) AdminUsage(w http.ResponseWriter, r *http.Request) {
	days := parseInt(r.URL.Query().Get("days"), 30)
	if days != 7 && days != 30 && days != 90 {
		days = 30
	}
	now := time.Now().In(h.cfg.App.Timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, h.cfg.App.Timezone)
	from := today.AddDate(0, 0, -(days - 1))
	to := today.AddDate(0, 0, 1)
	summary, err := h.usageLog.Summary(r.Context(), "", from, to, h.cfg.App.Timezone.String())
	if err != nil {
		writeError(w, r, err)
		return
	}
	summary.Quota = 0
	summary.UsedRatio = 0
	writeJSON(w, http.StatusOK, summary)
}
