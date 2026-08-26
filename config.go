package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// QuotaSource is one whitelisted vendor template. All AI-provider entries whose
// base-url matches any of MatchBaseURLs are collected under this vendor.
type QuotaSource struct {
	ID            string   `yaml:"id" json:"id"`
	Name          string   `yaml:"name" json:"name"`
	MatchBaseURLs []string `yaml:"match-base-urls" json:"match-base-urls"`
	QuotaURL      string   `yaml:"quota-url" json:"quota-url"`
	Auth          string   `yaml:"auth,omitempty" json:"auth"` // bearer (default)
	Kind          string   `yaml:"kind" json:"kind"`           // percent-windows | balance | grants
	// AdminKey optionally overrides the key used to query this vendor's quota
	// (e.g. MiniMax's account/balance requires an admin key).
	AdminKey string `yaml:"admin-key,omitempty" json:"admin-key"`
}

type pluginConfig struct {
	ConfigPath string        `yaml:"config-path" json:"config-path"`
	CacheTTL   int           `yaml:"cache-ttl-seconds" json:"cache-ttl-seconds"`
	Sources    []QuotaSource `yaml:"quota-sources" json:"quota-sources"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

// runtime is the mutable plugin state. It is replaced as a whole on reconfigure.
type runtime struct {
	configPath  string
	cacheTTL    int64
	sources     []QuotaSource
	entries     []*scannedEntry
	byKey       map[string]*scannedEntry
	quota       map[string]*quotaData
	lastScan    int64
	refreshing  bool
	now         func() int64 // unix seconds, overridable in tests
	configError string
}

var (
	rtMu   sync.RWMutex
	active *runtime
)

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return fmt.Errorf("decode configure request: %w", errUnmarshal)
		}
	}
	cfg := pluginConfig{}
	if len(req.ConfigYAML) > 0 {
		if errDecode := yaml.Unmarshal(req.ConfigYAML, &cfg); errDecode != nil {
			return fmt.Errorf("decode plugin config: %w", errDecode)
		}
	}
	if errValid := normalizeAndValidate(&cfg); errValid != nil {
		return errValid
	}
	rtMu.Lock()
	active = buildRuntime(&cfg)
	rtMu.Unlock()
	return nil
}

func normalizeAndValidate(cfg *pluginConfig) error {
	cfg.ConfigPath = strings.TrimSpace(cfg.ConfigPath)
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/CLIProxyAPI/config.yaml"
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 300
	}
	if len(cfg.Sources) == 0 {
		cfg.Sources = defaultSources()
	}
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("quota-sources must not be empty")
	}
	seen := map[string]struct{}{}
	for i := range cfg.Sources {
		src := &cfg.Sources[i]
		src.ID = strings.TrimSpace(src.ID)
		src.Name = strings.TrimSpace(src.Name)
		src.QuotaURL = strings.TrimSpace(src.QuotaURL)
		src.Auth = strings.ToLower(strings.TrimSpace(src.Auth))
		if src.Auth == "" {
			src.Auth = "bearer"
		}
		src.AdminKey = strings.TrimSpace(src.AdminKey)
		src.Kind = strings.TrimSpace(src.Kind)
		if src.ID == "" {
			return fmt.Errorf("quota-sources[%d].id is required", i)
		}
		if _, dup := seen[src.ID]; dup {
			return fmt.Errorf("quota-sources[%d].id %q is duplicated", i, src.ID)
		}
		seen[src.ID] = struct{}{}
		if src.Name == "" {
			src.Name = src.ID
		}
		if len(src.MatchBaseURLs) == 0 {
			return fmt.Errorf("quota-sources[%d].id %q: match-base-urls is required", i, src.ID)
		}
		norm := src.MatchBaseURLs[:0]
		for _, u := range src.MatchBaseURLs {
			u = normalizeBaseURL(u)
			if u != "" {
				norm = append(norm, u)
			}
		}
		src.MatchBaseURLs = norm
		if src.QuotaURL == "" {
			return fmt.Errorf("quota-sources[%d].id %q: quota-url is required", i, src.ID)
		}
		switch src.Kind {
		case "percent-windows", "balance", "grants", "coding-plan", "zhipu-plan":
		default:
			return fmt.Errorf("quota-sources[%d].id %q: kind must be percent-windows|balance|grants|coding-plan|zhipu-plan", i, src.ID)
		}
	}
	return nil
}

// defaultSources matches the three vendors the plugin ships with.
func defaultSources() []QuotaSource {
	return []QuotaSource{
		{
			ID:            "opencode",
			Name:          "OpenCode",
			MatchBaseURLs: []string{"https://opencode.ai/zen/go/v1"},
			QuotaURL:      "https://opencode.ai/zen/go/v1/usage",
			Auth:          "bearer",
			Kind:          "percent-windows",
		},
		{
			ID:            "deepseek",
			Name:          "Deepseek API",
			MatchBaseURLs: []string{"https://api.deepseek.com", "https://api.deepseek.com/anthropic"},
			QuotaURL:      "https://api.deepseek.com/user/balance",
			Auth:          "bearer",
			Kind:          "balance",
		},
		{
			ID:   "minimax",
			Name: "MiniMax",
			// The coding plan /remains endpoint works with a normal sk-cp- API key,
			// unlike /v1/account/balance which requires an admin key (403 otherwise).
			MatchBaseURLs: []string{"https://www.minimaxi.com", "https://api.minimaxi.com/v1"},
			QuotaURL:      "https://www.minimaxi.com/v1/api/openplatform/coding_plan/remains",
			Auth:          "bearer",
			Kind:          "coding-plan",
		},
		{
			ID:   "zhipu",
			Name: "智谱CodingPlan",
			// 智谱开放平台（bigmodel）国内版：anthropic 变体、coding 专用端点与 v1
			// 兼容端点都按 GLM Coding Plan 用量接口查询（/api/biz/usage，普通 API key 即可）。
			MatchBaseURLs: []string{
				"https://open.bigmodel.cn/api/anthropic",
				"https://open.bigmodel.cn/api/coding/paas/v4",
				"https://open.bigmodel.cn/api/v1",
			},
			QuotaURL: "https://open.bigmodel.cn/api/biz/usage",
			Auth:     "bearer",
			Kind:     "zhipu-plan",
		},
	}
}

func buildRuntime(cfg *pluginConfig) *runtime {
	rt := &runtime{
		configPath: cfg.ConfigPath,
		cacheTTL:   int64(cfg.CacheTTL),
		sources:    append([]QuotaSource(nil), cfg.Sources...),
		byKey:      map[string]*scannedEntry{},
		quota:      map[string]*quotaData{},
		now:        func() int64 { return nowFn() },
	}
	rt.scanLocked()
	return rt
}

func resetRuntime() {
	rtMu.Lock()
	active = nil
	rtMu.Unlock()
}

func loadedRuntime() *runtime {
	rtMu.RLock()
	defer rtMu.RUnlock()
	return active
}

var nowFn = func() int64 { return time.Now().Unix() }

// normalizeBaseURL trims whitespace and a trailing slash.
func normalizeBaseURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	return u
}

// matchBaseURL reports whether an entry base-url matches any whitelist pattern.
// A pattern matches exactly, or as a path prefix of the entry URL at a "/" boundary,
// so listing "https://api.deepseek.com" also covers "https://api.deepseek.com/anthropic".
func matchBaseURL(entryBase string, patterns []string) bool {
	entryBase = normalizeBaseURL(entryBase)
	if entryBase == "" {
		return false
	}
	for _, p := range patterns {
		p = normalizeBaseURL(p)
		if p == "" {
			continue
		}
		if entryBase == p {
			return true
		}
		if strings.HasPrefix(entryBase, p+"/") {
			return true
		}
	}
	return false
}
