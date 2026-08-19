package main

import (
	"testing"
)

func TestNormalizeAndValidateDefaults(t *testing.T) {
	cfg := pluginConfig{}
	if err := normalizeAndValidate(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConfigPath != "/CLIProxyAPI/config.yaml" {
		t.Fatalf("default config-path = %q", cfg.ConfigPath)
	}
	if cfg.CacheTTL != 300 {
		t.Fatalf("default cache-ttl = %d", cfg.CacheTTL)
	}
	if len(cfg.Sources) != 3 {
		t.Fatalf("default sources = %d", len(cfg.Sources))
	}
	ids := map[string]bool{}
	for _, s := range cfg.Sources {
		ids[s.ID] = true
		if s.Name == "" || s.QuotaURL == "" || len(s.MatchBaseURLs) == 0 || s.Kind == "" {
			t.Fatalf("default source incomplete: %+v", s)
		}
	}
	if !ids["opencode"] || !ids["deepseek"] || !ids["minimax"] {
		t.Fatalf("default sources missing vendor: %v", ids)
	}
}

func TestNormalizeAndValidateBulk(t *testing.T) {
	base := []QuotaSource{{
		ID: "x", Name: "X", MatchBaseURLs: []string{"https://x.com/"}, QuotaURL: "https://x.com/q", Kind: "balance",
	}}
	// Duplicate id must fail.
	dup := pluginConfig{Sources: append(append([]QuotaSource{}, base...), base[0])}
	if err := normalizeAndValidate(&dup); err == nil {
		t.Fatal("duplicate source id must error")
	}
	// Missing match-base-urls must fail.
	empty := pluginConfig{Sources: []QuotaSource{{ID: "x", QuotaURL: "u", Kind: "balance"}}}
	if err := normalizeAndValidate(&empty); err == nil {
		t.Fatal("missing match-base-urls must error")
	}
	// Missing quota-url must fail.
	noURL := pluginConfig{Sources: []QuotaSource{{ID: "x", MatchBaseURLs: []string{"u"}, Kind: "balance"}}}
	if err := normalizeAndValidate(&noURL); err == nil {
		t.Fatal("missing quota-url must error")
	}
	// Invalid kind must fail.
	badKind := pluginConfig{Sources: []QuotaSource{{ID: "x", MatchBaseURLs: []string{"u"}, QuotaURL: "u", Kind: "nope"}}}
	if err := normalizeAndValidate(&badKind); err == nil {
		t.Fatal("invalid kind must error")
	}
	// Trailing slash normalized.
	ok := pluginConfig{Sources: []QuotaSource{{ID: "x", MatchBaseURLs: []string{"https://x.com/"}, QuotaURL: "u", Kind: "balance"}}}
	if err := normalizeAndValidate(&ok); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if ok.Sources[0].MatchBaseURLs[0] != "https://x.com" {
		t.Fatalf("match base-url not normalized: %q", ok.Sources[0].MatchBaseURLs[0])
	}
}

func TestMatchBaseURL(t *testing.T) {
	if !matchBaseURL("https://api.deepseek.com", []string{"https://api.deepseek.com"}) {
		t.Fatal("exact match failed")
	}
	if !matchBaseURL("https://api.deepseek.com/anthropic", []string{"https://api.deepseek.com"}) {
		t.Fatal("path-prefix match failed")
	}
	if matchBaseURL("https://api.deepseek.com-evil.example", []string{"https://api.deepseek.com"}) {
		t.Fatal("prefix must not match across a non-boundary")
	}
	if !matchBaseURL("https://opencode.ai/zen/go/v1/", []string{"https://opencode.ai/zen/go/v1"}) {
		t.Fatal("trailing slash normalization failed")
	}
	if matchBaseURL("https://api.other.com", []string{"https://opencode.ai/zen/go/v1"}) {
		t.Fatal("unrelated URL must not match")
	}
}
