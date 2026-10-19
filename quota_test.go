package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
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

func TestParseClinePlan(t *testing.T) {
	// Shape per api.cline.bot/api/v1/users/me/plan/usage-limits: Cline names the
	// windows five_hour/weekly/monthly and reports percentUsed (0-100).
	body := `{"success":true,"data":{"limits":[
		{"type":"five_hour","percentUsed":12.5},
		{"type":"weekly","percentUsed":24,"resetsAt":"2026-08-08T15:33:55.178045653Z"},
		{"type":"monthly","percentUsed":6,"resetsAt":"2026-08-31T15:33:55Z"}
	]}}`
	out := &quotaData{Kind: "cline-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.Windows["rolling"].Percent != 12.5 || out.Windows["weekly"].Percent != 24 || out.Windows["monthly"].Percent != 6 {
		t.Fatalf("windows = %+v", out.Windows)
	}
	// Cline stamps nanoseconds; the bar needs the second-precision RFC3339 the
	// countdown JS parses.
	if out.Windows["weekly"].ResetsAt != "2026-08-08T15:33:55Z" {
		t.Fatalf("weekly resetsAt = %q", out.Windows["weekly"].ResetsAt)
	}
	if out.Windows["rolling"].ResetsAt != "" {
		t.Fatalf("rolling resetsAt = %q, want empty (the 5-hour window omits it)", out.Windows["rolling"].ResetsAt)
	}
}

func TestParseClinePlanWithoutSubscription(t *testing.T) {
	// A key without an active ClinePass gets data:null; the card stays empty.
	body := `{"success":true,"data":null}`
	out := &quotaData{Kind: "cline-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("data:null must not error: %v", err)
	}
	if out.Windows != nil {
		t.Fatalf("windows = %+v, want none", out.Windows)
	}
}

func TestParseClinePlanEnvelopeError(t *testing.T) {
	// Failures arrive as HTTP 200 + success=false.
	body := `{"success":false,"error":"Unauthorized","data":null}`
	out := &quotaData{Kind: "cline-plan"}
	err := parseQuotaPayload(out, []byte(body))
	if err == nil {
		t.Fatal("success=false must error")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseCommandCodePlan(t *testing.T) {
	// Shape per api.commandcode.ai/alpha/billing/credits (the endpoint behind
	// the `cmd` CLI's /usage): USD credit buckets plus two rolling windows.
	body := `{
		"credits":{"monthlyCredits":38.5,"purchasedCredits":10,"freeCredits":1.5,"monthlyCreditsGranted":70},
		"windowLimits":{
			"fiveHour":{"used":4.2,"cap":14,"resetAt":"2026-10-19T12:00:00.123456789Z"},
			"weekly":{"used":31.5,"cap":35,"resetAt":1785000000000}
		}
	}`
	out := &quotaData{Kind: "commandcode-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := out.Windows["rolling"].Percent; got < 29.9 || got > 30.1 {
		t.Fatalf("rolling percent = %v, want 30", got)
	}
	if out.Windows["weekly"].Percent != 90 {
		t.Fatalf("weekly = %+v", out.Windows["weekly"])
	}
	if out.Windows["rolling"].Used != 4.2 || out.Windows["rolling"].Cap != 14 {
		t.Fatalf("rolling dollars = %+v", out.Windows["rolling"])
	}
	// ISO reset stamps are cut to the second precision the countdown JS parses.
	if out.Windows["rolling"].ResetsAt != "2026-10-19T12:00:00Z" {
		t.Fatalf("rolling resetsAt = %q", out.Windows["rolling"].ResetsAt)
	}
	if out.Windows["weekly"].ResetsAt == "" {
		t.Fatalf("weekly resetsAt missing: %+v", out.Windows["weekly"])
	}
	if out.Credits == nil || out.Credits.Monthly != 38.5 || out.Credits.Purchased != 10 ||
		out.Credits.Free != 1.5 || out.Credits.Granted != 70 {
		t.Fatalf("credits = %+v", out.Credits)
	}
}

func TestParseCommandCodePlanNestedWindowLimits(t *testing.T) {
	// Older payloads nest windowLimits under credits and may quote numbers.
	body := `{"credits":{"monthlyCredits":"12.25","windowLimits":{"fiveHour":{"used":"1.5","cap":"14"}}}}`
	out := &quotaData{Kind: "commandcode-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := out.Windows["rolling"].Percent; got < 10.7 || got > 10.8 {
		t.Fatalf("rolling percent = %v, want ~10.71", got)
	}
	if _, ok := out.Windows["weekly"]; ok {
		t.Fatalf("weekly must be absent when the payload omits it: %+v", out.Windows)
	}
	if out.Credits == nil || out.Credits.Monthly != 12.25 {
		t.Fatalf("credits = %+v", out.Credits)
	}
}

func TestParseCommandCodePlanNoData(t *testing.T) {
	out := &quotaData{Kind: "commandcode-plan"}
	if err := parseQuotaPayload(out, []byte(`{}`)); err == nil {
		t.Fatal("payload without credits or window limits must error")
	}
}

func TestParseCommandCodeSubscription(t *testing.T) {
	// Shape per api.commandcode.ai/alpha/billing/subscriptions.
	body := `{"success":true,"data":{"planId":"individual-goat","status":"active","currentPeriodStart":"2026-10-19T12:40:05.000Z","currentPeriodEnd":"2026-11-19T12:40:05.000Z"}}`
	sub, err := parseCommandCodeSubscription([]byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sub.PlanID != "individual-goat" {
		t.Fatalf("planId = %q", sub.PlanID)
	}
	if sub.CurrentPeriodEnd != "2026-11-19T12:40:05Z" {
		t.Fatalf("period end = %q", sub.CurrentPeriodEnd)
	}
}

func TestParseCommandCodeSubscriptionNoSubscription(t *testing.T) {
	if _, err := parseCommandCodeSubscription([]byte(`{"success":true,"data":null}`)); err == nil {
		t.Fatal("data:null must report no subscription")
	}
}

func TestApplyCommandCodeSubscriptionAddsMonthlyWindow(t *testing.T) {
	// Live credits payload: monthlyCredits is remaining, no grant field.
	body := `{"credits":{"monthlyCredits":69.7527567329,"purchasedCredits":0,"freeCredits":0},
		"windowLimits":{"fiveHour":{"used":0.2472432671,"cap":14},"weekly":{"used":0.2472432671,"cap":35}}}`
	out := &quotaData{Kind: "commandcode-plan"}
	if err := parseQuotaPayload(out, []byte(body)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	applyCommandCodeSubscription(out, commandCodeSubscription{
		PlanID:           "individual-goat",
		CurrentPeriodEnd: "2026-11-19T12:40:05Z",
	})
	m, ok := out.Windows["monthly"]
	if !ok {
		t.Fatalf("monthly window missing: %+v", out.Windows)
	}
	if m.Cap != 70 {
		t.Fatalf("monthly cap = %v, want 70", m.Cap)
	}
	if m.Used < 0.2472 || m.Used > 0.2473 {
		t.Fatalf("monthly used = %v, want ~0.2472", m.Used)
	}
	if m.Percent < 0.35 || m.Percent > 0.36 {
		t.Fatalf("monthly percent = %v, want ~0.353", m.Percent)
	}
	if m.ResetsAt != "2026-11-19T12:40:05Z" {
		t.Fatalf("monthly resetsAt = %q", m.ResetsAt)
	}
	if out.Credits == nil || out.Credits.Plan != "GOAT" || out.Credits.Granted != 70 {
		t.Fatalf("credits = %+v", out.Credits)
	}
	// The rolling windows are untouched.
	if out.Windows["rolling"].Cap != 14 || out.Windows["weekly"].Cap != 35 {
		t.Fatalf("rolling windows changed: %+v", out.Windows)
	}
}

func TestApplyCommandCodeSubscriptionUnknownPlan(t *testing.T) {
	out := &quotaData{
		Kind:    "commandcode-plan",
		Credits: &creditInfo{Monthly: 5},
	}
	applyCommandCodeSubscription(out, commandCodeSubscription{PlanID: "individual-brand-new"})
	if _, ok := out.Windows["monthly"]; ok {
		t.Fatalf("unknown plan must not draw a monthly bar: %+v", out.Windows)
	}
	if out.Credits.Plan != "" {
		t.Fatalf("unknown plan name = %q", out.Credits.Plan)
	}
}

func TestApplyCommandCodeSubscriptionPrefersAPIGrant(t *testing.T) {
	// monthlyCreditsGranted is authoritative when the API sends it.
	out := &quotaData{
		Kind:    "commandcode-plan",
		Credits: &creditInfo{Monthly: 5, Granted: 10},
	}
	applyCommandCodeSubscription(out, commandCodeSubscription{PlanID: "individual-brand-new"})
	m, ok := out.Windows["monthly"]
	if !ok || m.Cap != 10 || m.Used != 5 || m.Percent != 50 {
		t.Fatalf("monthly window = %+v (ok=%v), want cap 10 used 5 at 50%%", m, ok)
	}
}

func TestApplyCommandCodeSubscriptionRejectsCatalogMismatch(t *testing.T) {
	// A renamed/re-priced plan must not be forced into the stale catalog.
	out := &quotaData{
		Kind:    "commandcode-plan",
		Credits: &creditInfo{Monthly: 80},
	}
	applyCommandCodeSubscription(out, commandCodeSubscription{PlanID: "individual-goat"})
	if _, ok := out.Windows["monthly"]; ok {
		t.Fatalf("mismatched catalog must not draw a monthly bar: %+v", out.Windows)
	}
}

func TestCommandCodePlanURL(t *testing.T) {
	cases := map[string]string{
		"https://api.commandcode.ai/alpha/billing/credits":               "https://api.commandcode.ai/alpha/billing/subscriptions",
		"https://api.commandcode.ai/alpha/billing/credits/":              "https://api.commandcode.ai/alpha/billing/subscriptions",
		"https://api.commandcode.ai/alpha/billing/credits?orgId=org_123": "https://api.commandcode.ai/alpha/billing/subscriptions?orgId=org_123",
		"https://example.com/custom":                                     "",
	}
	for in, want := range cases {
		if got := commandCodePlanURL(in); got != want {
			t.Fatalf("commandCodePlanURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFetchOneQuotaCommandCodeAddsMonthlyWindow exercises the two-request
// sequence through a stubbed host bridge: credits first, then subscriptions.
func TestFetchOneQuotaCommandCodeAddsMonthlyWindow(t *testing.T) {
	originalCallHost := callHost
	defer func() { callHost = originalCallHost }()

	var urls []string
	callHost = func(method string, payload any) (json.RawMessage, error) {
		req, ok := payload.(pluginapi.HTTPRequest)
		if !ok {
			return nil, fmt.Errorf("payload type %T", payload)
		}
		urls = append(urls, req.URL)
		var body string
		switch req.URL {
		case "https://api.commandcode.ai/alpha/billing/credits":
			body = `{"credits":{"monthlyCredits":69.75,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"fiveHour":{"used":0.25,"cap":14},"weekly":{"used":0.25,"cap":35}}}`
		case "https://api.commandcode.ai/alpha/billing/subscriptions":
			body = `{"success":true,"data":{"planId":"individual-goat","currentPeriodEnd":"2026-11-19T12:40:05.000Z"}}`
		default:
			return nil, fmt.Errorf("unexpected URL %q", req.URL)
		}
		resp, errMarshal := json.Marshal(pluginapi.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(body)})
		if errMarshal != nil {
			return nil, errMarshal
		}
		return resp, nil
	}

	rt := buildRuntimeForTest()
	e := &scannedEntry{
		Kind:     "commandcode-plan",
		QuotaURL: "https://api.commandcode.ai/alpha/billing/credits",
		APIKey:   "k",
	}
	out := fetchOneQuota(rt, e)
	if out.Err != "" {
		t.Fatalf("fetch error: %s", out.Err)
	}
	if len(urls) != 2 || urls[1] != "https://api.commandcode.ai/alpha/billing/subscriptions" {
		t.Fatalf("requests = %v, want credits then subscriptions", urls)
	}
	m, ok := out.Windows["monthly"]
	if !ok || m.Cap != 70 || m.Used != 0.25 {
		t.Fatalf("monthly window = %+v (ok=%v)", m, ok)
	}
	if out.Credits == nil || out.Credits.Plan != "GOAT" {
		t.Fatalf("credits = %+v", out.Credits)
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
