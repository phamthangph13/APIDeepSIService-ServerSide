package domain

import "time"

type UsageLog struct {
	ID               string    `bson:"id"`
	UserID           string    `bson:"user_id"`
	APIKeyID         string    `bson:"api_key_id"`
	RequestID        string    `bson:"request_id,omitempty"`
	Model            string    `bson:"model"`
	Endpoint         string    `bson:"endpoint"`
	PromptTokens     int64     `bson:"prompt_tokens"`
	CompletionTokens int64     `bson:"completion_tokens"`
	TotalTokens      int64     `bson:"total_tokens"`
	LatencyMS        int64     `bson:"latency_ms"`
	Status           int       `bson:"status"`
	ErrorCode        string    `bson:"error_code,omitempty"`
	CreatedAt        time.Time `bson:"created_at"`
	// ExpiresAt lets MongoDB remove old logs with a TTL index. Retention is
	// plan-dependent and therefore stored per document.
	ExpiresAt time.Time `bson:"expires_at"`
}

type DayUsage struct {
	Date     time.Time `json:"-"`
	DateKey  string    `json:"date"`
	Label    string    `json:"label"`
	Tokens   int64     `json:"tokens"`
	Requests int64     `json:"requests"`
	Input    int64     `json:"input"`
	Output   int64     `json:"output"`
}

type ModelUsage struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Share    float64 `json:"share"`
	Tokens   int64   `json:"tokens"`
	Requests int64   `json:"requests"`
}

type UsageSummary struct {
	Days          []DayUsage   `json:"days"`
	TotalTokens   int64        `json:"total_tokens"`
	TotalRequests int64        `json:"total_requests"`
	AvgLatency    int64        `json:"avg_latency_ms"`
	P95Latency    int64        `json:"p95_latency_ms"`
	ErrorRate     float64      `json:"error_rate"`
	Models        []ModelUsage `json:"models"`
	Quota         int64        `json:"quota"`
	UsedRatio     float64      `json:"used_ratio"`
	InputTokens   int64        `json:"input_tokens"`
	OutputTokens  int64        `json:"output_tokens"`
}

type ActivityRow struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Model     string    `json:"model"`
	Tokens    int64     `json:"tokens"`
	LatencyMS int64     `json:"latency_ms"`
	Status    int       `json:"status"`
	Endpoint  string    `json:"endpoint"`
}

type UsageAggregate struct {
	Tokens   int64
	Requests int64
	Input    int64
	Output   int64
}
