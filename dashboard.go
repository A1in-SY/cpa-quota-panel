package main

import (
	"html"
	"sort"
	"strconv"
	"strings"
)

type vendorTab struct {
	ID    string
	Name  string
	Count int
}

type pageEntry struct {
	VendorID      string
	VendorName    string
	KeyTail       string
	ProviderTypes []string
	Windows       map[string]percentWindow
	Balance       *balanceInfo
	Grants        []grantRow
	Err           string
	Fresh         bool
}

type pageData struct {
	Vendors     []vendorTab
	Entries     []pageEntry
	ConfigError string
	RefreshedAt int64
}

// vendorIcon returns the official brand SVG (white glyph) for a vendor id.
// Unknown vendors fall back to a neutral circle with a letter.
func vendorIcon(id string) string {
	switch id {
	case "opencode":
		return `<svg viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M384 416H128V96H384V416ZM320 160H192V352H320V160Z" fill="#fff"/></svg>`
	case "deepseek":
		return `<svg role="img" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path fill="#fff" d="M23.748 4.651c-.254-.124-.364.113-.512.233-.051.04-.094.09-.137.137-.372.397-.806.657-1.373.626-.829-.046-1.537.214-2.163.848-.133-.782-.575-1.248-1.247-1.548-.352-.155-.708-.311-.955-.65-.172-.24-.219-.509-.305-.774-.055-.16-.11-.323-.293-.35-.2-.031-.278.136-.356.276-.313.572-.434 1.202-.422 1.84.027 1.436.633 2.58 1.838 3.393.137.094.172.187.129.323-.082.28-.18.553-.266.833-.055.179-.137.218-.328.14a5.5 5.5 0 0 1-1.737-1.179c-.857-.828-1.631-1.743-2.597-2.46a12 12 0 0 0-.689-.47c-.985-.957.13-1.743.387-1.836.27-.098.094-.433-.778-.428-.872.003-1.67.295-2.687.685a3 3 0 0 1-.465.136 9.6 9.6 0 0 0-2.883-.101c-1.885.21-3.39 1.1-4.497 2.622C.082 8.776-.231 10.854.152 13.02c.403 2.284 1.568 4.175 3.36 5.653 1.857 1.533 3.997 2.284 6.438 2.14 1.482-.085 3.132-.284 4.994-1.86.47.234.962.328 1.78.398.629.058 1.235-.031 1.705-.129.735-.155.684-.836.418-.961-2.155-1.004-1.682-.595-2.112-.926 1.095-1.295 2.768-3.598 3.284-6.733.05-.346.115-.834.108-1.114-.004-.171.035-.238.23-.257a4.2 4.2 0 0 0 1.545-.475c1.397-.763 1.96-2.016 2.093-3.517.02-.23-.004-.467-.247-.588M11.58 18.168c-2.088-1.642-3.101-2.183-3.52-2.16-.39.024-.32.472-.234.763.09.288.207.487.371.74.114.167.192.416-.113.603-.673.416-1.842-.14-1.897-.168-1.361-.801-2.5-1.86-3.301-3.306-.775-1.393-1.225-2.888-1.299-4.482-.02-.385.094-.522.477-.592a4.7 4.7 0 0 1 1.53-.038c2.131.311 3.946 1.264 5.467 2.774.868.86 1.525 1.887 2.202 2.89.72 1.066 1.494 2.082 2.48 2.915.348.291.626.513.892.677-.802.09-2.14.109-3.055-.615zm1.001-6.44a.306.306 0 0 1 .415-.287.3.3 0 0 1 .113.074.3.3 0 0 1 .086.214c0 .17-.136.307-.308.307a.303.303 0 0 1-.306-.307m3.11 1.596c-.2.081-.4.151-.591.16a1.25 1.25 0 0 1-.798-.254c-.274-.23-.47-.358-.551-.758a1.7 1.7 0 0 1 .015-.588c.07-.327-.007-.537-.238-.727-.188-.156-.426-.199-.689-.199a.6.6 0 0 1-.254-.078.253.253 0 0 1-.114-.358 1 1 0 0 1 .192-.21c.356-.202.767-.136 1.146.016.352.144.618.408 1.001.782.392.451.462.576.685.915.176.264.336.536.446.848.066.194-.02.353-.25.45"/></svg>`
	case "minimax":
		return `<svg role="img" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path fill="#fff" d="M11.43 3.92a.86.86 0 1 0-1.718 0v14.236a1.999 1.999 0 0 1-3.997 0V9.022a.86.86 0 1 0-1.718 0v3.87a1.999 1.999 0 0 1-3.997 0V11.49a.57.57 0 0 1 1.139 0v1.404a.86.86 0 0 0 1.719 0V9.022a1.999 1.999 0 0 1 3.997 0v9.134a.86.86 0 0 0 1.719 0V3.92a1.998 1.998 0 1 1 3.996 0v11.788a.57.57 0 1 1-1.139 0zm10.572 3.105a2 2 0 0 0-1.999 1.997v7.63a.86.86 0 0 1-1.718 0V3.923a1.999 1.999 0 0 0-3.997 0v16.16a.86.86 0 0 1-1.719 0V18.08a.57.57 0 1 0-1.138 0v2a1.998 1.998 0 0 0 3.996 0V3.92a.86.86 0 0 1 1.719 0v12.73a1.999 1.999 0 0 0 3.996 0V9.023a.86.86 0 1 1 1.72 0v6.686a.57.57 0 0 0 1.138 0V9.022a2 2 0 0 0-1.998-1.997"/></svg>`
	default:
		return ""
	}
}

// vendorColor returns the official brand base color of the icon tile.
func vendorColor(id string) string {
	switch id {
	case "opencode":
		return "#131010"
	case "deepseek":
		return "#5786FE"
	case "minimax":
		return "#E73562"
	default:
		return "#3b82f6"
	}
}

// maskKey converts a scanned key tail like "…AbCdEf" to the display form "sk******Ef".
func maskKey(tail string) string {
	t := strings.TrimPrefix(tail, "…")
	if len(t) < 2 {
		return "sk******" + t
	}
	return "sk******" + t[len(t)-2:]
}

func buildPageData(rt *runtime) pageData {
	out := pageData{RefreshedAt: rt.now()}
	rtMu.RLock()
	defer rtMu.RUnlock()
	out.ConfigError = rt.configError

	for _, src := range rt.sources {
		out.Vendors = append(out.Vendors, vendorTab{ID: src.ID, Name: src.Name})
	}
	for _, e := range rt.entries {
		pe := pageEntry{
			VendorID:      e.VendorID,
			VendorName:    e.VendorName,
			KeyTail:       e.KeyTail,
			ProviderTypes: append([]string(nil), e.ProviderTypes...),
		}
		if d, ok := rt.quota[e.VendorID+"\x00"+e.APIKey]; ok {
			fresh := d.FetchedAt != 0 && rt.now()-d.FetchedAt <= rt.cacheTTL
			pe.Fresh = fresh
			pe.Err = d.Err
			pe.Windows = d.Windows
			pe.Balance = d.Balance
			pe.Grants = d.Grants
		}
		out.Entries = append(out.Entries, pe)
	}
	// Tabs in configured order; counts on 全部.
	for i := range out.Vendors {
		for _, e := range out.Entries {
			if e.VendorID == out.Vendors[i].ID {
				out.Vendors[i].Count++
			}
		}
	}
	sort.SliceStable(out.Entries, func(a, b int) bool {
		if out.Entries[a].VendorID == out.Entries[b].VendorID {
			return out.Entries[a].KeyTail < out.Entries[b].KeyTail
		}
		return out.Entries[a].VendorID < out.Entries[b].VendorID
	})
	return out
}

func renderDashboard(page pageData) string {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>额度面板</title>\n")
	b.WriteString(dashboardCSS())
	b.WriteString("</head>\n<body>\n")
	b.WriteString(dashboardBody(page))
	b.WriteString("<script>\n")
	b.WriteString(dashboardJS())
	b.WriteString("\n</script>\n</body>\n</html>\n")
	return b.String()
}

func dashboardCSS() string {
	return `<style>
/* ============ 1) 独立预览默认值（浅色） ============ */
:root{
  --qp-bg:#eff2f7;
  --qp-bg-gradient:linear-gradient(120deg,#f0f7ff 0%,#e7f2ff 50%,#edf7ff 100%);
  --qp-blob-1:#7aa2ff; --qp-blob-2:#6bc5ff;
  --qp-surface:rgba(255,255,255,.9); --qp-surface-strong:#fff; --qp-surface-muted:rgba(255,255,255,.62);
  --qp-border:rgba(15,23,42,.08); --qp-border-strong:rgba(15,23,42,.12);
  --qp-text-primary:#2c3e50; --qp-text-regular:#5f6c7b; --qp-text-muted:#8b95a6;
  --qp-primary:#3b82f6; --qp-primary-hover:#60a5fa; --qp-primary-active:#2563eb; --qp-primary-contrast:#fff;
  --qp-green:#22c55e; --qp-green-dark:#16a34a; --qp-green-bg:#f0fdf4; --qp-green-bd:#bbf7d0;
  --qp-amber:#f59e0b; --qp-amber-dark:#d97706; --qp-amber-bg:#fffbeb; --qp-amber-bd:#fde68a;
  --qp-red:#ef4444; --qp-red-dark:#dc2626; --qp-red-bg:#fef2f2; --qp-red-bd:#fecaca;
  --qp-slate-dark:#475569; --qp-slate-bg:#f8fafc; --qp-slate-bd:#e2e8f0;
  --qp-track:var(--qp-slate-bd);
  --qp-radius-lg:20px; --qp-radius-md:12px; --qp-radius-sm:8px; --qp-radius-full:9999px;
  --qp-gap:20px; --qp-card-padding:20px; --qp-blur:20px;
  color-scheme:light;
}
/* ============ 2) 深色回退（CPAMP 深色 / 兜底） ============ */
html[data-theme='dark'], html.theme-dark{
  --qp-bg:#0a0a0a;
  --qp-bg-gradient:linear-gradient(120deg,#0b1324 0%,#0a1426 50%,#091521 100%);
  --qp-blob-1:#1b2a55; --qp-blob-2:#0f3d59;
  --qp-surface:rgba(24,28,40,.9); --qp-surface-strong:#1b1f2a; --qp-surface-muted:rgba(255,255,255,.08);
  --qp-border:rgba(255,255,255,.08); --qp-border-strong:rgba(255,255,255,.12);
  --qp-text-primary:#e5e5e5; --qp-text-regular:#a3a3a3; --qp-text-muted:#7a7a7a;
  --qp-primary:#60a5fa; --qp-primary-hover:#93c5fd; --qp-primary-active:#60a5fa; --qp-primary-contrast:#08111f;
  --qp-green:#4ade80; --qp-green-dark:#86efac; --qp-green-bg:rgba(74,222,128,.14); --qp-green-bd:rgba(74,222,128,.24);
  --qp-amber:#fbbf24; --qp-amber-dark:#fcd34d; --qp-amber-bg:rgba(251,191,36,.14); --qp-amber-bd:rgba(251,191,36,.24);
  --qp-red:#f87171; --qp-red-dark:#fca5a5; --qp-red-bg:rgba(248,113,113,.14); --qp-red-bd:rgba(248,113,113,.24);
  --qp-slate-dark:#cbd5e1; --qp-slate-bg:rgba(148,163,184,.12); --qp-slate-bd:rgba(148,163,184,.2);
  --qp-track:rgba(255,255,255,.08);
  color-scheme:dark;
}
/* ============ 3) CPAMP 宿主内：绑定其实时注入的主题变量（优先级最高） ============ */
html[data-cpamp-plugin-host='true']{
  --qp-bg:var(--app-bg, var(--bg-secondary,#eff2f7));
  --qp-bg-gradient:var(--app-bg-gradient, var(--qp-bg-gradient));
  --qp-blob-1:var(--app-bg-blob-1-start,#7aa2ff); --qp-blob-2:var(--app-bg-blob-2-start,#6bc5ff);
  --qp-surface:var(--app-surface, var(--bg-primary,#fff));
  --qp-surface-strong:var(--app-surface-strong,#fff);
  --qp-surface-muted:var(--app-surface-muted, var(--bg-tertiary,#f6faff));
  --qp-border:var(--app-border, var(--border-color,rgba(15,23,42,.08)));
  --qp-border-strong:var(--app-border-strong,rgba(15,23,42,.12));
  --qp-text-primary:var(--app-text-primary, var(--text-primary,#2c3e50));
  --qp-text-regular:var(--app-text-regular, var(--text-secondary,#5f6c7b));
  --qp-text-muted:var(--app-text-muted, var(--text-tertiary,#8b95a6));
  --qp-primary:var(--primary-color,#409eff);
  --qp-primary-hover:var(--primary-hover,#79bbff);
  --qp-primary-active:var(--primary-active,#337ecc);
  --qp-primary-contrast:var(--primary-contrast,#fff);
  --qp-green:var(--data-green-base,#22c55e); --qp-green-dark:var(--data-green-dark-2,#16a34a);
  --qp-green-bg:var(--data-green-light-9,#f0fdf4); --qp-green-bd:var(--data-green-light-7,#bbf7d0);
  --qp-amber:var(--data-amber-base,#f59e0b); --qp-amber-dark:var(--data-amber-dark-2,#d97706);
  --qp-amber-bg:var(--data-amber-light-9,#fffbeb); --qp-amber-bd:var(--data-amber-light-7,#fde68a);
  --qp-red:var(--data-red-base,#ef4444); --qp-red-dark:var(--data-red-dark-2,#dc2626);
  --qp-red-bg:var(--data-red-light-9,#fef2f2); --qp-red-bd:var(--data-red-light-7,#fecaca);
  --qp-slate-dark:var(--data-slate-dark-2,#475569); --qp-slate-bg:var(--data-slate-light-9,#f8fafc); --qp-slate-bd:var(--data-slate-light-7,#e2e8f0);
  --qp-track:var(--data-track-bg, var(--data-slate-light-7,#e2e8f0));
  background:var(--bg-primary);
}

*{box-sizing:border-box}
html,body{width:100%;min-height:100%}
body{margin:0;color:var(--qp-text-primary);
  font:14px/1.6 -apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;
  background:var(--qp-bg);background-image:var(--qp-bg-gradient);background-attachment:fixed;position:relative;
  min-height:100vh;padding:24px;transition:background-color .25s ease,color .25s ease}
body::before,body::after{content:"";position:fixed;border-radius:50%;filter:blur(70px);z-index:0;pointer-events:none}
body::before{width:420px;height:420px;top:-120px;right:-80px;background:radial-gradient(circle,var(--qp-blob-1),transparent 70%);opacity:.45}
body::after{width:460px;height:460px;bottom:-160px;left:-120px;background:radial-gradient(circle,var(--qp-blob-2),transparent 70%);opacity:.4}
.main{position:relative;z-index:1;max-width:1100px;margin:0 auto;display:flex;flex-direction:column;gap:var(--qp-gap)}

.card{background:var(--qp-surface);border:1px solid var(--qp-border);border-radius:var(--qp-radius-lg);
  backdrop-filter:blur(var(--qp-blur));-webkit-backdrop-filter:blur(var(--qp-blur));padding:var(--qp-card-padding)}
.head .row{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;flex-wrap:wrap}
h1{font-size:20px;margin:0;font-weight:700;letter-spacing:.2px}
.sub{color:var(--qp-text-muted);font-size:12px;margin:3px 0 0}
.head-actions{display:flex;align-items:center;gap:12px}
.last-refresh{color:var(--qp-text-muted);font-size:12px}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:1px solid transparent;border-radius:var(--qp-radius-md);
  height:38px;padding:0 16px;font-size:14px;font-weight:600;cursor:pointer;transition:all .15s ease;color:var(--qp-text-regular);
  background:var(--qp-surface-muted)}
.btn-primary{background:var(--qp-primary-active);color:var(--qp-primary-contrast);border-color:var(--qp-primary-active);
  box-shadow:0 8px 18px -14px color-mix(in srgb,var(--qp-primary-active) 72%,transparent)}
.btn-primary:hover{background:var(--qp-primary);transform:translateY(-1px);border-color:var(--qp-primary)}
.btn-primary:active{transform:translateY(0)}
.spin{display:none;width:14px;height:14px;border-radius:50%;border:2px solid color-mix(in srgb,var(--qp-primary-contrast) 45%,transparent);border-top-color:var(--qp-primary-contrast);animation:spin .8s linear infinite}
.btn.loading .spin{display:inline-block}
@keyframes spin{to{transform:rotate(360deg)}}

.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:16px}
.stat{background:var(--qp-surface);border:1px solid var(--qp-border);border-radius:var(--qp-radius-md);padding:14px 16px;
  backdrop-filter:blur(var(--qp-blur));-webkit-backdrop-filter:blur(var(--qp-blur));display:flex;flex-direction:column;gap:2px}
.stat .label{color:var(--qp-text-regular);font-size:12px}
.stat .value{font-weight:800;font-size:20px;color:var(--qp-text-primary);font-variant-numeric:tabular-nums}
.stat .value.green{color:var(--qp-green)}
.stat .value.red{color:var(--qp-red)}

.tabs{display:flex;flex-wrap:wrap;gap:8px}
.tab{display:inline-flex;align-items:center;gap:6px;height:34px;padding:0 14px;border-radius:var(--qp-radius-full);
  border:1px solid var(--qp-border);background:var(--qp-surface);color:var(--qp-text-regular);font-size:13px;font-weight:600;cursor:pointer;transition:all .15s ease}
.tab:hover{background:color-mix(in srgb,var(--qp-primary) 8%,transparent);color:var(--qp-primary)}
.tab.active{background:var(--qp-primary);border-color:var(--qp-primary);color:var(--qp-primary-contrast);box-shadow:0 6px 14px -8px color-mix(in srgb,var(--qp-primary) 60%,transparent)}
.tab .count{font-size:11px;opacity:.75;font-weight:600}
.tab.active .count{opacity:.9}

.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(335px,1fr));gap:var(--qp-gap)}
.entry{background:var(--qp-surface);border:1px solid var(--qp-border);border-radius:var(--qp-radius-lg);
  backdrop-filter:blur(var(--qp-blur));-webkit-backdrop-filter:blur(var(--qp-blur));padding:18px 20px;display:none;flex-direction:column;gap:12px}
.entry.on{display:flex;animation:fadeIn .2s ease}
@keyframes fadeIn{from{opacity:0;transform:translateY(4px)}to{opacity:1;transform:none}}
.entry-head{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap}
.entry-title{display:flex;align-items:center;gap:10px;min-width:0}
.entry-icon{width:38px;height:38px;border-radius:10px;display:grid;place-items:center;flex:0 0 38px;
  overflow:hidden;box-shadow:0 1px 2px rgba(20,40,90,.08)}
.entry-icon svg{width:22px;height:22px;display:block}
.entry-name{font-weight:700;font-size:15px}
.entry-key{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px;
  color:var(--qp-text-muted);letter-spacing:.3px;white-space:nowrap;margin-left:auto;align-self:center}
.entry-meta{display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin-top:2px}
.badge{display:inline-flex;align-items:center;gap:4px;font-size:11px;padding:2px 9px;border-radius:var(--qp-radius-full);border:1px solid;font-weight:600;white-space:nowrap}
.badge.type{color:var(--qp-slate-dark);background:var(--qp-slate-bg);border-color:var(--qp-slate-bd)}
.badge.fresh{color:var(--qp-green-dark);background:var(--qp-green-bg);border-color:var(--qp-green-bd)}
.badge.stale{color:var(--qp-amber-dark);background:var(--qp-amber-bg);border-color:var(--qp-amber-bd)}
.badge.err{color:var(--qp-red-dark);background:var(--qp-red-bg);border-color:var(--qp-red-bd)}
.entry-body{display:flex;flex-direction:column;gap:10px}

.win{display:flex;flex-direction:column;gap:5px}
.win .whead{display:flex;justify-content:space-between;align-items:baseline;font-size:12px;color:var(--qp-text-regular)}
.win .whead b{font-weight:700;color:var(--qp-text-primary);font-variant-numeric:tabular-nums}
.bar{height:8px;border-radius:var(--qp-radius-full);background:var(--qp-track);overflow:hidden}
.bar>i{display:block;height:100%;border-radius:var(--qp-radius-full);transition:width .35s ease;background:var(--qp-green)}
.bar>i.amber{background:var(--qp-amber)}
.bar>i.red{background:var(--qp-red)}
.win .reset{font-size:11px;color:var(--qp-text-muted)}

.balance{display:flex;align-items:flex-end;justify-content:space-between;gap:12px}
.balance .left{display:flex;flex-direction:column;gap:2px}
.balance .caption{font-size:12px;color:var(--qp-text-muted)}
.balance .amount{font-size:30px;font-weight:800;line-height:1.1;color:var(--qp-green);font-variant-numeric:tabular-nums}
.balance .amount.red{color:var(--qp-red)}
.balance .break{display:flex;flex-direction:column;gap:2px;font-size:12px;color:var(--qp-text-regular);text-align:right;font-variant-numeric:tabular-nums}
.balance .break b{color:var(--qp-text-primary)}

.errbox{display:flex;align-items:flex-start;gap:10px;padding:12px 14px;border-radius:var(--qp-radius-md);
  border:1px solid var(--qp-red-bd);background:var(--qp-red-bg);color:var(--qp-red-dark);font-size:12px;line-height:1.5}
.errbox .ic{flex:0 0 auto;font-weight:800}
.empty{color:var(--qp-text-muted);padding:32px 0;text-align:center}
#configErr{display:none;padding:12px 16px;border-radius:var(--qp-radius-md);border:1px solid var(--qp-red-bd);background:var(--qp-red-bg);color:var(--qp-red-dark);font-size:12px}
#configErr.on{display:block}
::-webkit-scrollbar{width:8px;height:8px}
::-webkit-scrollbar-thumb{background:var(--qp-slate-bd);border-radius:var(--qp-radius-full)}
@media(max-width:640px){body{padding:16px 12px}.amount{font-size:24px}}
</style>`
}

func dashboardBody(page pageData) string {
	var b strings.Builder
	b.WriteString("<div class=\"main\">\n")

	b.WriteString("<div class=\"card head\"><div class=\"row\"><div><h1>额度面板</h1>")
	b.WriteString("<p class=\"sub\">只读额度查询 · 数据来源：各厂商额度接口 · 分类筛选用</p></div>")
	b.WriteString("<div class=\"head-actions\"><span class=\"last-refresh\" id=\"refreshAt\"></span>")
	b.WriteString("<button class=\"btn btn-primary\" id=\"refresh\"><span class=\"spin\"></span>刷新</button></div></div></div>\n")

	if page.ConfigError != "" {
		b.WriteString("<div id=\"configErr\" class=\"on\">配置读取失败：" + html.EscapeString(page.ConfigError) + "</div>\n")
	} else {
		b.WriteString("<div id=\"configErr\"></div>\n")
	}

	b.WriteString("<div class=\"stats\" id=\"stats\"></div>\n")
	b.WriteString("<div class=\"tabs card\" id=\"tabs\">")
	total := 0
	for range page.Entries {
		total++
	}
	writeTab(&b, "", "全部", total, true)
	for _, v := range page.Vendors {
		writeTab(&b, v.ID, v.Name, v.Count, false)
	}
	b.WriteString("</div>\n")
	if len(page.Entries) == 0 {
		if page.ConfigError == "" {
			b.WriteString("<div class=\"empty\">白名单内暂无命中的 AI 提供商条目（或 config 中还没有对应 key）。</div>\n")
		}
		b.WriteString("</div>\n")
		return b.String()
	}
	b.WriteString("<div class=\"grid\" id=\"grid\">\n")
	for _, e := range page.Entries {
		renderEntry(&b, e)
	}
	b.WriteString("</div>\n")
	b.WriteString("<div class=\"empty\" id=\"emptyWrap\"></div>\n")
	b.WriteString("</div>\n")
	return b.String()
}

func percentInt(p float64) int {
	return int(p + 0.5)
}

func writeTab(b *strings.Builder, id, name string, count int, active bool) {
	cls := ""
	if active {
		cls = " active"
	}
	b.WriteString("<button class=\"tab" + cls + "\" data-tab=\"" + html.EscapeString(id) + "\">" +
		html.EscapeString(name) + "<span class=\"count\">" + strconv.Itoa(count) + "</span></button>")
}

func toneClass(used float64) string {
	rem := 100 - used
	if rem >= 70 {
		return ""
	}
	if rem >= 30 {
		return "amber"
	}
	return "red"
}

func renderEntry(b *strings.Builder, e pageEntry) {
	icon := vendorIcon(e.VendorID)
	icBg := vendorColor(e.VendorID)
	iconHTML := icon
	if icon == "" {
		// fallback letter tile for unknown vendors
		letter := "?"
		if e.VendorName != "" {
			letter = strings.ToUpper(e.VendorName[:1])
		}
		iconHTML = "<span style=\"color:#fff;font-weight:800;font-size:16px\">" + html.EscapeString(letter) + "</span>"
	}
	b.WriteString("<div class=\"entry\" data-vendor=\"" + html.EscapeString(e.VendorID) + "\">\n")
	b.WriteString("<div class=\"entry-head\"><div class=\"entry-title\">")
	b.WriteString("<span class=\"entry-icon\" style=\"background:" + icBg + "\">" + iconHTML + "</span>")
	b.WriteString("<div><div class=\"entry-name\">" + html.EscapeString(e.VendorName) + "</div>")
	b.WriteString("<div class=\"entry-meta\">")
	for _, t := range e.ProviderTypes {
		b.WriteString("<span class=\"badge type\">" + html.EscapeString(t) + "</span>")
	}
	if e.Fresh {
		b.WriteString("<span class=\"badge fresh\">● 实时</span>")
	} else {
		b.WriteString("<span class=\"badge stale\">● 缓存</span>")
	}
	b.WriteString("</div></div></div>")
	b.WriteString("<span class=\"entry-key\">" + html.EscapeString(maskKey(e.KeyTail)) + "</span>")
	if e.Err != "" {
		b.WriteString("<span class=\"badge err\">异常</span>")
	}
	b.WriteString("</div>\n")
	b.WriteString("<div class=\"entry-body\">")
	if e.Err != "" {
		b.WriteString("<div class=\"errbox\"><span class=\"ic\">⚠</span><span>" + html.EscapeString(e.Err) + "</span></div>")
	} else {
		switch {
		case e.Windows != nil:
			for _, key := range []string{"rolling", "weekly", "monthly"} {
				renderWindow(b, key, e.Windows[key])
			}
		case e.Balance != nil:
			renderBalance(b, e.Balance)
		case len(e.Grants) > 0:
			for _, g := range e.Grants {
				b.WriteString("<div class=\"grant\">剩余 " + html.EscapeString(g.Remaining))
				if g.ExpiresAt != "" {
					b.WriteString(" · 到期 " + html.EscapeString(g.ExpiresAt))
				}
				b.WriteString("</div>")
			}
		default:
			b.WriteString("<div class=\"empty\">该条目暂无额度数据</div>")
		}
	}
	b.WriteString("</div></div>\n")
}

func renderWindow(b *strings.Builder, key string, w percentWindow) {
	cls := toneClass(w.Percent)
	remaining := 100 - w.Percent
	label := map[string]string{"rolling": "5小时窗口", "weekly": "每周窗口", "monthly": "每月窗口"}[key]
	b.WriteString("<div class=\"win\"><div class=\"whead\"><span>" + label + "</span>")
	b.WriteString("<span><b>剩余 " + strconv.Itoa(percentInt(remaining)) + "%</b> · 已用 " + strconv.Itoa(percentInt(w.Percent)) + "%")
	if w.Status == "rate-limited" {
		b.WriteString(" · 限流")
	}
	b.WriteString("</span></div>\n")
	b.WriteString("<div class=\"bar\"><i class=\"" + cls + "\" style=\"width:" + strconv.Itoa(percentInt(remaining)) + "%\"></i></div>\n")
	if w.ResetsAt != "" {
		b.WriteString("<div class=\"reset\">重置：<span data-reset=\"" + html.EscapeString(w.ResetsAt) + "\"></span>")
		if w.Status == "rate-limited" {
			b.WriteString(" · 等待自动恢复")
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
}

func renderBalance(b *strings.Builder, bi *balanceInfo) {
	low := false
	if v, err := strconv.ParseFloat(strings.ReplaceAll(bi.Total, ",", ""), 64); err == nil {
		low = v < 3
	}
	b.WriteString("<div class=\"balance\"><div class=\"left\"><span class=\"caption\">账户余额</span>")
	b.WriteString("<span class=\"amount")
	if low {
		b.WriteString(" red")
	}
	b.WriteString("\">" + html.EscapeString(currencyPrefix(bi.Currency)) + html.EscapeString(bi.Total) + "</span></div>")
	b.WriteString("<div class=\"break\">")
	if bi.Granted != "" && bi.Granted != "0" {
		b.WriteString("<span>赠送 <b>" + html.EscapeString(bi.Granted) + "</b></span>")
	}
	if bi.ToppedUp != "" && bi.ToppedUp != "0" {
		b.WriteString("<span>充值 <b>" + html.EscapeString(bi.ToppedUp) + "</b></span>")
	}
	b.WriteString("</div></div>\n")
}

func currencyPrefix(c string) string {
	switch strings.ToUpper(c) {
	case "USD":
		return "$"
	case "CNY":
		return "¥"
	case "EUR":
		return "€"
	default:
		if c == "" {
			return ""
		}
		return c + " "
	}
}

func dashboardJS() string {
	return `
(function(){
  var showStats = document.getElementById('stats');
  if (showStats) {
    var cards = document.querySelectorAll('.entry');
    var total = cards.length;
    var ok = 0, err = 0, fresh = 0;
    cards.forEach(function(c){
      if (c.querySelector('.errbox') || c.querySelector('.badge.err')) err++;
      else ok++;
      if (c.querySelector('.badge.fresh')) fresh++;
    });
    function stat(label, value, cls){
      var d=document.createElement('div');d.className='stat';
      d.innerHTML='<div class="label">'+label+'</div><div class="value '+(cls||'')+'">'+value+'</div>';
      return d;
    }
    showStats.append(
      stat('条目总数', total),
      stat('可用', ok, 'green'),
      stat('异常', err, err>0?'red':'green'),
      stat('实时', fresh+'/'+total, fresh>0?'green':'')
    );
  }

  var tabs = document.querySelectorAll('[data-tab]');
  var refreshBtn = document.getElementById('refresh');
  if (refreshBtn) refreshBtn.addEventListener('click', function(){
    var u = new URL(window.location.href);
    u.searchParams.set('refresh','1');
    u.searchParams.delete('v');
    window.location.href = u.toString();
  });
  function applyTab(id){
    document.querySelectorAll('[data-tab]').forEach(function(t){ t.classList.toggle('active', t.getAttribute('data-tab')===id); });
    document.querySelectorAll('.entry').forEach(function(c){
      c.classList.toggle('on', !id || c.getAttribute('data-vendor')===id);
    });
    var wrap = document.getElementById('emptyWrap');
    if (wrap) wrap.textContent = '';
  }
  function onTab(e){
    var id = e.currentTarget.getAttribute('data-tab');
    try { history.replaceState(null,'','?v='+encodeURIComponent(id)); } catch(_) {}
    applyTab(id);
  }
  tabs.forEach(function(t){ t.addEventListener('click', onTab); });
  var q = new URLSearchParams(window.location.search).get('v');
  applyTab(q || '');

  var rt = document.getElementById('refreshAt');
  if (rt) {
    try {
      var last = new Date(document.lastModified);
      rt.textContent = '上次 ' + last.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'});
    } catch(_){ rt.textContent=''; }
  }

  function fmt(sec){ sec=Math.max(0,Math.floor(sec)); var d=Math.floor(sec/86400),h=Math.floor((sec%86400)/3600),m=Math.floor((sec%3600)/60); var p=[]; if(d>0)p.push(d+'日'); if(h>0)p.push(h+'时'); if(m>0||p.length===0)p.push(m+'分'); return p.join(' '); }
  function tick(){
    var now=Date.now();
    document.querySelectorAll('.reset span[data-reset]').forEach(function(el){
      var t=new Date(el.getAttribute('data-reset')).getTime();
      if (isNaN(t)) { el.textContent = el.getAttribute('data-reset'); return; }
      var wait=Math.round((t-now)/1000);
      el.textContent = wait>0 ? ('约 '+fmt(wait)+' 后') : '即将重置';
    });
  }
  tick(); setInterval(tick,5000);
})();`
}
