package main

import (
	"encoding/json"
	"net/http"
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
	force := len(req.Query.Get("refresh")) > 0
	refreshDashboard(rt, force)
	page := buildPageData(rt)
	html := renderDashboard(page)
	return okEnvelope(pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(html),
	})
}

// refreshDashboard rescans the host config when stale and refreshes stale quota results.
func refreshDashboard(rt *runtime, force bool) {
	if rt == nil {
		return
	}
	rtMu.Lock()
	if force || rt.lastScan == 0 || rt.now()-rt.lastScan >= 30 {
		rt.scanLocked()
	}
	rtMu.Unlock()
	refreshQuota(rt, force, nil)
}
