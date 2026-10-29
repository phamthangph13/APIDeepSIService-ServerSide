package config

import "testing"

func TestValidateLLMGatewayRequiresCentralRouteInProduction(t *testing.T) {
	cases := []struct {
		name string
		llm  LLMConfig
	}{
		{name: "disabled", llm: LLMConfig{}},
		{name: "missing base url", llm: LLMConfig{Enabled: true, APIKey: "gateway-key"}},
		{name: "missing gateway key", llm: LLMConfig{Enabled: true, BaseURL: "http://127.0.0.1:8085/v1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{App: AppConfig{Env: "production"}, LLM: tc.llm}
			if err := cfg.validateLLMGateway(); err == nil {
				t.Fatal("validateLLMGateway accepted an incomplete production configuration")
			}
		})
	}
}

func TestValidateLLMGatewayAcceptsCompleteProductionRoute(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Env: "production"},
		LLM: LLMConfig{Enabled: true, BaseURL: "http://127.0.0.1:8085/v1", APIKey: "gateway-key"},
	}
	if err := cfg.validateLLMGateway(); err != nil {
		t.Fatalf("validateLLMGateway rejected complete configuration: %v", err)
	}
}

func TestValidateLLMGatewayAllowsDevelopmentWithoutGateway(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "development"}}
	if err := cfg.validateLLMGateway(); err != nil {
		t.Fatalf("validateLLMGateway rejected development defaults: %v", err)
	}
}
