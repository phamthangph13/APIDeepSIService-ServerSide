package service

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/google/uuid"
)

type UsageService struct {
	cfg   *config.Config
	usage domain.UsageRepository
}

func NewUsageService(cfg *config.Config, usage domain.UsageRepository) *UsageService {
	return &UsageService{cfg: cfg, usage: usage}
}

func (s *UsageService) Record(ctx context.Context, log *domain.UsageLog, plan domain.Plan) error {
	if log.ID == "" {
		log.ID = "use_" + uuid.NewString()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.TotalTokens == 0 {
		log.TotalTokens = log.PromptTokens + log.CompletionTokens
	}
	retention := plan.RetentionDays
	if retention <= 0 {
		retention = 7
	}
	log.ExpiresAt = log.CreatedAt.AddDate(0, 0, retention)
	return s.usage.Insert(ctx, log)
}

func (s *UsageService) Summary(ctx context.Context, user *domain.User, rangeDays int) (*domain.UsageSummary, error) {
	if rangeDays != 1 && rangeDays != 7 && rangeDays != 30 && rangeDays != 90 {
		rangeDays = 30
	}
	now := time.Now().In(s.cfg.App.Timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.cfg.App.Timezone)
	from := today.AddDate(0, 0, -(rangeDays - 1))
	to := today.AddDate(0, 0, 1)

	summary, err := s.usage.Summary(ctx, user.ID, from, to, s.cfg.App.Timezone.String())
	if err != nil {
		return nil, err
	}
	plan := s.cfg.Plans.MustGet(user.EffectivePlan(now))
	summary.Quota = plan.MonthlyTokens
	if plan.MonthlyTokens > 0 {
		summary.UsedRatio = math.Min(1, float64(summary.TotalTokens)/float64(plan.MonthlyTokens))
	}

	// The repository only returns days with activity. The client chart needs a
	// continuous range, including zero-usage days.
	byDay := make(map[string]domain.DayUsage, len(summary.Days))
	for _, day := range summary.Days {
		byDay[day.DateKey] = day
	}
	days := make([]domain.DayUsage, 0, rangeDays)
	for i := 0; i < rangeDays; i++ {
		date := from.AddDate(0, 0, i)
		key := date.Format("2006-01-02")
		day, ok := byDay[key]
		if !ok {
			day = domain.DayUsage{DateKey: key}
		}
		day.Date = date
		day.Label = dayLabel(date, rangeDays)
		days = append(days, day)
	}
	summary.Days = days

	var totalForModels int64
	for i := range summary.Models {
		if summary.Models[i].Name == "" {
			summary.Models[i].Name = displayModelName(summary.Models[i].ID)
		}
		totalForModels += summary.Models[i].Tokens
	}
	if totalForModels > 0 {
		for i := range summary.Models {
			summary.Models[i].Share = float64(summary.Models[i].Tokens) / float64(totalForModels)
		}
	}
	sort.Slice(summary.Models, func(i, j int) bool { return summary.Models[i].Tokens > summary.Models[j].Tokens })
	return summary, nil
}

func (s *UsageService) Today(ctx context.Context, user *domain.User) (*domain.UsageSummary, error) {
	return s.Summary(ctx, user, 1)
}

func (s *UsageService) Activity(ctx context.Context, user *domain.User, limit int64) ([]domain.ActivityRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 7
	}
	return s.usage.Activity(ctx, user.ID, limit)
}

// CheckQuota enforces the Free daily quota and the paid monthly quota. It is
// intentionally simple (aggregate on demand); put a cache in front when traffic
// grows.
func (s *UsageService) CheckQuota(ctx context.Context, user *domain.User) (domain.UsageAggregate, error) {
	now := time.Now().In(s.cfg.App.Timezone)
	plan := s.cfg.Plans.MustGet(user.EffectivePlan(now))
	var from time.Time
	var quota int64
	var code, message string
	if plan.ID == domain.PlanFree {
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.cfg.App.Timezone)
		quota = plan.DailyTokens
		code = "daily_quota_exhausted"
		message = "Hạn mức token hôm nay của gói Free đã hết."
	} else {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.cfg.App.Timezone)
		quota = plan.MonthlyTokens
		code = "monthly_quota_exhausted"
		message = "Hạn mức token tháng này của gói " + plan.Name + " đã hết."
	}
	used, err := s.usage.Sum(ctx, user.ID, from, now.Add(time.Second))
	if err != nil {
		return domain.UsageAggregate{}, err
	}
	if quota > 0 && used.Tokens >= quota {
		return used, domain.NewError(429, code, message, nil)
	}
	return used, nil
}

func (s *UsageService) AdminTotals(ctx context.Context, from, to time.Time) (domain.UsageAggregate, error) {
	return s.usage.AdminTotals(ctx, from, to)
}

func dayLabel(date time.Time, rangeDays int) string {
	if rangeDays <= 7 {
		weekdays := []string{"CN", "T2", "T3", "T4", "T5", "T6", "T7"}
		return weekdays[int(date.Weekday())]
	}
	return date.Format("02/01")
}

func displayModelName(id string) string {
	switch id {
	case "deepsi-s1":
		return "DeepSI-S1"
	case "deepsi-m1":
		return "DeepSI-M1"
	case "deepsi-v1":
		return "DeepSI-V1"
	case "deepsi-e1":
		return "DeepSI-E1"
	default:
		return id
	}
}
