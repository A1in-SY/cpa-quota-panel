package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestParsePercentWindows(t *testing.T) {
	body := `{"usage":{"rolling":{"status":"ok","percent":42.5,"resetsAt":"2026-08-18T10:00:00Z"},"weekly":{"status":"ok","percent":30,"resetsAt":"2026-08-20T00:00:00Z"},"monthly":{"status":"rate-limited","percent":100,"resetsAt":"2026-09-01T00:00:00Z"}}}`
	out := &quotaData{Kind: "percent-windows"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Windows["rolling"].Percent != 42.5 || out.Windows["monthly"].Status != "rate-limited" {
		t.Fatalf("windows = %+v", out.Windows)
	}
}

func TestParseBalance(t *testing.T) {
	body := `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"12.345","granted_balance":"0","topped_up_balance":"12.345"}]}`
	out := &quotaData{Kind: "balance"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Balance == nil || out.Balance.Total != "12.345" || out.Balance.Currency != "CNY" {
		t.Fatalf("balance = %+v", out.Balance)
	}
}

func TestParseGrants(t *testing.T) {
	body := `{"grants":[{"remaining":"9.6","expires_at":"2026-09-01"}]}`
	out := &quotaData{Kind: "grants"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Grants) != 1 || out.Grants[0].Remaining != "9.6" {
		t.Fatalf("grants = %+v", out.Grants)
	}
}

func TestParseQuotaPayloadError(t *testing.T) {
	out := &quotaData{Kind: "percent-windows"}
	if err := parseQuotaPayload(out, []byte(`not json`)); err == nil {
		t.Fatal("invalid JSON must error")
	}
}

func TestClassifyQuotaError(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusUnauthorized, "bad key", "invalid key"},
		{http.StatusForbidden, "nope", "forbidden"},
		{http.StatusTooManyRequests, "slow", "rate limited"},
		{http.StatusInternalServerError, "boom", "HTTP 500"},
		{http.StatusForbidden, `{"type":"...error-1010..."}`, "Cloudflare"},
	}
	for _, c := range cases {
		got := classifyQuotaError(c.status, []byte(c.body))
		if !strings.Contains(got, c.want) {
			t.Fatalf("classify(%d) = %q, want contains %q", c.status, got, c.want)
		}
	}
}

func TestFetchOneQuotaWithoutHostBridge(t *testing.T) {
	rt := buildRuntimeForTest()
	e := &scannedEntry{Kind: "percent-windows", QuotaURL: "https://opencode.ai/zen/go/v1/usage", APIKey: "k"}
	d := fetchOneQuota(rt, e)
	if d == nil || d.Err == "" || d.FetchedAt == 0 {
		t.Fatalf("expected graceful error entry, got %+v", d)
	}
}

func buildRuntimeForTest() *runtime {
	cfg := pluginConfig{}
	if err := normalizeAndValidate(&cfg); err != nil {
		panic(err)
	}
	return buildRuntime(&cfg)
}
