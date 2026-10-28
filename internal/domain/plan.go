package domain

import "strings"

type PlanID string

const (
	PlanFree   PlanID = "free"
	PlanPlus   PlanID = "plus"
	PlanUltral PlanID = "ultral"
)

type Plan struct {
	ID            PlanID   `json:"id"`
	Name          string   `json:"name"`
	PriceVND      int64    `json:"price_vnd"`
	PriceLabel    string   `json:"price_label"`
	Cadence       string   `json:"cadence"`
	Tagline       string   `json:"tagline"`
	Description   string   `json:"description"`
	QuotaLabel    string   `json:"quota_label"`
	MonthlyTokens int64    `json:"monthly_tokens"`
	DailyTokens   int64    `json:"daily_tokens"`
	KeyLimit      int      `json:"key_limit"` // 0 means unlimited
	RetentionDays int      `json:"retention_days"`
	Features      []string `json:"features"`
}

func (p Plan) UnlimitedKeys() bool { return p.KeyLimit <= 0 }

// DefaultPlans mirrors the marketing site. Prices and quotas can be overridden
// through environment variables in config.LoadPlans.
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:            PlanFree,
			Name:          "Free",
			PriceVND:      0,
			PriceLabel:    "0đ",
			Cadence:       "mãi mãi",
			Tagline:       "Đủ để một agent bắt đầu.",
			Description:   "Dành cho sinh viên, side project và những ý tưởng cần được thử trước khi cam kết.",
			QuotaLabel:    "10.000 token/ngày",
			MonthlyTokens: 300_000,
			DailyTokens:   10_000,
			KeyLimit:      5,
			RetentionDays: 7,
			Features: []string{
				"10.000 token mỗi ngày",
				"5 API key miễn phí",
				"2 mô hình: M1 và E1",
				"Log yêu cầu trong 7 ngày",
				"Tương thích OpenAI SDK",
				"Hỗ trợ qua cộng đồng",
			},
		},
		{
			ID:            PlanPlus,
			Name:          "Plus",
			PriceVND:      1_499_000,
			PriceLabel:    "1.499.000đ",
			Cadence:       "/ tháng",
			Tagline:       "Cho sản phẩm đang có người dùng.",
			Description:   "Mức phổ biến cho team nhỏ, SaaS và các agent đã qua giai đoạn thử nghiệm.",
			QuotaLabel:    "2.000.000 token/tháng",
			MonthlyTokens: 2_000_000,
			DailyTokens:   70_000,
			KeyLimit:      20,
			RetentionDays: 30,
			Features: []string{
				"2.000.000 token mỗi tháng",
				"20 API key miễn phí",
				"Tất cả mô hình S1, M1, V1, E1",
				"Ưu tiên xử lý",
				"Log yêu cầu trong 30 ngày",
				"Hỗ trợ email trong 24 giờ",
				"Tool calling & JSON mode",
			},
		},
		{
			ID:            PlanUltral,
			Name:          "Ultral",
			PriceVND:      2_499_000,
			PriceLabel:    "2.499.000đ",
			Cadence:       "/ tháng",
			Tagline:       "Khi agent là một phần của vận hành.",
			Description:   "Cho hệ thống chạy liên tục, khối lượng lớn và những yêu cầu không thể chờ.",
			QuotaLabel:    "5.000.000 token/tháng",
			MonthlyTokens: 5_000_000,
			DailyTokens:   180_000,
			KeyLimit:      0,
			RetentionDays: 90,
			Features: []string{
				"5.000.000 token mỗi tháng",
				"API key không giới hạn",
				"Tất cả mô hình + mô hình beta",
				"Ưu tiên cao nhất",
				"Log yêu cầu trong 90 ngày",
				"Hỗ trợ Slack / Telegram",
				"SLA 99,9% & báo cáo sự cố",
				"Hóa đơn VAT, hợp đồng dịch vụ",
			},
		},
	}
}

type PlanCatalog struct {
	plans  map[PlanID]Plan
	order  []PlanID
	byName map[string]PlanID
}

func NewPlanCatalog(plans []Plan) *PlanCatalog {
	c := &PlanCatalog{
		plans:  make(map[PlanID]Plan, len(plans)),
		order:  make([]PlanID, 0, len(plans)),
		byName: make(map[string]PlanID, len(plans)),
	}
	for _, plan := range plans {
		c.plans[plan.ID] = plan
		c.order = append(c.order, plan.ID)
		c.byName[strings.ToLower(plan.Name)] = plan.ID
	}
	return c
}

func (c *PlanCatalog) Get(id PlanID) (Plan, bool) {
	plan, ok := c.plans[id]
	return plan, ok
}

func (c *PlanCatalog) MustGet(id PlanID) Plan {
	plan, ok := c.plans[id]
	if !ok {
		return c.plans[PlanFree]
	}
	return plan
}

func (c *PlanCatalog) List() []Plan {
	out := make([]Plan, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.plans[id])
	}
	return out
}

func (c *PlanCatalog) Parse(value string) (PlanID, bool) {
	id := PlanID(strings.ToLower(strings.TrimSpace(value)))
	_, ok := c.plans[id]
	return id, ok
}
