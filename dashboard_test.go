package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func testEntry(vendor, key string) *scannedEntry {
	return &scannedEntry{
		VendorID:      vendor,
		VendorName:    vendor,
		Kind:          "balance",
		QuotaURL:      "https://example.com/balance",
		APIKey:        key,
		KeyTail:       keyTail(key, 6),
		ProviderTypes: []string{"codex"},
	}
}

func testRuntime(entries []*scannedEntry) *runtime {
	rt := &runtime{
		sources:  defaultSources(),
		entries:  entries,
		byKey:    map[string]*scannedEntry{},
		quota:    map[string]*quotaData{},
		cacheTTL: 300,
		now:      func() int64 { return 100 },
	}
	for _, e := range entries {
		rt.byKey[e.VendorID+"\x00"+e.APIKey] = e
	}
	return rt
}

func TestSelectPageViewPagination(t *testing.T) {
	var entries []*scannedEntry
	for i := 1; i <= 45; i++ {
		entries = append(entries, testEntry("opencode", fmt.Sprintf("oc-key-%03d", i)))
	}
	for i := 1; i <= 3; i++ {
		entries = append(entries, testEntry("deepseek", fmt.Sprintf("ds-key-%03d", i)))
	}
	rt := testRuntime(entries)

	cases := []struct {
		name string
		q    pageQuery
		page int
		n    int
		tot  int
	}{
		{"defaults to 20/page", pageQuery{}, 1, 20, 48},
		{"page 2", pageQuery{Page: 2, PageSize: 20}, 2, 20, 48},
		{"page 3 last partial", pageQuery{Page: 3, PageSize: 20}, 3, 8, 48},
		{"page beyond clamps", pageQuery{Page: 99, PageSize: 20}, 3, 8, 48},
		{"page size 100", pageQuery{PageSize: 100}, 1, 48, 48},
		{"vendor filter", pageQuery{Vendor: "deepseek"}, 1, 3, 3},
	}
	for _, c := range cases {
		v := selectPageView(rt, c.q)
		if v.Page != c.page || len(v.Entries) != c.n || v.Total != c.tot {
			t.Fatalf("%s: page=%d n=%d total=%d, want page=%d n=%d total=%d",
				c.name, v.Page, len(v.Entries), v.Total, c.page, c.n, c.tot)
		}
	}
}

func TestBuildPageDataKeepsFullVendorCounts(t *testing.T) {
	var entries []*scannedEntry
	for i := 1; i <= 45; i++ {
		entries = append(entries, testEntry("opencode", fmt.Sprintf("oc-key-%03d", i)))
	}
	for i := 1; i <= 3; i++ {
		entries = append(entries, testEntry("deepseek", fmt.Sprintf("ds-key-%03d", i)))
	}
	rt := testRuntime(entries)
	view := selectPageView(rt, pageQuery{Page: 1, PageSize: 20})
	page := buildPageData(rt, view)

	if len(page.Entries) != 20 || page.Total != 48 || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("page data = %d entries, total=%d page=%d size=%d", len(page.Entries), page.Total, page.Page, page.PageSize)
	}
	counts := map[string]int{}
	all := 0
	for _, v := range page.Vendors {
		counts[v.ID] = v.Count
		all += v.Count
	}
	if counts["opencode"] != 45 || counts["deepseek"] != 3 || all != 48 {
		t.Fatalf("vendor counts = %v, all=%d, want opencode=45 deepseek=3 all=48", counts, all)
	}
}

func TestEmptyTextWithPagination(t *testing.T) {
	if got := emptyText(pageData{Total: 0, Vendor: "deepseek"}); got != "该厂商暂无条目" {
		t.Fatalf("vendor empty text = %q", got)
	}
	if got := emptyText(pageData{Total: 0}); got == "" {
		t.Fatal("global empty text must not be empty")
	}
	if got := emptyText(pageData{Total: 3}); got != "" {
		t.Fatalf("non-empty page text = %q, want empty", got)
	}
	if got := emptyText(pageData{Total: 0, ConfigError: "boom"}); got != "" {
		t.Fatalf("config error text = %q, want empty", got)
	}
}

// TestBuildPageDataMissingFlags: entries without a cached quota result must be
// marked Missing (rendered as a skeleton card), while cached ones are not.
func TestBuildPageDataMissingFlags(t *testing.T) {
	rt := testRuntime([]*scannedEntry{
		testEntry("opencode", "oc-key-A"),
		testEntry("deepseek", "ds-key-B"),
	})
	rt.quota["opencode\x00oc-key-A"] = &quotaData{
		Kind: "percent-windows", FetchedAt: 99,
		Windows: map[string]percentWindow{"rolling": {Percent: 40}},
	}
	view := selectPageView(rt, pageQuery{PageSize: 20})
	page := buildPageData(rt, view)
	if len(page.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(page.Entries))
	}
	byVendor := map[string]pageEntry{}
	for _, e := range page.Entries {
		byVendor[e.VendorID] = e
	}
	if byVendor["opencode"].Missing {
		t.Fatalf("entry with cache must not be Missing")
	}
	if !byVendor["deepseek"].Missing {
		t.Fatalf("entry without cache must be Missing")
	}
	if byVendor["opencode"].Idx != 1 || byVendor["deepseek"].Idx != 0 {
		t.Fatalf("entry indexes = opencode:%d deepseek:%d, want 1,0",
			byVendor["opencode"].Idx, byVendor["deepseek"].Idx)
	}
	// RefreshedAt reflects the latest real fetch (99), not the render time.
	if page.RefreshedAt != 99 {
		t.Fatalf("refreshedAt = %d, want 99 (latest cached fetch)", page.RefreshedAt)
	}

	// With no cache at all, RefreshedAt must be 0 so the UI hides "上次…".
	rt2 := testRuntime([]*scannedEntry{testEntry("opencode", "oc-key-A")})
	pageEmpty := buildPageData(rt2, selectPageView(rt2, pageQuery{PageSize: 20}))
	if pageEmpty.RefreshedAt != 0 {
		t.Fatalf("refreshedAt with empty cache = %d, want 0", pageEmpty.RefreshedAt)
	}
}

// TestManagementQueryAliases: the server must honor the JS URL spellings
// (v= vendor tab, p= page) the same as vendor=/page=, so a reload keeps
// skeleton lazy-fetch indexes aligned with the rendered grid.
func TestManagementQueryAliases(t *testing.T) {
	rt := testRuntime([]*scannedEntry{
		testEntry("opencode", "oc-key-A"),
		testEntry("deepseek", "ds-key-B"),
		testEntry("deepseek", "ds-key-C"),
	})
	rt.lastScan = 100 // testRuntime pins now()=100; skip the periodic rescan
	rtMu.Lock()
	wasActive := active
	active = rt
	rtMu.Unlock()
	defer func() {
		rtMu.Lock()
		active = wasActive
		rtMu.Unlock()
	}()

	reqFor := func(alias bool) []byte {
		q := url.Values{"partial": {"1"}, "page": {"1"}, "page-size": {"20"}, "vendor": {"deepseek"}}
		if alias {
			q = url.Values{"partial": {"1"}, "p": {"1"}, "v": {"deepseek"}}
		}
		raw, _ := json.Marshal(pluginapi.ManagementRequest{Path: "/status", Query: q})
		return raw
	}
	stdRaw, errStd := handleManagement(reqFor(false))
	if errStd != nil {
		t.Fatalf("canonical query: %v", errStd)
	}
	aliasRaw, errAlias := handleManagement(reqFor(true))
	if errAlias != nil {
		t.Fatalf("alias query: %v", errAlias)
	}
	fragOf := func(envRaw []byte) pageFragments {
		var env struct {
			OK     bool            `json:"ok"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(envRaw, &env); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		var resp struct {
			Body string `json:"Body"`
		}
		if err := json.Unmarshal(env.Result, &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		body, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			t.Fatalf("decode body: %v", err)
		}
		var frag pageFragments
		if err := json.Unmarshal(body, &frag); err != nil {
			t.Fatalf("decode fragments: %v", err)
		}
		return frag
	}
	aliasFrag := fragOf(aliasRaw)
	stdFrag := fragOf(stdRaw)
	countVendor := func(grid string) int { return strings.Count(grid, `data-vendor="deepseek"`) }
	if n := countVendor(aliasFrag.GridHTML); n != 2 {
		t.Fatalf("grid not filtered by v=: %d deepseek cards in %q", n, aliasFrag.GridHTML)
	}
	if strings.Contains(aliasFrag.GridHTML, `data-vendor="opencode"`) {
		t.Fatalf("grid leaked other vendors despite v=deepseek")
	}
	if stdFrag.Total != 2 || aliasFrag.Total != 2 || stdFrag.Page != aliasFrag.Page {
		t.Fatalf("alias mismatch: std total=%d page=%d, alias total=%d page=%d",
			stdFrag.Total, stdFrag.Page, aliasFrag.Total, aliasFrag.Page)
	}
}

// TestRenderEntryHTMLLazy: the single-card fragment endpoint must render a
// skeleton for a Missing entry and real data once cached.
func TestRenderEntryHTMLLazy(t *testing.T) {
	rt := testRuntime([]*scannedEntry{testEntry("opencode", "oc-key-A")})

	view := selectPageView(rt, pageQuery{PageSize: 20})
	pageMissing := buildPageData(rt, view)
	skel := renderEntryHTML(pageMissing, 0)
	if !strings.Contains(skel, "data-entry-idx=\"0\"") || !strings.Contains(skel, "data-state=\"missing\"") {
		t.Fatalf("missing card = %q, want entry-idx + missing state", skel)
	}
	if !strings.Contains(skel, "sk-body") {
		t.Fatalf("missing card should render a skeleton body, got %q", skel)
	}

	rt.quota["opencode\x00oc-key-A"] = &quotaData{
		Kind: "balance", FetchedAt: 99,
		Balance: &balanceInfo{Currency: "CNY", Total: "9.67"},
	}
	pageCached := buildPageData(rt, view)
	frag := buildPageFragments(pageCached)
	frag.EntryIdx = 0
	frag.EntryHTML = renderEntryHTML(pageCached, 0)
	if frag.EntryHTML == "" || strings.Contains(frag.EntryHTML, "sk-body") {
		t.Fatalf("cached card should not be a skeleton, got %q", frag.EntryHTML)
	}
	if !strings.Contains(frag.EntryHTML, "data-state=\"fresh\"") {
		t.Fatalf("just-fetched card should be fresh, got %q", frag.EntryHTML)
	}
	if frag.EntryIdx != 0 {
		t.Fatalf("entry idx = %d, want 0", frag.EntryIdx)
	}
	if !strings.Contains(frag.EntryHTML, "¥9.67") {
		t.Fatalf("card should show balance, got %q", frag.EntryHTML)
	}
}

// TestFragmentJSONKeepsEntryIdxZero: entry-idx=0 is a real card (the first
// skeleton). omitempty used to drop entryIdx:0 from the payload and the client
// then discarded the successful fill, leaving the card loading forever.
func TestFragmentJSONKeepsEntryIdxZero(t *testing.T) {
	rt := testRuntime([]*scannedEntry{testEntry("opencode", "oc-key-A")})
	view := selectPageView(rt, pageQuery{PageSize: 20})
	page := buildPageData(rt, view)

	frag := buildPageFragments(page)
	frag.EntryIdx = 0
	frag.EntryHTML = renderEntryHTML(page, 0)
	raw, err := json.Marshal(frag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"entryIdx":0`) {
		t.Fatalf("entryIdx zero must be serialized, got %s", raw)
	}
}
