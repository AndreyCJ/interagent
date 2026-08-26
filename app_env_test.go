package main

import (
	"os"
	"testing"
)

func TestEnvVarAgent_WithAPIKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key-123")
	t.Setenv("LLM_BASE_URL", "https://custom.api.com/v1")
	t.Setenv("LLM_MODEL", "custom-model")

	cfg, ok := envAgentConfig()
	if !ok {
		t.Fatal("expected envAgentConfig() to return true when LLM_API_KEY is set")
	}
	if cfg.Provider != "openai-compatible" {
		t.Errorf("provider = %q, want openai-compatible", cfg.Provider)
	}
	if cfg.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("baseURL = %q, want https://custom.api.com/v1", cfg.BaseURL)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("model = %q, want custom-model", cfg.Model)
	}
}

func TestEnvVarAgent_Defaults(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg, ok := envAgentConfig()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if cfg.BaseURL != "https://api.deepseek.com" {
		t.Errorf("baseURL = %q, want default deepseek URL", cfg.BaseURL)
	}
	if cfg.Model != "deepseek-chat" {
		t.Errorf("model = %q, want deepseek-chat", cfg.Model)
	}
}

func TestEnvVarAgent_NotSet(t *testing.T) {
	os.Unsetenv("LLM_API_KEY")
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("LLM_MODEL")

	_, ok := envAgentConfig()
	if ok {
		t.Fatal("expected envAgentConfig() to return false when LLM_API_KEY is not set")
	}
}
