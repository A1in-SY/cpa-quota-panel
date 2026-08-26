package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type managementRegistrationResponse struct {
	Resources []pluginapi.ResourceRoute `json:"resources,omitempty"`
}

func managementRegistration() managementRegistrationResponse {
	return managementRegistrationResponse{
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        "/status",
				Menu:        "额度面板",
				Description: "按厂商查看各 AI 提供商条目（base-url 白名单命中）的剩余额度。",
			},
		},
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var req pluginapi.ManagementRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return errorEnvelope("invalid_management_request", "invalid management request: "+errUnmarshal.Error()), nil
	}
	path := strings.TrimRight(req.Path, "/")
	rt := loadedRuntime()
	if rt == nil {
		return okEnvelope(pluginapi.ManagementResponse{
			StatusCode: http.StatusServiceUnavailable,
			Headers:    http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:       []byte("cpa-quota-panel 尚未配置请重启或检查 plugins.configs.cpa-quota-panel"),
		})
	}
	if !strings.HasSuffix(path, "/status") {
		return okEnvelope(pluginapi.ManagementResponse{
			StatusCode: http.StatusNotFound,
			Body:       []byte("not found"),
		})
	}
	q := pageQuery{
		Vendor:   strings.TrimSpace(req.Query.Get("vendor")),
		Page:     intParam(req.Query.Get("page"), 1),
		PageSize: intParam(req.Query.Get("page-size"), defaultPageSize),
	}
	force := len(req.Query.Get("refresh")) > 0
	entryIdx := intParam0(req.Query.Get("entry-idx"), -1)
	rescanHostConfig(rt, force)
	view := selectPageView(rt, q)

	// Quota refresh is scoped to what the client actually needs, so the panel
	// never blocks on "all entries at once":
	//   - ?entry-idx=N  -> consider ONLY that single card (progressive lazy
	//                      fill); the cache TTL decides whether upstream is
	//                      actually queried (force only with ?refresh=1);
	//   - ?refresh=1    -> full-page forced refresh (explicit 刷新 click);
	//   - otherwise     -> return instantly from cache, missing cards become
	//                      skeletons the client fills in via entry-idx requests.
	if entryIdx >= 0 && entryIdx < len(view.Entries) {
		refreshQuota(rt, force, view.Entries[entryIdx:entryIdx+1])
	} else if force {
		refreshQuota(rt, true, view.Entries)
	}
	page := buildPageData(rt, view)
	if req.Query.Get("partial") != "" {
		frag := buildPageFragments(page)
		if entryIdx >= 0 && entryIdx < len(view.Entries) {
			frag.EntryIdx = entryIdx
			frag.EntryHTML = renderEntryHTML(page, entryIdx)
		}
		raw, errMarshal := json.Marshal(frag)
		if errMarshal != nil {
			return errorEnvelope("encode_partial", "encode partial page: "+errMarshal.Error()), nil
		}
		return okEnvelope(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Body:       raw,
		})
	}
	html := renderDashboard(page)
	return okEnvelope(pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(html),
	})
}

func intParam(v string, def int) int {
	n, errParse := strconv.Atoi(strings.TrimSpace(v))
	if errParse != nil || n < 1 {
		return def
	}
	return n
}

// intParam0 is like intParam but keeps 0 (used for 0-based entry indexes).
func intParam0(v string, def int) int {
	if strings.TrimSpace(v) == "" {
		return def
	}
	n, errParse := strconv.Atoi(strings.TrimSpace(v))
	if errParse != nil {
		return def
	}
	return n
}

// rescanHostConfig rescans the host config when stale (or when forced).
func rescanHostConfig(rt *runtime, force bool) {
	if rt == nil {
		return
	}
	rtMu.Lock()
	if force || rt.lastScan == 0 || rt.now()-rt.lastScan >= 30 {
		rt.scanLocked()
	}
	rtMu.Unlock()
}
