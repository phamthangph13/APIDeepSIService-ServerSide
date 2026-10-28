package config

import (
	"testing"

	"github.com/deepsi/agent-service/internal/domain"
)

func TestLoadPlansDefaults(t *testing.T) {
	catalog := LoadPlans()
	plans := catalog.List()
	if len(plans) != 3 {
		t.Fatalf("got %d plans, want 3", len(plans))
	}
	free := catalog.MustGet(domain.PlanFree)
	if free.PriceVND != 0 || free.MonthlyTokens != 300_000 || free.DailyTokens != 10_000 || free.KeyLimit != 5 {
		t.Fatalf("unexpected free plan: %+v", free)
	}
	plus := catalog.MustGet(domain.PlanPlus)
	if plus.PriceVND != 1_499_000 || plus.PriceLabel != "1.499.000đ" {
		t.Fatalf("unexpected plus plan: %+v", plus)
	}
	ultral := catalog.MustGet(domain.PlanUltral)
	if !ultral.UnlimitedKeys() {
		t.Fatal("ultral should have unlimited keys")
	}
}

func TestLoadPlansOverrides(t *testing.T) {
	t.Setenv("PLAN_PLUS_PRICE_VND", "1234567")
	t.Setenv("PLAN_PLUS_MONTHLY_TOKENS", "9000000")
	t.Setenv("PLAN_PLUS_DAILY_TOKENS", "300000")
	t.Setenv("PLAN_PLUS_KEY_LIMIT", "42")
	t.Setenv("PLAN_PLUS_RETENTION_DAYS", "60")

	plus := LoadPlans().MustGet(domain.PlanPlus)
	if plus.PriceVND != 1_234_567 {
		t.Fatalf("PriceVND = %d", plus.PriceVND)
	}
	if plus.PriceLabel != "1.234.567đ" {
		t.Fatalf("PriceLabel = %q", plus.PriceLabel)
	}
	if plus.MonthlyTokens != 9_000_000 || plus.DailyTokens != 300_000 {
		t.Fatalf("unexpected quota: %+v", plus)
	}
	if plus.QuotaLabel != "9.000.000 token/tháng" {
		t.Fatalf("QuotaLabel = %q", plus.QuotaLabel)
	}
	if plus.KeyLimit != 42 || plus.RetentionDays != 60 {
		t.Fatalf("unexpected limits: %+v", plus)
	}
}

func TestLoadPlansCustomLabelsAndFeatures(t *testing.T) {
	t.Setenv("PLAN_FREE_QUOTA_LABEL", "5.000 token/ngày")
	t.Setenv("PLAN_FREE_FEATURES", "Một, Hai, Ba")

	free := LoadPlans().MustGet(domain.PlanFree)
	if free.QuotaLabel != "5.000 token/ngày" {
		t.Fatalf("QuotaLabel = %q", free.QuotaLabel)
	}
	if len(free.Features) != 3 || free.Features[1] != "Hai" {
		t.Fatalf("Features = %#v", free.Features)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, b ,, c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("splitCSV() = %#v", got)
	}
}
