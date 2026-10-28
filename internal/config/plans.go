package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/deepsi/agent-service/internal/domain"
)

// LoadPlans builds the plan catalog from defaults and optional environment
// overrides. Keeping the catalog here makes prices/quotas deploy-time config
// instead of hardcoded values scattered through handlers.
func LoadPlans() *domain.PlanCatalog {
	plans := domain.DefaultPlans()
	for i := range plans {
		prefix := "PLAN_" + strings.ToUpper(string(plans[i].ID)) + "_"

		plans[i].Name = env(prefix+"NAME", plans[i].Name)
		plans[i].Cadence = env(prefix+"CADENCE", plans[i].Cadence)
		plans[i].Tagline = env(prefix+"TAGLINE", plans[i].Tagline)
		plans[i].Description = env(prefix+"DESCRIPTION", plans[i].Description)

		priceChanged := false
		if value, ok := lookupInt(prefix + "PRICE_VND"); ok {
			plans[i].PriceVND = value
			plans[i].PriceLabel = formatVND(value)
			priceChanged = true
		}
		if value := env(prefix+"PRICE_LABEL", ""); value != "" {
			plans[i].PriceLabel = value
		} else if priceChanged {
			plans[i].PriceLabel = formatVND(plans[i].PriceVND)
		}

		quotaChanged := false
		if value, ok := lookupInt(prefix + "MONTHLY_TOKENS"); ok {
			plans[i].MonthlyTokens = value
			quotaChanged = true
		}
		if value, ok := lookupInt(prefix + "DAILY_TOKENS"); ok {
			plans[i].DailyTokens = value
			quotaChanged = true
		}
		if value, ok := lookupInt(prefix + "KEY_LIMIT"); ok {
			plans[i].KeyLimit = int(value)
		}
		if value, ok := lookupInt(prefix + "RETENTION_DAYS"); ok {
			plans[i].RetentionDays = int(value)
		}
		if value := env(prefix+"QUOTA_LABEL", ""); value != "" {
			plans[i].QuotaLabel = value
		} else if quotaChanged {
			plans[i].QuotaLabel = defaultQuotaLabel(plans[i])
		}
		if value := env(prefix+"FEATURES", ""); value != "" {
			plans[i].Features = splitCSV(value)
		}
	}
	return domain.NewPlanCatalog(plans)
}

func defaultQuotaLabel(plan domain.Plan) string {
	if plan.ID == domain.PlanFree {
		return formatThousands(plan.DailyTokens) + " token/ngày"
	}
	return formatThousands(plan.MonthlyTokens) + " token/tháng"
}

func lookupInt(key string) (int64, bool) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func formatVND(value int64) string {
	if value == 0 {
		return "0đ"
	}
	return formatThousands(value) + "đ"
}

func formatThousands(value int64) string {
	raw := strconv.FormatInt(value, 10)
	var b strings.Builder
	for i, r := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (c *Config) PlanSummary() string {
	parts := make([]string, 0, len(c.Plans.List()))
	for _, plan := range c.Plans.List() {
		parts = append(parts, fmt.Sprintf("%s=%dđ", plan.Name, plan.PriceVND))
	}
	return strings.Join(parts, ", ")
}
