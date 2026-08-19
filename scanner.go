package main

import (
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// hostConfig is the minimal view of the host config.yaml that the plugin scans.
type hostConfig struct {
	Codex        []keyEntry     `yaml:"codex-api-key"`
	XAI          []keyEntry     `yaml:"xai-api-key"`
	Claude       []keyEntry     `yaml:"claude-api-key"`
	Gemini       []keyEntry     `yaml:"gemini-api-key"`
	Interactions []keyEntry     `yaml:"interactions-api-key"`
	Vertex       []keyEntry     `yaml:"vertex-api-key"`
	OpenAICompat []openAICompat `yaml:"openai-compatibility"`
}

type keyEntry struct {
	APIKey  string `yaml:"api-key"`
	BaseURL string `yaml:"base-url"`
}

type openAICompat struct {
	BaseURL       string     `yaml:"base-url"`
	APIKeyEntries []keyEntry `yaml:"api-key-entries"`
}

type rawItem struct {
	ProviderType string // config section name: codex, xai, claude, gemini, interactions, vertex, openai-compatibility
	BaseURL      string
	APIKey       string
}

// scannedEntry is one merged quota entry: unique per (vendor, exact API key).
type scannedEntry struct {
	VendorID      string
	VendorName    string
	Kind          string
	QuotaURL      string
	Auth          string
	APIKey        string
	AdminKey      string
	KeyTail       string
	ProviderTypes []string
}

func keyTail(key string, n int) string {
	if key == "" {
		return ""
	}
	if len(key) <= n {
		return key
	}
	return "…" + key[len(key)-n:]
}

// defaultBaseURL returns the well-known upstream base-url for providers whose
// entry leaves base-url empty, so whitelist matching still sees a URL.
func defaultBaseURL(providerType string) string {
	switch providerType {
	case "claude":
		return "https://api.anthropic.com"
	case "gemini", "interactions":
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}

// scanConfigFile reads the host config, matches whitelisted entries and merges
// entries whose vendor + API key are identical.
func scanConfigFile(path string, sources []QuotaSource) ([]*scannedEntry, error) {
	data, errRead := os.ReadFile(path)
	if errRead != nil {
		return nil, errRead
	}
	var hc hostConfig
	if errParse := yaml.Unmarshal(data, &hc); errParse != nil {
		return nil, errParse
	}

	var items []rawItem
	collect := func(providerType string, providerBase string, entries []keyEntry) {
		for _, e := range entries {
			key := strings.TrimSpace(e.APIKey)
			if key == "" {
				continue
			}
			// Effective base-url: entry-level overrides, else provider-level
			// (openai-compatibility), else the well-known default.
			base := normalizeBaseURL(e.BaseURL)
			if base == "" {
				base = normalizeBaseURL(providerBase)
			}
			if base == "" {
				base = defaultBaseURL(providerType)
			}
			items = append(items, rawItem{ProviderType: providerType, BaseURL: base, APIKey: key})
		}
	}
	collect("codex", "", hc.Codex)
	collect("xai", "", hc.XAI)
	collect("claude", "", hc.Claude)
	collect("gemini", "", hc.Gemini)
	collect("interactions", "", hc.Interactions)
	collect("vertex", "", hc.Vertex)
	for _, p := range hc.OpenAICompat {
		base := normalizeBaseURL(p.BaseURL)
		collect("openai-compatibility", base, p.APIKeyEntries)
	}

	// Group by (vendor, key).
	type merged struct {
		types map[string]struct{}
		item  rawItem
		src   *QuotaSource
	}
	groups := map[string]*merged{}
	order := []string{}
	// Preserve vendor order from sources.
	for i := range sources {
		src := &sources[i]
		for _, it := range items {
			if !matchBaseURL(it.BaseURL, src.MatchBaseURLs) {
				continue
			}
			key := src.ID + "\x00" + it.APIKey
			g, ok := groups[key]
			if !ok {
				g = &merged{types: map[string]struct{}{}, item: it, src: src}
				groups[key] = g
				order = append(order, key)
			}
			g.types[it.ProviderType] = struct{}{}
		}
	}

	out := make([]*scannedEntry, 0, len(order))
	for _, key := range order {
		g := groups[key]
		types := make([]string, 0, len(g.types))
		for t := range g.types {
			types = append(types, t)
		}
		sort.Strings(types)
		out = append(out, &scannedEntry{
			VendorID:      g.src.ID,
			VendorName:    g.src.Name,
			Kind:          g.src.Kind,
			QuotaURL:      g.src.QuotaURL,
			Auth:          g.src.Auth,
			APIKey:        g.item.APIKey,
			AdminKey:      g.src.AdminKey,
			KeyTail:       keyTail(g.item.APIKey, 6),
			ProviderTypes: types,
		})
	}
	return out, nil
}

// scanLocked re-scans the host config. Caller must hold rtMu.Lock().
func (rt *runtime) scanLocked() {
	entries, errScan := scanConfigFile(rt.configPath, rt.sources)
	if errScan != nil {
		rt.configError = errScan.Error()
		rt.entries = nil
		rt.byKey = map[string]*scannedEntry{}
		return
	}
	rt.configError = ""
	rt.entries = entries
	rt.byKey = map[string]*scannedEntry{}
	for _, e := range entries {
		rt.byKey[e.VendorID+"\x00"+e.APIKey] = e
	}
	rt.lastScan = rt.now()
}
