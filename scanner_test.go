package main

import (
	"os"
	"path/filepath"
	"testing"
)

const testConfigYAML = `
codex-api-key:
  - api-key: "oc-key-111"
    base-url: "https://opencode.ai/zen/go/v1"
  - api-key: "oc-key-222"
    base-url: "https://opencode.ai/zen/go/v1"
  - api-key: "ds-key-222"
    base-url: "https://api.deepseek.com"
  - api-key: "mm-key-333"
    base-url: "https://api.minimaxi.com/v1"
  - api-key: "oc-other"
    base-url: "https://silicon.example.com/v1"   # not whitelisted
claude-api-key:
  - api-key: "oc-key-111"                        # same opencode key under claude too
    base-url: "https://opencode.ai/zen/go/v1"
  - api-key: "cl-key-000"
    base-url: ""                                  # empty -> official (not whitelisted here)
openai-compatibility:
  - base-url: "https://api.deepseek.com/anthropic"
    api-key-entries:
      - api-key: "ds-anthropic-key"
  - base-url: "https://api.other.com/v1"
    api-key-entries:
      - api-key: "other-key"
`

func TestScanConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(testConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	sources := defaultSources()
	entries, err := scanConfigFile(path, sources)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	byVendor := map[string]map[string]*scannedEntry{}
	for _, e := range entries {
		if byVendor[e.VendorID] == nil {
			byVendor[e.VendorID] = map[string]*scannedEntry{}
		}
		byVendor[e.VendorID][e.APIKey] = e
	}

	// opencode: two entries, and the same key under codex+claude must merge with both types.
	oc := byVendor["opencode"]
	if len(oc) != 2 {
		t.Fatalf("opencode entries = %d, want 2 (merged same-key + oc-other excluded)", len(oc))
	}
	m := oc["oc-key-111"]
	if m == nil {
		t.Fatalf("oc-key-111 missing")
	}
	if len(m.ProviderTypes) != 2 {
		t.Fatalf("oc-key-111 provider types = %v, want [claude codex]", m.ProviderTypes)
	}
	if m.ProviderTypes[0] != "claude" || m.ProviderTypes[1] != "codex" {
		t.Fatalf("oc-key-111 provider types = %v, want [claude codex]", m.ProviderTypes)
	}

	// deepseek: plain base-url entry + an openai-compatibility entry whose base-url
	// is the /anthropic variant (matched by path-prefix).
	ds := byVendor["deepseek"]
	if len(ds) != 2 {
		t.Fatalf("deepseek entries = %d (%v), want 2", len(ds), keysOf(ds))
	}
	if ds["ds-anthropic-key"] == nil {
		t.Fatalf("deepseek /anthropic variant not matched: %v", keysOf(ds))
	}
	if ds["ds-anthropic-key"].ProviderTypes[0] != "openai-compatibility" {
		t.Fatalf("ds-anthropic provider type = %v", ds["ds-anthropic-key"].ProviderTypes)
	}

	// minimax.
	if mm := byVendor["minimax"]; len(mm) != 1 || mm["mm-key-333"] == nil {
		t.Fatalf("minimax entries = %v", keysOf(mm))
	}

	// Not whitelisted keys must be absent.
	for _, e := range entries {
		if e.APIKey == "oc-other" || e.APIKey == "cl-key-000" || e.APIKey == "other-key" {
			t.Fatalf("non-whitelisted key leaked into entries: %s", e.APIKey)
		}
	}

	// Key tail derived (last 6 chars prefixed with ellipsis).
	if m.KeyTail != "…ey-111" {
		t.Fatalf("unexpected key tail %q", m.KeyTail)
	}
}

func TestScanConfigFileMissing(t *testing.T) {
	if _, err := scanConfigFile("/nonexistent/nope.yaml", defaultSources()); err == nil {
		t.Fatal("missing file must error")
	}
}

func keysOf(m map[string]*scannedEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestScanAdminKeyPropagation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := "codex-api-key:\n  - api-key: \"ds-key-222\"\n    base-url: \"https://api.deepseek.com\"\n"
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	sources := defaultSources()
	for i := range sources {
		if sources[i].ID == "deepseek" {
			sources[i].AdminKey = "adm-ds-admin"
		}
	}
	entries, err := scanConfigFile(path, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].AdminKey != "adm-ds-admin" {
		t.Fatalf("admin key not propagated: %+v", entries)
	}
}
