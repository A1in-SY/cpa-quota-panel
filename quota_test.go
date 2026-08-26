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

func TestParseCodingPlan(t *testing.T) {
	body := `{"model_remains":[
		{"model_name":"general","remains_time":14843648,"weekly_remains_time":342443648,
		 "current_interval_total_count":10,"current_interval_usage_count":3,
		 "current_weekly_total_count":100,"current_weekly_usage_count":20,
		 "current_interval_status":1,"current_weekly_status":1,
		 "current_interval_remaining_percent":100,"current_weekly_remaining_percent":99}
	],"base_resp":{"status_code":0,"status_msg":"success"}}`
	out := &quotaData{Kind: "coding-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Models) != 1 {
		t.Fatalf("models = %+v", out.Models)
	}
	m := out.Models[0]
	if m.Name != "general" || m.IntervalPercent != 100 || m.WeeklyPercent != 99 {
		t.Fatalf("model = %+v", m)
	}
	// remains_time is in millis -> stored seconds.
	if m.IntervalRemain != 14843 || m.WeeklyRemain != 342443 {
		t.Fatalf("remain seconds = %d / %d", m.IntervalRemain, m.WeeklyRemain)
	}
	if m.IntervalUsed != 3 || m.IntervalTotal != 10 || m.WeeklyUsed != 20 || m.WeeklyTotal != 100 {
		t.Fatalf("counts = %+v", m)
	}
}

func TestParseCodingPlanEmpty(t *testing.T) {
	out := &quotaData{Kind: "coding-plan"}
	if err := parseQuotaPayload(out, []byte(`{"base_resp":{"status_code":0}}`)); err == nil {
		t.Fatal("empty model_remains must error")
	}
}

func TestParseZhipuPlan(t *testing.T) {
	// Shape per a live response from open.bigmodel.cn/api/monitor/usage/quota/limit:
	// CREDIT_LIMIT windows carry unit (3 = 5-hour, 6 = weekly); percentage is used %.
	body := `{"code":200,"msg":"操作成功","success":true,"data":{"level":"lite","limits":[
		{"type":"CREDIT_LIMIT","unit":6,"number":1,"usage":2000,"currentValue":242,"remaining":1757,"percentage":12,"nextResetTime":1785000000000},
		{"type":"CREDIT_LIMIT","unit":3,"number":5,"usage":2000,"currentValue":242,"remaining":1757,"percentage":44,"nextResetTime":1784400000000},
		{"type":"TIME_LIMIT","percentage":7,"usage":1000,"currentValue":72,"remaining":928}
	]}}`
	out := &quotaData{Kind: "zhipu-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Windows["rolling"].Percent != 44 {
		t.Fatalf("rolling = %+v", out.Windows["rolling"])
	}
	if out.Windows["weekly"].Percent != 12 {
		t.Fatalf("weekly = %+v", out.Windows["weekly"])
	}
	if out.Windows["monthly"].Percent != 7 {
		t.Fatalf("monthly = %+v", out.Windows["monthly"])
	}
	if out.Windows["weekly"].ResetsAt == "" {
		t.Fatalf("weekly resetsAt missing: %+v", out.Windows["weekly"])
	}
}

func TestParseZhipuPlanUnitFallbackByResetTime(t *testing.T) {
	// Without the unit field the soonest reset is the 5-hour window.
	body := `{"code":200,"success":true,"data":{"level":"pro","limits":[
		{"type":"TOKENS_LIMIT","percentage":53,"nextResetTime":1785000000000},
		{"type":"TOKENS_LIMIT","percentage":44,"nextResetTime":1784400000000}
	]}}`
	out := &quotaData{Kind: "zhipu-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Windows["rolling"].Percent != 44 || out.Windows["weekly"].Percent != 53 {
		t.Fatalf("windows = %+v", out.Windows)
	}
}

func TestParseZhipuPlanCreditLimitAlias(t *testing.T) {
	// Newer plans renamed TOKENS_LIMIT to CREDIT_LIMIT.
	body := `{"code":200,"success":true,"data":{"level":"lite","limits":[
		{"type":"CREDIT_LIMIT","percentage":12,"nextResetTime":1785000000000}
	]}}`
	out := &quotaData{Kind: "zhipu-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Windows["rolling"].Percent != 12 {
		t.Fatalf("rolling = %+v", out.Windows["rolling"])
	}
	if _, ok := out.Windows["weekly"]; ok {
		t.Fatalf("weekly must be absent for a single window: %+v", out.Windows)
	}
	if _, ok := out.Windows["monthly"]; ok {
		t.Fatalf("monthly must be absent without percentage: %+v", out.Windows)
	}
}

func TestParseZhipuPlanAuthErrorViaBodyCode(t *testing.T) {
	// The endpoint returns HTTP 200 even on bad keys; the body code carries the error.
	body := `{"code":401,"msg":"令牌已过期或验证不正确","success":false}`
	out := &quotaData{Kind: "zhipu-plan"}
	err := parseQuotaPayload(out, []byte(body))
	if err == nil {
		t.Fatal("body code 401 must error")
	}
	if !strings.Contains(err.Error(), "zhipu-plan error 401") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseZhipuPlanNoTokenLimits(t *testing.T) {
	body := `{"code":200,"msg":"操作成功","success":true,"data":{"limits":[{"type":"TIME_LIMIT","usage":1000,"currentValue":72}]}}`
	out := &quotaData{Kind: "zhipu-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err == nil {
		t.Fatal("missing token limits must error")
	}
}

func TestFmtDuration(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{0, "0 分钟"},
		{1500, "25 分钟"},
		{3600, "1 小时"},
		{5400, "1 小时 30 分钟"},
	}
	for _, c := range cases {
		if got := fmtDuration(c.sec); got != c.want {
			t.Fatalf("fmtDuration(%d) = %q, want %q", c.sec, got, c.want)
		}
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
