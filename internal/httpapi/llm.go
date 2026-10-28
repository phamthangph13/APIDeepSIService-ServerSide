package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/httpx"
	"github.com/deepsi/agent-service/internal/service"
)

// LLMProxy exposes an OpenAI-compatible surface and forwards requests to the
// upstream configured with LLM_BASE_URL. It records token usage after each
// request so quotas and dashboards reflect real traffic.
type LLMProxy struct {
	cfg    *config.Config
	log    *slog.Logger
	usage  *service.UsageService
	client *http.Client
}

func NewLLMProxy(cfg *config.Config, log *slog.Logger, usage *service.UsageService) *LLMProxy {
	return &LLMProxy{
		cfg:   cfg,
		log:   log,
		usage: usage,
		client: &http.Client{
			Timeout: cfg.LLM.Timeout,
		},
	}
}

type proxyUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
}

type proxyResponse struct {
	Model string     `json:"model"`
	Usage proxyUsage `json:"usage"`
}

type proxyRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func (p *LLMProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.User(r.Context())
	if !ok {
		writeOpenAIError(w, http.StatusUnauthorized, "invalid_api_key", "Thiếu API key.")
		return
	}
	key, ok := httpx.APIKey(r.Context())
	if !ok {
		writeOpenAIError(w, http.StatusUnauthorized, "invalid_api_key", "Thiếu API key.")
		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "Không đọc được nội dung yêu cầu.")
		return
	}
	var parsed proxyRequest
	_ = json.Unmarshal(body, &parsed)

	scope := scopeForPath(r.URL.Path)
	if !key.HasScope(scope) {
		writeOpenAIError(w, http.StatusForbidden, "insufficient_scope",
			fmt.Sprintf("API key không có quyền %s.", scope))
		return
	}
	if bytes.Contains(body, []byte(`"image_url"`)) && !key.HasScope("vision") {
		writeOpenAIError(w, http.StatusForbidden, "insufficient_scope", "API key không có quyền vision.")
		return
	}

	if used, err := p.usage.CheckQuota(r.Context(), user); err != nil {
		appErr := domain.AsError(err)
		message := "Đã hết hạn mức token."
		code := "insufficient_quota"
		if appErr != nil {
			message = appErr.Message
			code = appErr.Code
		}
		p.log.Warn("llm request rejected by quota", "user_id", user.ID, "used", used.Tokens)
		writeOpenAIError(w, http.StatusTooManyRequests, code, message)
		return
	}

	if r.URL.Path == "/v1/models" && (!p.cfg.LLM.Enabled || p.cfg.LLM.BaseURL == "") {
		p.writeStaticModels(w)
		return
	}
	if !p.cfg.LLM.Enabled || p.cfg.LLM.BaseURL == "" {
		writeOpenAIError(w, http.StatusServiceUnavailable, "upstream_not_configured",
			"Upstream LLM chưa được cấu hình. Liên hệ quản trị viên.")
		return
	}

	target := p.targetURL(r.URL.Path, r.URL.RawQuery)
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "proxy_error", "Không tạo được yêu cầu upstream.")
		return
	}
	copyRequestHeaders(request.Header, r.Header)
	request.Header.Set("Content-Type", "application/json")
	if p.cfg.LLM.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.cfg.LLM.APIKey)
	} else {
		request.Header.Del("Authorization")
	}

	started := time.Now()
	response, err := p.client.Do(request)
	if err != nil {
		p.log.Error("llm upstream request failed", "error", err, "path", r.URL.Path)
		p.record(user, key, r.URL.Path, parsed.Model, proxyUsage{}, http.StatusBadGateway, "upstream_error", started, r)
		writeOpenAIError(w, http.StatusBadGateway, "upstream_error", "Không kết nối được upstream LLM.")
		return
	}
	defer response.Body.Close()

	copyResponseHeaders(w.Header(), response.Header)
	streaming := parsed.Stream || strings.Contains(response.Header.Get("Content-Type"), "text/event-stream")

	if response.StatusCode >= 400 {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(errorBody)
		p.record(user, key, r.URL.Path, parsed.Model, proxyUsage{}, response.StatusCode, "upstream_error", started, r)
		return
	}

	if streaming {
		w.WriteHeader(response.StatusCode)
		usage := p.streamResponse(w, response.Body)
		p.record(user, key, r.URL.Path, parsed.Model, usage, response.StatusCode, "", started, r)
		return
	}

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		writeOpenAIError(w, http.StatusBadGateway, "upstream_error", "Không đọc được phản hồi upstream.")
		p.record(user, key, r.URL.Path, parsed.Model, proxyUsage{}, http.StatusBadGateway, "upstream_error", started, r)
		return
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(responseBody)

	var decoded proxyResponse
	_ = json.Unmarshal(responseBody, &decoded)
	model := decoded.Model
	if model == "" {
		model = parsed.Model
	}
	p.record(user, key, r.URL.Path, model, decoded.Usage, response.StatusCode, "", started, r)
}

func (p *LLMProxy) streamResponse(w http.ResponseWriter, body io.Reader) proxyUsage {
	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var usage proxyUsage
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := w.Write(line); err != nil {
			break
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			break
		}
		if flusher != nil {
			flusher.Flush()
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}
			var chunk proxyResponse
			if json.Unmarshal(payload, &chunk) == nil && (chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0) {
				usage = chunk.Usage
			}
		}
	}
	return usage
}

func (p *LLMProxy) record(
	user *domain.User,
	key *domain.APIKey,
	endpoint, model string,
	usage proxyUsage,
	status int,
	errorCode string,
	started time.Time,
	r *http.Request,
) {
	prompt := usage.PromptTokens
	if prompt == 0 {
		prompt = usage.InputTokens
	}
	completion := usage.CompletionTokens
	if completion == 0 {
		completion = usage.OutputTokens
	}
	total := usage.TotalTokens
	if total == 0 {
		total = prompt + completion
	}
	if model == "" {
		model = "unknown"
	}
	plan := p.cfg.Plans.MustGet(user.EffectivePlan(time.Now()))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logEntry := &domain.UsageLog{
		UserID:           user.ID,
		APIKeyID:         key.ID,
		RequestID:        requestID(r.Context()),
		Model:            model,
		Endpoint:         endpoint,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		LatencyMS:        time.Since(started).Milliseconds(),
		Status:           status,
		ErrorCode:        errorCode,
		CreatedAt:        time.Now(),
	}
	if err := p.usage.Record(ctx, logEntry, plan); err != nil {
		p.log.Error("record usage failed", "error", err, "user_id", user.ID)
	}
}

func (p *LLMProxy) targetURL(path, rawQuery string) string {
	base := strings.TrimRight(p.cfg.LLM.BaseURL, "/")
	trimmed := strings.TrimPrefix(path, "/v1")
	target := base
	if !strings.HasSuffix(base, "/v1") {
		target += "/v1"
	}
	target += trimmed
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	return target
}

func (p *LLMProxy) writeStaticModels(w http.ResponseWriter) {
	now := time.Now().Unix()
	data := make([]map[string]any, 0, len(p.cfg.LLM.Models))
	for _, id := range p.cfg.LLM.Models {
		data = append(data, map[string]any{
			"id":       id,
			"object":   "model",
			"created":  now,
			"owned_by": "deepsi",
		})
	}
	writeRawJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	return io.ReadAll(io.LimitReader(r.Body, 20<<20))
}

func scopeForPath(path string) string {
	switch {
	case strings.Contains(path, "/embeddings"):
		return "embeddings"
	case strings.HasSuffix(path, "/models"):
		return "models"
	default:
		return "chat"
	}
}

func copyRequestHeaders(dst, src http.Header) {
	for _, key := range []string{"Accept", "OpenAI-Beta", "OpenAI-Organization", "X-Request-ID"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for _, key := range []string{"Content-Type", "Cache-Control", "Retry-After", "OpenAI-Version", "OpenAI-Organization", "X-Request-ID"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func writeOpenAIError(w http.ResponseWriter, status int, code, message string) {
	errorType := "invalid_request_error"
	switch status {
	case http.StatusUnauthorized:
		errorType = "authentication_error"
	case http.StatusForbidden:
		errorType = "permission_error"
	case http.StatusTooManyRequests:
		errorType = "insufficient_quota"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		errorType = "server_error"
	}
	writeRawJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"code":    code,
		},
	})
}
