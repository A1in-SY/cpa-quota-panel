package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type percentWindow struct {
	Status   string  `json:"status"`
	Percent  float64 `json:"percent"`
	ResetsAt string  `json:"resetsAt"`
}

type balanceInfo struct {
	Currency string `json:"currency"`
	Total    string `json:"total"`
	Granted  string `json:"granted"`
	ToppedUp string `json:"toppedUp"`
}

type grantRow struct {
	Remaining string `json:"remaining"`
	ExpiresAt string `json:"expiresAt"`
}

// modelPlan is one MiniMax coding-plan model's remaining quota row (see the
// /v1/api/openplatform/coding_plan/remains endpoint).
type modelPlan struct {
	Name            string
	IntervalPercent float64 // current interval remaining %
	WeeklyPercent   float64 // current week remaining %
	IntervalRemain  int64   // seconds left in the current interval
	WeeklyRemain    int64   // seconds left in the current week
	IntervalUsed    int64   // requests used in the current interval
	IntervalTotal   int64
	WeeklyUsed      int64
	WeeklyTotal     int64
	IntervalStatus  int // 1 = normal
	WeeklyStatus    int
}

// quantum sake; keeps common prefix readable
func fmtDuration(seconds int64) string {
	if seconds <= 0 {
		return "0 分钟"
	}
	totalMinutes := seconds / 60
	if totalMinutes < 60 {
		return fmt.Sprintf("%d 分钟", totalMinutes)
	}
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if minutes == 0 {
		return fmt.Sprintf("%d 小时", hours)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", hours, minutes)
}

// quotaData is the parsed quota result for one scanned entry.
type quotaData struct {
	Kind      string
	Windows   map[string]percentWindow
	Balance   *balanceInfo
	Grants    []grantRow
	Models    []modelPlan
	FetchedAt int64
	Status    int
	Err       string
}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.537.36 Safari/537.36"

func fetchOneQuota(rt *runtime, e *scannedEntry) *quotaData {
	out := &quotaData{Kind: e.Kind, FetchedAt: rt.now()}
	useKey := e.APIKey
	if e.AdminKey != "" {
		useKey = e.AdminKey
	}
	req := pluginapi.HTTPRequest{
		Method: "GET",
		URL:    e.QuotaURL,
		Headers: http.Header{
			"Authorization": []string{"Bearer " + useKey},
			"Accept":        []string{"application/json"},
			"User-Agent":    []string{userAgent},
		},
	}
	raw, errCall := callHost(pluginabi.MethodHostHTTPDo, req)
	if errCall != nil {
		out.Err = "host.http error: " + errCall.Error()
		return out
	}
	if len(raw) == 0 {
		out.Err = "empty response"
		return out
	}
	var resp pluginapi.HTTPResponse
	if errUnmarshal := json.Unmarshal(raw, &resp); errUnmarshal != nil {
		out.Err = "invalid host response: " + errUnmarshal.Error()
		return out
	}
	out.Status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		out.Err = classifyQuotaError(resp.StatusCode, resp.Body)
		return out
	}
	if errParse := parseQuotaPayload(out, resp.Body); errParse != nil {
		out.Err = errParse.Error()
	}
	return out
}

func parseQuotaPayload(out *quotaData, body []byte) error {
	switch out.Kind {
	case "percent-windows":
		var payload struct {
			Usage struct {
				Rolling percentWindow `json:"rolling"`
				Weekly  percentWindow `json:"weekly"`
				Monthly percentWindow `json:"monthly"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("invalid usage payload: %w", err)
		}
		out.Windows = map[string]percentWindow{
			"rolling": payload.Usage.Rolling,
			"weekly":  payload.Usage.Weekly,
			"monthly": payload.Usage.Monthly,
		}
	case "balance":
		var payload struct {
			BalanceInfos []struct {
				Currency        string `json:"currency"`
				TotalBalance    any    `json:"total_balance"`
				GrantedBalance  any    `json:"granted_balance"`
				ToppedUpBalance any    `json:"topped_up_balance"`
			} `json:"balance_infos"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("invalid balance payload: %w", err)
		}
		if len(payload.BalanceInfos) == 0 {
			out.Balance = &balanceInfo{Total: "0"}
			return nil
		}
		bi := payload.BalanceInfos[0]
		out.Balance = &balanceInfo{
			Currency: bi.Currency,
			Total:    fmtNum(bi.TotalBalance),
			Granted:  fmtNum(bi.GrantedBalance),
			ToppedUp: fmtNum(bi.ToppedUpBalance),
		}
	case "coding-plan":
		var payload struct {
			ModelRemains []struct {
				ModelName                string  `json:"model_name"`
				IntervalRemainingPercent float64 `json:"current_interval_remaining_percent"`
				WeeklyRemainingPercent   float64 `json:"current_weekly_remaining_percent"`
				IntervalRemainsTime      float64 `json:"remains_time"` // millis
				WeeklyRemainsTime        float64 `json:"weekly_remains_time"`
				IntervalTotalCount       int64   `json:"current_interval_total_count"`
				IntervalUsageCount       int64   `json:"current_interval_usage_count"`
				WeeklyTotalCount         int64   `json:"current_weekly_total_count"`
				WeeklyUsageCount         int64   `json:"current_weekly_usage_count"`
				IntervalStatus           int     `json:"current_interval_status"`
				WeeklyStatus             int     `json:"current_weekly_status"`
			} `json:"model_remains"`
			BaseResp struct {
				StatusCode int `json:"status_code"`
			} `json:"base_resp"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("invalid coding-plan payload: %w", err)
		}
		if len(payload.ModelRemains) == 0 {
			return fmt.Errorf("coding-plan payload has no model_remains")
		}
		for _, m := range payload.ModelRemains {
			out.Models = append(out.Models, modelPlan{
				Name:            m.ModelName,
				IntervalPercent: m.IntervalRemainingPercent,
				WeeklyPercent:   m.WeeklyRemainingPercent,
				IntervalRemain:  int64(m.IntervalRemainsTime / 1000),
				WeeklyRemain:    int64(m.WeeklyRemainsTime / 1000),
				IntervalUsed:    m.IntervalUsageCount,
				IntervalTotal:   m.IntervalTotalCount,
				WeeklyUsed:      m.WeeklyUsageCount,
				WeeklyTotal:     m.WeeklyTotalCount,
				IntervalStatus:  m.IntervalStatus,
				WeeklyStatus:    m.WeeklyStatus,
			})
		}
	case "grants":
		var payload struct {
			Grants []struct {
				Remaining  any    `json:"remaining"`
				ExpiresAt  string `json:"expires_at"`
				ExpiresAt2 string `json:"expiredAt"`
			} `json:"grants"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("invalid grants payload: %w", err)
		}
		out.Grants = make([]grantRow, 0, len(payload.Grants))
		for _, g := range payload.Grants {
			exp := g.ExpiresAt
			if exp == "" {
				exp = g.ExpiresAt2
			}
			out.Grants = append(out.Grants, grantRow{Remaining: fmtNum(g.Remaining), ExpiresAt: exp})
		}
	case "zhipu-plan":
		if err := parseZhipuPlan(out, body); err != nil {
			return err
		}
	case "cline-plan":
		if err := parseClinePlan(out, body); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported kind %q", out.Kind)
	}
	return nil
}

// zhipuLimitRow is one quota window in the GLM Coding Plan usage limits array
// (open.bigmodel.cn/api/monitor/usage/quota/limit). TOKENS_LIMIT is the
// historical type name of the percentage windows; CREDIT_LIMIT is the current
// variant. TIME_LIMIT counts monthly MCP calls and carries a percentage too.
type zhipuLimitRow struct {
	Type          string  `json:"type"`
	Unit          int     `json:"unit"` // window length code: 3 = 5-hour, 6 = weekly
	Number        int     `json:"number"`
	Percentage    float64 `json:"percentage"`
	Usage         int64   `json:"usage"`
	CurrentValue  int64   `json:"currentValue"`
	Remaining     int64   `json:"remaining"`
	NextResetTime int64   `json:"nextResetTime"` // epoch milliseconds
}

// parseZhipuPlan decodes the GLM Coding Plan usage envelope. Note the endpoint
// reports auth failures with HTTP 200 + a non-zero body code, so the code must
// be checked here rather than via the HTTP status alone.
func parseZhipuPlan(out *quotaData, body []byte) error {
	var payload struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Success bool   `json:"success"`
		Data    struct {
			Level  string          `json:"level"`
			Limits []zhipuLimitRow `json:"limits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid zhipu-plan payload: %w", err)
	}
	if payload.Code != 0 && payload.Code != 200 {
		msg := strings.TrimSpace(payload.Msg)
		if msg == "" {
			switch payload.Code {
			case 401:
				msg = "invalid key (认证失败)"
			default:
				msg = "unknown error"
			}
		}
		return fmt.Errorf("zhipu-plan error %d: %s", payload.Code, msg)
	}

	out.Windows = map[string]percentWindow{}
	tokens := make([]zhipuLimitRow, 0, len(payload.Data.Limits))
	for _, l := range payload.Data.Limits {
		switch l.Type {
		case "TOKENS_LIMIT", "CREDIT_LIMIT":
			tokens = append(tokens, l)
		case "TIME_LIMIT":
			if l.Percentage > 0 {
				out.Windows["monthly"] = percentWindow{
					Percent:  l.Percentage,
					ResetsAt: zhipuResetRFC3339(l.NextResetTime),
				}
			}
		}
	}
	if len(tokens) == 0 {
		return fmt.Errorf("zhipu-plan payload has no TOKENS_LIMIT/CREDIT_LIMIT entries")
	}
	// The window's unit field identifies it definitively: 3 = 5-hour rolling,
	// 6 = weekly (confirmed against live responses). Anything unmapped falls
	// back to reset-time order: the soonest reset is the 5-hour window, the
	// next one is the weekly window.
	leftovers := make([]zhipuLimitRow, 0, len(tokens))
	for _, l := range tokens {
		switch l.Unit {
		case 3:
			out.Windows["rolling"] = percentWindow{
				Percent:  l.Percentage,
				ResetsAt: zhipuResetRFC3339(l.NextResetTime),
			}
		case 6:
			out.Windows["weekly"] = percentWindow{
				Percent:  l.Percentage,
				ResetsAt: zhipuResetRFC3339(l.NextResetTime),
			}
		default:
			leftovers = append(leftovers, l)
		}
	}
	sort.Slice(leftovers, func(i, j int) bool { return leftovers[i].NextResetTime < leftovers[j].NextResetTime })
	for _, l := range leftovers {
		if _, ok := out.Windows["rolling"]; !ok {
			out.Windows["rolling"] = percentWindow{
				Percent:  l.Percentage,
				ResetsAt: zhipuResetRFC3339(l.NextResetTime),
			}
			continue
		}
		if _, ok := out.Windows["weekly"]; !ok {
			out.Windows["weekly"] = percentWindow{
				Percent:  l.Percentage,
				ResetsAt: zhipuResetRFC3339(l.NextResetTime),
			}
		}
	}
	return nil
}

// zhipuResetRFC3339 converts the endpoint's epoch-millisecond reset stamps to
// RFC3339 so the dashboard countdown JS parses them natively.
func zhipuResetRFC3339(millis int64) string {
	if millis <= 0 {
		return ""
	}
	return time.UnixMilli(millis).UTC().Format(time.RFC3339)
}

// clineLimitRow is one window of the ClinePass usage payload
// (api.cline.bot/api/v1/users/me/plan/usage-limits). percentUsed is the used
// share of the window — the same meaning the other vendors' bars take — and
// resetsAt is RFC3339, absent for the rolling 5-hour window.
type clineLimitRow struct {
	Type        string  `json:"type"` // five_hour | weekly | monthly
	PercentUsed float64 `json:"percentUsed"`
	ResetsAt    string  `json:"resetsAt"`
}

// clineWindowKeys maps Cline's window names onto the three window keys the
// dashboard renders, so the bars look exactly like the other vendors'.
var clineWindowKeys = map[string]string{
	"five_hour": "rolling",
	"weekly":    "weekly",
	"monthly":   "monthly",
}

// parseClinePlan decodes the {success,data,error} envelope the Cline API always
// answers with: auth/plan failures arrive as HTTP 200 with success=false, and a
// key without an active ClinePass returns data:null. The latter leaves the card
// empty (the dashboard's existing 暂无额度数据 state) rather than erroring.
func parseClinePlan(out *quotaData, body []byte) error {
	var payload struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Data    *struct {
			Limits []clineLimitRow `json:"limits"`
		} `json:"data"`
	}
	if errUnmarshal := json.Unmarshal(body, &payload); errUnmarshal != nil {
		return fmt.Errorf("invalid cline-plan payload: %w", errUnmarshal)
	}
	if !payload.Success {
		msg := strings.TrimSpace(payload.Error)
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("cline-plan error: %s", msg)
	}
	if payload.Data == nil {
		return nil
	}
	windows := map[string]percentWindow{}
	for _, l := range payload.Data.Limits {
		key, ok := clineWindowKeys[l.Type]
		if !ok {
			continue
		}
		windows[key] = percentWindow{
			Percent:  l.PercentUsed,
			ResetsAt: clineResetRFC3339(l.ResetsAt),
		}
	}
	if len(windows) == 0 {
		return nil
	}
	out.Windows = windows
	return nil
}

// clineResetRFC3339 trims Cline's nanosecond fractional seconds down to the
// second-precision RFC3339 every other vendor's reset stamp already uses, so the
// dashboard's countdown JS parses it instead of falling back to the raw string.
func clineResetRFC3339(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return ""
	}
	parsed, errParse := time.Parse(time.RFC3339Nano, ts)
	if errParse != nil {
		return ts
	}
	return parsed.UTC().Format(time.RFC3339)
}

func fmtNum(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return fmt.Sprintf("%v", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func classifyQuotaError(status int, body []byte) string {
	msg := strings.TrimSpace(string(body))
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "cloudflare") || strings.Contains(lower, "error-10") || strings.Contains(lower, "cf-") {
		if len(msg) > 160 {
			msg = msg[:160]
		}
		return "blocked upstream by Cloudflare (may need a different IP/UA): " + msg
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	switch status {
	case http.StatusUnauthorized:
		return "invalid key (401): " + msg
	case http.StatusForbidden:
		if strings.Contains(lower, "admin key") || strings.Contains(lower, "token_type_mismatch") || strings.Contains(lower, "permission_error") {
			return "该厂商额度接口需要管理员 key（当前 key 无权限，403）：若需显示余额，请在 quota-sources 对应项配置 admin-key"
		}
		return "forbidden (403): " + msg
	case http.StatusTooManyRequests:
		return "rate limited (429): " + msg
	default:
		return fmt.Sprintf("HTTP %d: %s", status, msg)
	}
}

func storeQuota(rt *runtime, key string, data *quotaData) {
	rtMu.Lock()
	defer rtMu.Unlock()
	if active != rt {
		return
	}
	rt.quota[key] = data
}

// refreshQuota refreshes stale entries (all when entrySet is empty). Callers
// pass the current page so the work stays bounded by the page size.
func refreshQuota(rt *runtime, force bool, entrySet []*scannedEntry) {
	if rt == nil {
		return
	}
	if entrySet == nil {
		rtMu.RLock()
		entrySet = append([]*scannedEntry(nil), rt.entries...)
		rtMu.RUnlock()
	}
	now := rt.now()
	stale := make([]*scannedEntry, 0, len(entrySet))
	for _, e := range entrySet {
		rtMu.RLock()
		d := rt.quota[e.VendorID+"\x00"+e.APIKey]
		fresh := d != nil && d.FetchedAt != 0 && now-d.FetchedAt <= rt.cacheTTL
		rtMu.RUnlock()
		if force || !fresh {
			stale = append(stale, e)
		}
	}
	for _, e := range stale {
		d := fetchOneQuota(rt, e)
		storeQuota(rt, e.VendorID+"\x00"+e.APIKey, d)
	}
}

// quotaFor returns the cached quota for an entry key. fresh=false means missing/stale.
func (rt *runtime) quotaFor(key string) (*quotaData, bool) {
	if rt == nil {
		return nil, false
	}
	rtMu.RLock()
	d := rt.quota[key]
	rtMu.RUnlock()
	if d == nil || d.FetchedAt == 0 {
		return nil, false
	}
	return d, rt.now()-d.FetchedAt <= rt.cacheTTL
}
