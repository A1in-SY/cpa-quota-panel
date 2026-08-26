package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	default:
		return fmt.Errorf("unsupported kind %q", out.Kind)
	}
	return nil
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
