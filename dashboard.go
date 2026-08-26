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
	Idx           int
	VendorID      string
	VendorName    string
	KeyTail       string
	ProviderTypes []string
	Windows       map[string]percentWindow
	Balance       *balanceInfo
	Grants        []grantRow
	Models        []modelPlan
	Err           string
	Fresh         bool
	// Missing reports that no quota result is cached yet; the card renders as a
	// skeleton and the client lazy-loads it via ?entry-idx=.
	Missing bool
}

type pageData struct {
	Vendors     []vendorTab
	Entries     []pageEntry
	ConfigError string
	RefreshedAt int64
	Vendor      string
	Page        int
	PageSize    int
	Total       int
}

// pageQuery describes the server-side pagination requested by the client.
type pageQuery struct {
	Vendor   string
	Page     int
	PageSize int
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// pageFragments is the JSON payload returned for in-place refresh requests
// (?partial=1). The client swaps these fragments into the live page so the
// skeleton state never causes a full navigation. When only one card is being
// lazily loaded (?partial=1&entry-idx=N), EntryIdx/EntryHTML carry that single
// card's replacement HTML.
type pageFragments struct {
	ConfigError string `json:"configError"`
	RefreshedAt int64  `json:"refreshedAt"`
	TabsHTML    string `json:"tabsHTML"`
	GridHTML    string `json:"gridHTML"`
	EmptyText   string `json:"emptyText"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
	Total       int    `json:"total"`
	EntryIdx    int    `json:"entryIdx,omitempty"`
	EntryHTML   string `json:"entryHTML,omitempty"`
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

func providerTypeIcon(code string) string {
	switch strings.TrimSpace(code) {
	case "codex":
		return `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M19.503 0H4.496A4.496 4.496 0 000 4.496v15.007A4.496 4.496 0 004.496 24h15.007A4.496 4.496 0 0024 19.503V4.496A4.496 4.496 0 0019.503 0z" fill="#fff"/><path d="M9.064 3.344a4.578 4.578 0 012.285-.312c1 .115 1.891.54 2.673 1.275.01.01.024.017.037.021a.09.09 0 00.043 0 4.55 4.55 0 013.046.275l.047.022.116.057a4.581 4.581 0 012.188 2.399c.209.51.313 1.041.315 1.595a4.24 4.24 0 01-.134 1.223.123.123 0 00.03.115c.594.607.988 1.33 1.183 2.17.289 1.425-.007 2.71-.887 3.854l-.136.166a4.548 4.548 0 01-2.201 1.388.123.123 0 00-.081.076c-.191.551-.383 1.023-.74 1.494-.9 1.187-2.222 1.846-3.711 1.838-1.187-.006-2.239-.44-3.157-1.302a.107.107 0 00-.105-.024c-.388.125-.78.143-1.204.138a4.441 4.441 0 01-1.945-.466 4.544 4.544 0 01-1.61-1.335c-.152-.202-.303-.392-.414-.617a5.81 5.81 0 01-.37-.961 4.582 4.582 0 01-.014-2.298.124.124 0 00.006-.056.085.085 0 00-.027-.048 4.467 4.467 0 01-1.034-1.651 3.896 3.896 0 01-.251-1.192 5.189 5.189 0 01.141-1.6c.337-1.112.982-1.985 1.933-2.618.212-.141.413-.251.601-.33.215-.089.43-.164.646-.227a.098.098 0 00.065-.066 4.51 4.51 0 01.829-1.615 4.535 4.535 0 011.837-1.388zm3.482 10.565a.637.637 0 000 1.272h3.636a.637.637 0 100-1.272h-3.636zM8.462 9.23a.637.637 0 00-1.106.631l1.272 2.224-1.266 2.136a.636.636 0 101.095.649l1.454-2.455a.636.636 0 00.005-.64L8.462 9.23z" fill="url(#qp-icon-codex)"/><defs><linearGradient gradientUnits="userSpaceOnUse" id="qp-icon-codex" x1="12" x2="12" y1="3" y2="21"><stop stop-color="#B1A7FF"/><stop offset=".5" stop-color="#7A9DFF"/><stop offset="1" stop-color="#3941FF"/></linearGradient></defs></svg>`
	case "claude":
		return `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M4.709 15.955l4.72-2.647.08-.23-.08-.128H9.2l-.79-.048-2.698-.073-2.339-.097-2.266-.122-.571-.121L0 11.784l.055-.352.48-.321.686.06 1.52.103 2.278.158 1.652.097 2.449.255h.389l.055-.157-.134-.098-.103-.097-2.358-1.596-2.552-1.688-1.336-.972-.724-.491-.364-.462-.158-1.008.656-.722.881.06.225.061.893.686 1.908 1.476 2.491 1.833.365.304.145-.103.019-.073-.164-.274-1.355-2.446-1.446-2.49-.644-1.032-.17-.619a2.97 2.97 0 01-.104-.729L6.283.134 6.696 0l.996.134.42.364.62 1.414 1.002 2.229 1.555 3.03.456.898.243.832.091.255h.158V9.01l.128-1.706.237-2.095.23-2.695.08-.76.376-.91.747-.492.584.28.48.685-.067.444-.286 1.851-.559 2.903-.364 1.942h.212l.243-.242.985-1.306 1.652-2.064.73-.82.85-.904.547-.431h1.033l.76 1.129-.34 1.166-1.064 1.347-.881 1.142-1.264 1.7-.79 1.36.073.11.188-.02 2.856-.606 1.543-.28 1.841-.315.833.388.091.395-.328.807-1.969.486-2.309.462-3.439.813-.042.03.049.061 1.549.146.662.036h1.622l3.02.225.79.522.474.638-.079.485-1.215.62-1.64-.389-3.829-.91-1.312-.329h-.182v.11l1.093 1.068 2.006 1.81 2.509 2.33.127.578-.322.455-.34-.049-2.205-1.657-.851-.747-1.926-1.62h-.128v.17l.444.649 2.345 3.521.122 1.08-.17.353-.608.213-.668-.122-1.374-1.925-1.415-2.167-1.143-1.943-.14.08-.674 7.254-.316.37-.729.28-.607-.461-.322-.747.322-1.476.389-1.924.315-1.53.286-1.9.17-.632-.012-.042-.14.018-1.434 1.967-2.18 2.945-1.726 1.845-.414.164-.717-.37.067-.662.401-.589 2.388-3.036 1.44-1.882.93-1.086-.006-.158h-.055L4.132 18.56l-1.13.146-.487-.456.061-.746.231-.243 1.908-1.312-.006.006z" fill="#D97757"/></svg>`
	case "gemini":
		return `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#qp-icon-gemini-a)"/><path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#qp-icon-gemini-b)"/><path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#qp-icon-gemini-c)"/><defs><linearGradient id="qp-icon-gemini-a" gradientUnits="userSpaceOnUse" x1="7" x2="11" y1="15.5" y2="12"><stop stop-color="#08B962"/><stop offset="1" stop-color="#08B962" stop-opacity="0"/></linearGradient><linearGradient id="qp-icon-gemini-b" gradientUnits="userSpaceOnUse" x1="8" x2="11.5" y1="5.5" y2="11"><stop stop-color="#F94543"/><stop offset="1" stop-color="#F94543" stop-opacity="0"/></linearGradient><linearGradient id="qp-icon-gemini-c" gradientUnits="userSpaceOnUse" x1="3.5" x2="17.5" y1="13.5" y2="12"><stop stop-color="#FABC12"/><stop offset=".46" stop-color="#FABC12" stop-opacity="0"/></linearGradient></defs></svg>`
	case "vertex":
		return `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M20,13.89A.77.77,0,0,0,19,13.73l-7,5.14v.22a.72.72,0,1,1,0,1.43v0a.74.74,0,0,0,.45-.15l7.41-5.47A.76.76,0,0,0,20,13.89Z" fill="#669df6"/><path d="M12,20.52a.72.72,0,0,1,0-1.43h0v-.22L5,13.73a.76.76,0,0,0-1,.16.74.74,0,0,0,.16,1l7.41,5.47a.73.73,0,0,0,.44.15v0Z" fill="#aecbfa"/><path d="M12,18.34a1.47,1.47,0,1,0,1.47,1.47A1.47,1.47,0,0,0,12,18.34Zm0,2.18a.72.72,0,1,1,.72-.71A.71.71,0,0,1,12,20.52Z" fill="#4285f4"/><path d="M6,6.11a.76.76,0,0,1-.75-.75V3.48a.76.76,0,1,1,1.51,0V5.36A.76.76,0,0,1,6,6.11Z" fill="#aecbfa"/><circle cx="5.98" cy="12" r="0.76" fill="#aecbfa"/><circle cx="5.98" cy="9.79" r="0.76" fill="#aecbfa"/><circle cx="5.98" cy="7.57" r="0.76" fill="#aecbfa"/><path d="M18,8.31a.76.76,0,0,1-.75-.76V5.67a.75.75,0,1,1,1.5,0V7.55A.75.75,0,0,1,18,8.31Z" fill="#4285f4"/><circle cx="18.02" cy="12.01" r="0.76" fill="#4285f4"/><circle cx="18.02" cy="9.76" r="0.76" fill="#4285f4"/><circle cx="18.02" cy="3.48" r="0.76" fill="#4285f4"/><path d="M12,15a.76.76,0,0,1-.75-.75V12.34a.76.76,0,0,1,1.51,0v1.89A.76.76,0,0,1,12,15Z" fill="#669df6"/><circle cx="12" cy="16.45" r="0.76" fill="#669df6"/><circle cx="12" cy="10.14" r="0.76" fill="#669df6"/><circle cx="12" cy="7.92" r="0.76" fill="#669df6"/><path d="M15,10.54a.76.76,0,0,1-.75-.75V7.91a.76.76,0,1,1,1.51,0V9.79A.76.76,0,0,1,15,10.54Z" fill="#4285f4"/><circle cx="15.01" cy="5.69" r="0.76" fill="#4285f4"/><circle cx="15.01" cy="14.19" r="0.76" fill="#4285f4"/><circle cx="15.01" cy="11.97" r="0.76" fill="#4285f4"/><circle cx="8.99" cy="14.19" r="0.76" fill="#aecbfa"/><circle cx="8.99" cy="7.92" r="0.76" fill="#aecbfa"/><circle cx="8.99" cy="5.69" r="0.76" fill="#aecbfa"/><path d="M9,12.73A.76.76,0,0,1,8.24,12V10.1a.75.75,0,1,1,1.5,0V12A.75.75,0,0,1,9,12.73Z" fill="#aecbfa"/></svg>`
	case "xai":
		return `<svg fill="currentColor" class="ic-type-xai" fill-rule="evenodd" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M9.27 15.29l7.978-5.897c.391-.29.95-.177 1.137.272.98 2.369.542 5.215-1.41 7.169-1.951 1.954-4.667 2.382-7.149 1.406l-2.711 1.257c3.889 2.661 8.611 2.003 11.562-.953 2.341-2.344 3.066-5.539 2.388-8.42l.006.007c-.983-4.232.242-5.924 2.75-9.383.06-.082.12-.164.179-.248l-3.301 3.305v-.01L9.267 15.292M7.623 16.723c-2.792-2.67-2.31-6.801.071-9.184 1.761-1.763 4.647-2.483 7.166-1.425l2.705-1.25a7.808 7.808 0 00-1.829-1A8.975 8.975 0 005.984 5.83c-2.533 2.536-3.33 6.436-1.962 9.764 1.022 2.487-.653 4.246-2.34 6.022-.599.63-1.199 1.259-1.682 1.925l7.62-6.815"/></svg>`
	case "openai-compatibility":
		return `<svg fill="currentColor" class="ic-type-openai" fill-rule="evenodd" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z"/></svg>`
	default:
		return ""
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

// pageView is a snapshot of one filtered page of scanned entries, taken before
// quota refresh so only the visible entries are queried.
type pageView struct {
	Vendor   string
	Page     int
	PageSize int
	Total    int
	Entries  []*scannedEntry
}

func normalizePageQuery(q pageQuery) pageQuery {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}
	return q
}

// pageWindow clamps page to the valid range and returns its slice window.
func pageWindow(total, pageSize, page int) (int, int, int) {
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	return page, start, end
}

// entryLess is the global display order: vendor id, then key tail.
func entryLess(a, b *scannedEntry) bool {
	if a.VendorID == b.VendorID {
		return a.KeyTail < b.KeyTail
	}
	return a.VendorID < b.VendorID
}

// selectPageView filters rt.entries by vendor, sorts by display order, and
// returns only the requested page.
func selectPageView(rt *runtime, q pageQuery) pageView {
	q = normalizePageQuery(q)
	rtMu.RLock()
	defer rtMu.RUnlock()
	filtered := make([]*scannedEntry, 0, len(rt.entries))
	for _, e := range rt.entries {
		if q.Vendor == "" || e.VendorID == q.Vendor {
			filtered = append(filtered, e)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool { return entryLess(filtered[i], filtered[j]) })
	page, start, end := pageWindow(len(filtered), q.PageSize, q.Page)
	return pageView{
		Vendor:   q.Vendor,
		Page:     page,
		PageSize: q.PageSize,
		Total:    len(filtered),
		Entries:  append([]*scannedEntry(nil), filtered[start:end]...),
	}
}

// buildPageData renders one filtered page. Vendor tab counts always reflect the
// full scan, not just the page.
func buildPageData(rt *runtime, view pageView) pageData {
	out := pageData{
		Vendor:   view.Vendor,
		Page:     view.Page,
		PageSize: view.PageSize,
		Total:    view.Total,
	}
	rtMu.RLock()
	defer rtMu.RUnlock()
	out.ConfigError = rt.configError

	for _, src := range rt.sources {
		out.Vendors = append(out.Vendors, vendorTab{ID: src.ID, Name: src.Name})
	}
	for i := range out.Vendors {
		for _, e := range rt.entries {
			if e.VendorID == out.Vendors[i].ID {
				out.Vendors[i].Count++
			}
		}
	}
	// "上次刷新" 反映视图内最近一次真实拉取时间；没有缓存（全是待加载卡）时为 0，
	// 前端据此不显示误导性的时间。
	var latestFetch int64
	for i, e := range view.Entries {
		pe := pageEntry{
			Idx:           i,
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
			pe.Models = d.Models
			if d.FetchedAt > latestFetch {
				latestFetch = d.FetchedAt
			}
		} else {
			pe.Missing = true
		}
		out.Entries = append(out.Entries, pe)
	}
	if latestFetch > 0 {
		out.RefreshedAt = latestFetch
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
	b.WriteString("</head>\n<body data-refreshed-at=\"" + strconv.FormatInt(page.RefreshedAt, 10) +
		"\" data-page=\"" + strconv.Itoa(page.Page) +
		"\" data-page-size=\"" + strconv.Itoa(page.PageSize) +
		"\" data-total=\"" + strconv.Itoa(page.Total) + "\">\n")
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

.pager{display:none;align-items:center;justify-content:center;gap:6px;flex-wrap:wrap;padding:14px 0 2px}
.pager.on{display:flex}
.pager.busy{opacity:.55;pointer-events:none}
.pg{min-width:34px;height:34px;padding:0 10px;display:inline-flex;align-items:center;justify-content:center;
  border-radius:var(--qp-radius-sm);border:1px solid var(--qp-border);background:var(--qp-surface);
  color:var(--qp-text-regular);font-size:13px;font-weight:600;cursor:pointer;transition:all .15s ease}
.pg:hover:not(:disabled){background:color-mix(in srgb,var(--qp-primary) 8%,transparent);color:var(--qp-primary)}
.pg:disabled{opacity:.45;cursor:default}
.pg.active{background:var(--qp-primary);border-color:var(--qp-primary);color:var(--qp-primary-contrast)}
.pg.gap{border:0;background:transparent;pointer-events:none;min-width:14px;padding:0 2px}

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
.badge.type{color:var(--qp-text-primary);background:var(--qp-surface-strong,#fff);border:0;padding:2px;line-height:0;border-radius:6px}
.badge.type svg{width:14px;height:14px;display:block}
.badge.type.fallback{color:var(--qp-slate-dark);background:var(--qp-slate-bg);border:1px solid var(--qp-slate-bd);padding:2px 9px;line-height:1.6;border-radius:var(--qp-radius-full)}
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
.plan{display:flex;flex-direction:column;gap:8px}
.plan .pname{font-size:12px;font-weight:700;color:var(--qp-text-primary)}

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
/* ============ 刷新 loading：按钮 / tabs / 骨架屏 ============ */
.btn.loading{opacity:.85;cursor:progress}
.btn:disabled{pointer-events:none}
.tabs.busy{opacity:.55;pointer-events:none}
.sk-entry{pointer-events:none;min-height:152px;gap:14px}
.sk-row{display:flex;align-items:center;gap:10px}
.sk{display:block;height:11px;border-radius:var(--qp-radius-full);
  background:linear-gradient(90deg,var(--qp-track) 25%,color-mix(in srgb,var(--qp-primary) 18%,var(--qp-surface-strong)) 50%,var(--qp-track) 75%);
  background-size:200% 100%;animation:shimmer 1.2s linear infinite}
.sk-avatar{width:38px;height:38px;flex:0 0 38px;border-radius:10px}
.sk-title{width:88px;height:14px}
.sk-key{width:112px;height:10px;margin-left:auto}
.sk-line{width:100%}
.sk-line.w70{width:70%}
.sk-body{display:flex;flex-direction:column;gap:10px;padding-top:2px;min-height:64px;justify-content:center}
@keyframes shimmer{from{background-position:200% 0}to{background-position:-200% 0}}
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
	b.WriteString("<button class=\"btn btn-primary\" id=\"refresh\"><span class=\"spin\"></span><span class=\"lbl\">刷新</span></button></div></div></div>\n")

	if page.ConfigError != "" {
		b.WriteString("<div id=\"configErr\" class=\"on\">配置读取失败：" + html.EscapeString(page.ConfigError) + "</div>\n")
	} else {
		b.WriteString("<div id=\"configErr\"></div>\n")
	}

	b.WriteString("<div class=\"stats\" id=\"stats\"></div>\n")
	b.WriteString("<div class=\"tabs card\" id=\"tabs\">")
	b.WriteString(renderTabs(page, true))
	b.WriteString("</div>\n")
	b.WriteString("<div class=\"grid\" id=\"grid\">\n")
	b.WriteString(renderGrid(page))
	b.WriteString("</div>\n")
	b.WriteString("<div class=\"pager\" id=\"pager\"></div>\n")
	b.WriteString("<div class=\"empty\" id=\"emptyWrap\">")
	b.WriteString(html.EscapeString(emptyText(page)))
	b.WriteString("</div>\n")
	b.WriteString("</div>\n")
	return b.String()
}

// renderTabs renders the vendor filter tabs. allActive pre-activates the 全部
// tab on the initial full page; partial refreshes re-apply activation client-side.
// Counts always reflect the complete scan (not the current page).
func renderTabs(page pageData, allActive bool) string {
	var b strings.Builder
	all := 0
	for _, v := range page.Vendors {
		all += v.Count
	}
	writeTab(&b, "", "全部", all, allActive)
	for _, v := range page.Vendors {
		writeTab(&b, v.ID, v.Name, v.Count, false)
	}
	return b.String()
}

// renderGrid renders every entry card. Empty page returns "".
func renderGrid(page pageData) string {
	if len(page.Entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range page.Entries {
		renderEntry(&b, e)
	}
	return b.String()
}

// renderEntryHTML renders a single card (0-based index within the current view)
// as standalone HTML for the ?entry-idx= lazy-load response.
func renderEntryHTML(page pageData, idx int) string {
	if idx < 0 || idx >= len(page.Entries) {
		return ""
	}
	var b strings.Builder
	renderEntry(&b, page.Entries[idx])
	return b.String()
}

// emptyText is the message shown when the filtered view has nothing to list.
func emptyText(page pageData) string {
	if page.ConfigError != "" || page.Total > 0 {
		return ""
	}
	if page.Vendor != "" {
		return "该厂商暂无条目"
	}
	return "白名单内暂无命中的 AI 提供商条目（或 config 中还没有对应 key）。"
}

// buildPageFragments renders the dynamic sections as HTML fragments for the
// in-place refresh endpoint.
func buildPageFragments(page pageData) pageFragments {
	return pageFragments{
		ConfigError: page.ConfigError,
		RefreshedAt: page.RefreshedAt,
		TabsHTML:    renderTabs(page, false),
		GridHTML:    renderGrid(page),
		EmptyText:   emptyText(page),
		Page:        page.Page,
		PageSize:    page.PageSize,
		Total:       page.Total,
	}
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
		// fallback letter tile for unknown vendors (rune-safe for non-ASCII names)
		letter := "?"
		if runes := []rune(e.VendorName); len(runes) > 0 {
			letter = strings.ToUpper(string(runes[:1]))
		}
		iconHTML = "<span style=\"color:#fff;font-weight:800;font-size:16px\">" + html.EscapeString(letter) + "</span>"
	}
	state := "fresh"
	if e.Missing {
		state = "missing"
	} else if !e.Fresh {
		state = "stale"
	}
	b.WriteString("<div class=\"entry\" data-vendor=\"" + html.EscapeString(e.VendorID) +
		"\" data-entry-idx=\"" + strconv.Itoa(e.Idx) +
		"\" data-state=\"" + state + "\">\n")
	b.WriteString("<div class=\"entry-head\"><div class=\"entry-title\">")
	b.WriteString("<span class=\"entry-icon\" style=\"background:" + icBg + "\">" + iconHTML + "</span>")
	b.WriteString("<div><div class=\"entry-name\">" + html.EscapeString(e.VendorName) + "</div>")
	b.WriteString("<div class=\"entry-meta\">")
	for _, t := range e.ProviderTypes {
		if ic := providerTypeIcon(t); ic != "" {
			b.WriteString(`<span class="badge type" title="` + html.EscapeString(t) + `">` + ic + `</span>`)
		} else {
			b.WriteString(`<span class="badge type fallback">` + html.EscapeString(t) + `</span>`)
		}
	}
	if e.Missing {
		b.WriteString("<span class=\"badge stale\">● 待加载</span>")
	} else if e.Fresh {
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
	if e.Missing {
		// Skeleton body: the card header is real, the quota box shimmers in once
		// the client lazily fetches this single entry (?entry-idx=N).
		b.WriteString("<div class=\"sk-body\">")
		b.WriteString("<div class=\"sk sk-line\"></div><div class=\"sk sk-line w70\"></div>")
		b.WriteString("</div>")
	} else if e.Err != "" {
		b.WriteString("<div class=\"errbox\"><span class=\"ic\">⚠</span><span>" + html.EscapeString(e.Err) + "</span></div>")
	} else {
		switch {
		case e.Windows != nil:
			for _, key := range []string{"rolling", "weekly", "monthly"} {
				w, ok := e.Windows[key]
				if !ok {
					continue // vendor did not report this window
				}
				renderWindow(b, key, w)
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
		case len(e.Models) > 0:
			for _, m := range e.Models {
				renderPlanModels(b, m)
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

// renderPlanModels renders one MiniMax coding-plan model with its two windows
// (current interval + current week). The payload's *_remaining_percent is the
// remaining fraction, so what is left drives both the label and the bar width.
func renderPlanModels(b *strings.Builder, m modelPlan) {
	intervalUsed := 100 - m.IntervalPercent
	weeklyUsed := 100 - m.WeeklyPercent

	b.WriteString("<div class=\"plan\"><div class=\"pname\">" + html.EscapeString(m.Name) + "</div>")

	b.WriteString("<div class=\"win\"><div class=\"whead\"><span>当前周期</span>")
	b.WriteString("<span><b>剩余 " + strconv.Itoa(percentInt(m.IntervalPercent)) + "%</b> · 已用 " + strconv.Itoa(percentInt(intervalUsed)) + "%")
	if m.IntervalStatus == 0 {
		b.WriteString(" · 暂停")
	}
	b.WriteString("</span></div>\n")
	b.WriteString("<div class=\"bar\"><i class=\"" + toneClass(intervalUsed) + "\" style=\"width:" + strconv.Itoa(percentInt(m.IntervalPercent)) + "%\"></i></div>\n")
	b.WriteString("<div class=\"reset\">剩余时长：" + html.EscapeString(fmtDuration(m.IntervalRemain)) + usageCount(m.IntervalUsed, m.IntervalTotal) + "</div>\n")
	b.WriteString("</div>\n")

	b.WriteString("<div class=\"win\"><div class=\"whead\"><span>本周</span>")
	b.WriteString("<span><b>剩余 " + strconv.Itoa(percentInt(m.WeeklyPercent)) + "%</b> · 已用 " + strconv.Itoa(percentInt(weeklyUsed)) + "%")
	if m.WeeklyStatus == 0 {
		b.WriteString(" · 暂停")
	}
	b.WriteString("</span></div>\n")
	b.WriteString("<div class=\"bar\"><i class=\"" + toneClass(weeklyUsed) + "\" style=\"width:" + strconv.Itoa(percentInt(m.WeeklyPercent)) + "%\"></i></div>\n")
	b.WriteString("<div class=\"reset\">剩余时长：" + html.EscapeString(fmtDuration(m.WeeklyRemain)) + usageCount(m.WeeklyUsed, m.WeeklyTotal) + "</div>\n")
	b.WriteString("</div></div>\n")
}

func usageCount(used, total int64) string {
	if total <= 0 {
		return ""
	}
	return " · 已用 " + strconv.Itoa(int(used)) + "/" + strconv.Itoa(int(total)) + " 次"
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
  var grid = document.getElementById('grid');
  var tabsBox = document.getElementById('tabs');
  var statsBox = document.getElementById('stats');
  var configErr = document.getElementById('configErr');
  var emptyWrap = document.getElementById('emptyWrap');
  var refreshBtn = document.getElementById('refresh');
  var refreshAt = document.getElementById('refreshAt');
  var pager = document.getElementById('pager');
  var currentTab = '';
  var page = 1, pageSize = 20, total = 0;
  var busy = false;
  var pageEmptyText = '';

  try { currentTab = new URLSearchParams(window.location.search).get('v') || ''; } catch(_) {}
  page = parseInt(document.body.getAttribute('data-page')||'1',10) || 1;
  pageSize = parseInt(document.body.getAttribute('data-page-size')||'20',10) || 20;
  total = parseInt(document.body.getAttribute('data-total')||'0',10) || 0;

  function stat(label, value, cls){
    var d=document.createElement('div');d.className='stat';
    d.innerHTML='<div class="label">'+label+'</div><div class="value '+(cls||'')+'">'+value+'</div>';
    return d;
  }
  function renderStats(){
    if(!statsBox) return;
    statsBox.innerHTML='';
    var cards = document.querySelectorAll('.entry');
    var n = cards.length, ok = 0, err = 0, fresh = 0, pending = 0;
    cards.forEach(function(c){
      if(c.getAttribute('data-state')==='missing'){ pending++; return; }
      if(c.querySelector('.errbox') || c.querySelector('.badge.err')) err++;
      else ok++;
      if(c.querySelector('.badge.fresh')) fresh++;
    });
    statsBox.append(
      stat('本页条目', n),
      stat('本页可用', ok, 'green'),
      stat('本页异常', err, err>0?'red':'green'),
      stat('本页实时', fresh+'/'+n, fresh>0?'green':'')
    );
    if(pending>0) statsBox.append(stat('待加载', pending, ''));
  }
  function setRefreshAt(sec){
    if(!refreshAt || !sec) return;
    try {
      var d = new Date(sec*1000);
      refreshAt.textContent = '上次 ' + d.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'});
    } catch(_){ refreshAt.textContent=''; }
  }
  function setBtn(loading){
    if(!refreshBtn) return;
    refreshBtn.classList.toggle('loading', loading);
    refreshBtn.disabled = loading;
    var lbl = refreshBtn.querySelector('.lbl');
    if(lbl) lbl.textContent = loading ? '刷新中…' : '刷新';
  }
  function applyTab(id){
    id = id || '';
    document.querySelectorAll('[data-tab]').forEach(function(t){
      t.classList.toggle('active', t.getAttribute('data-tab')===id);
    });
    var cards = grid ? grid.querySelectorAll('.entry') : [];
    var visible = 0;
    cards.forEach(function(c){
      var on = !id || c.getAttribute('data-vendor')===id;
      c.classList.toggle('on', on);
      if(on) visible++;
    });
    if(emptyWrap){
      if(cards.length === 0){ emptyWrap.textContent = pageEmptyText; }
      else if(id && visible === 0){ emptyWrap.textContent = '该厂商暂无条目'; }
      else { emptyWrap.textContent = ''; }
    }
  }
  function renderPager(){
    if(!pager) return;
    var pages = total ? Math.max(1, Math.ceil(total/pageSize)) : 0;
    if(pages <= 1){ pager.innerHTML=''; pager.classList.remove('on'); return; }
    pager.classList.add('on');
    var i, html = '<button class="pg" data-page="'+(page-1)+'"'+(page<=1?' disabled':'')+'>上一页</button>';
    var nums = [];
    if(pages <= 7){
      for(i=1;i<=pages;i++) nums.push(i);
    } else {
      nums.push(1);
      var lo = Math.max(2, page-1), hi = Math.min(pages-1, page+1);
      if(lo > 2) nums.push('gap');
      for(i=lo;i<=hi;i++) nums.push(i);
      if(hi < pages-1) nums.push('gap');
      nums.push(pages);
    }
    nums.forEach(function(n){
      if(n === 'gap') html += '<span class="pg gap">…</span>';
      else html += '<button class="pg'+(n===page?' active':'')+'" data-page="'+n+'"'+(n===page?' disabled':'')+'>'+n+'</button>';
    });
    html += '<button class="pg" data-page="'+(page+1)+'"'+(page>=pages?' disabled':'')+'>下一页</button>';
    pager.innerHTML = html;
  }
  function showSkeleton(label){
    if(!grid) return;
    var cards = grid.querySelectorAll('.entry.on');
    if(cards.length === 0){
      grid.innerHTML = '';
      if(emptyWrap) emptyWrap.textContent = label || '正在加载…';
      return;
    }
    var s = '';
    cards.forEach(function(){
      s += '<div class="entry on sk-entry">'
         + '<div class="sk-row"><span class="sk sk-avatar"></span><span class="sk sk-title"></span><span class="sk sk-key"></span></div>'
         + '<div class="sk sk-line"></div><div class="sk sk-line w70"></div></div>';
    });
    grid.innerHTML = s;
  }
  function renderFragments(data){
    if(!data) return;
    pageEmptyText = data.emptyText || '';
    if(typeof data.page === 'number') page = data.page;
    if(typeof data.pageSize === 'number') pageSize = data.pageSize;
    if(typeof data.total === 'number') total = data.total;
    if(tabsBox){
      tabsBox.innerHTML = data.tabsHTML || '';
      bindTabs();
      if(currentTab){
        var tabGone = true;
        tabsBox.querySelectorAll('[data-tab]').forEach(function(t){
          if(t.getAttribute('data-tab')===currentTab) tabGone = false;
        });
        if(tabGone) currentTab = '';
      }
    }
    if(configErr){
      configErr.classList.toggle('on', !!data.configError);
      configErr.textContent = data.configError ? ('配置读取失败：'+data.configError) : '';
    }
    if(grid) grid.innerHTML = data.gridHTML || '';
    renderStats();
    renderPager();
    setRefreshAt(data.refreshedAt);
    applyTab(currentTab);
    collectLazyEntries();
  }
  function restore(backup){
    currentTab = backup.tab;
    page = backup.page;
    if(tabsBox){ tabsBox.innerHTML = backup.tabs; bindTabs(); }
    if(configErr){ configErr.classList.toggle('on', backup.errOn); configErr.textContent = backup.errText; }
    if(grid) grid.innerHTML = backup.grid;
    if(emptyWrap) emptyWrap.textContent = backup.empty;
    renderStats();
    renderPager();
    applyTab(currentTab);
  }
  function syncURL(targetPage){
    try {
      var u = new URL(window.location.href);
      if(currentTab) u.searchParams.set('v', currentTab); else u.searchParams.delete('v');
      u.searchParams.set('p', String(targetPage));
      u.searchParams.delete('refresh');
      u.searchParams.delete('partial');
      history.replaceState(null,'',u.toString());
    } catch(_) {}
  }
  function buildTarget(targetPage, force){
    var target = window.location.pathname + '?partial=1&page=' + encodeURIComponent(String(targetPage)) + '&page-size=' + encodeURIComponent(String(pageSize));
    if(currentTab) target += '&vendor=' + encodeURIComponent(currentTab);
    if(force) target += '&refresh=1';
    try {
      var u = new URL(window.location.href);
      u.searchParams.set('partial','1');
      u.searchParams.set('page', String(targetPage));
      u.searchParams.set('page-size', String(pageSize));
      if(currentTab) u.searchParams.set('vendor', currentTab); else u.searchParams.delete('vendor');
      if(force) u.searchParams.set('refresh','1'); else u.searchParams.delete('refresh');
      target = u.toString();
    } catch(_) {}
    return target;
  }
  function loadPage(targetPage, force){
    if(busy) return;
    busy = true;
    setBtn(true);
    if(tabsBox) tabsBox.classList.add('busy');
    if(pager) pager.classList.add('busy');
    var backup = {
      tabs: tabsBox ? tabsBox.innerHTML : '',
      grid: grid ? grid.innerHTML : '',
      empty: emptyWrap ? emptyWrap.textContent : '',
      errOn: configErr ? configErr.classList.contains('on') : false,
      errText: configErr ? configErr.textContent : '',
      tab: currentTab,
      page: page
    };
    showSkeleton(force ? '正在刷新…' : '正在加载…');
    var ctl = (typeof AbortController !== 'undefined') ? new AbortController() : null;
    var timer = ctl ? setTimeout(function(){ ctl.abort(); }, 120000) : null;
    function finish(){
      busy = false;
      setBtn(false);
      if(tabsBox) tabsBox.classList.remove('busy');
      if(pager) pager.classList.remove('busy');
    }
    fetch(buildTarget(targetPage, force), {
      method:'GET',
      headers:{'Accept':'application/json'},
      credentials:'same-origin',
      signal: ctl ? ctl.signal : undefined
    }).then(function(r){
      if(!r.ok) throw new Error('HTTP '+r.status);
      return r.json();
    }).then(function(data){
      if(timer) clearTimeout(timer);
      renderFragments(data);
      syncURL(typeof data.page === 'number' ? data.page : targetPage);
      finish();
    }).catch(function(err){
      if(timer) clearTimeout(timer);
      restore(backup);
      finish();
      if(configErr){
        var msg = (err && err.name === 'AbortError') ? '请求超时（120 秒）' : (err && err.message ? err.message : String(err));
        configErr.classList.add('on');
        configErr.textContent = '加载失败：' + msg + ' · 已恢复此前内容';
      }
    });
  }

  /* ===== 渐进懒加载：打开面板瞬间返回壳页，卡片逐条填充 =====
   * 服务端对首次打开/翻页不再同步查询当页全部额度，缺失或过期的卡
   * 通过 ?entry-idx=N 单独拉取，互不阻塞。并发上限 6（浏览器同源限制）。
   * 请求在统一的 page 视图内按卡片下标寻址，key 附带 page/tab 防止切换
   * 视图后旧响应错落到别的卡片上。 */
  var ENTRY_CONCURRENCY = 6;
  var entryQueue = [];
  var entryInflight = {};
  var entryRetry = {};
  function entryKey(idx){ return idx + '\u0000' + page + '\u0000' + currentTab; }
  function collectLazyEntries(){
    if(!grid) return;
    entryQueue = [];
    grid.querySelectorAll('.entry').forEach(function(c){
      var idx = c.getAttribute('data-entry-idx');
      var st = c.getAttribute('data-state');
      if(idx === null || idx === '' || st === 'fresh' || st === 'err') return;
      var k = entryKey(idx);
      if(entryInflight[k] || entryQueue.indexOf(k) !== -1) return;
      entryQueue.push(k);
    });
    pumpEntryLoaders();
  }
  function pumpEntryLoaders(){
    var active = 0;
    for(var k in entryInflight) if(entryInflight[k]) active++;
    while(active < ENTRY_CONCURRENCY && entryQueue.length){
      var k = entryQueue.shift();
      if(entryInflight[k]) continue;
      entryInflight[k] = true;
      active++;
      fetchEntry(k, parseInt(k.split('\u0000')[0],10));
    }
  }
  function entryTarget(idx){
    // Not forced: the server decides via cache TTL whether to hit upstream,
    // so repeated opens within the TTL return instantly from cache.
    var target = buildTarget(page, false);
    try {
      var u = new URL(target);
      u.searchParams.set('entry-idx', String(idx));
      return u.toString();
    } catch(_) {
      return target + '&entry-idx=' + encodeURIComponent(String(idx));
    }
  }
  function swapEntry(idx, html, key){
    if(!grid || !html) return;
    if(entryKey(idx) !== key) return; // 请求发出后翻页/切 tab，旧结果作废
    var found = null;
    grid.querySelectorAll('.entry').forEach(function(c){
      if(c.getAttribute('data-entry-idx') === String(idx)) found = c;
    });
    if(!found) return; // 卡片已被移除
    var div = document.createElement('div');
    div.innerHTML = html;
    var nc = div.firstElementChild;
    if(!nc) return;
    if(!currentTab || nc.getAttribute('data-vendor') === currentTab) nc.classList.add('on');
    found.replaceWith(nc);
    renderStats();
  }
  function fetchEntry(k, idx){
    var ctl = (typeof AbortController !== 'undefined') ? new AbortController() : null;
    var timer = ctl ? setTimeout(function(){ ctl.abort(); }, 30000) : null;
    fetch(entryTarget(idx), {
      method:'GET',
      headers:{'Accept':'application/json'},
      credentials:'same-origin',
      signal: ctl ? ctl.signal : undefined
    }).then(function(r){
      if(!r.ok) throw new Error('HTTP '+r.status);
      return r.json();
    }).then(function(data){
      if(timer) clearTimeout(timer);
      delete entryInflight[k];
      delete entryRetry[k];
      if(data && typeof data.entryIdx === 'number' && data.entryHTML){
        swapEntry(data.entryIdx, data.entryHTML, k);
      }
      pumpEntryLoaders();
    }).catch(function(){
      if(timer) clearTimeout(timer);
      delete entryInflight[k];
      var tries = (entryRetry[k] || 0) + 1;
      if(tries <= 2){ // 有界重试：最多补两张，避免单卡失败留白
        entryRetry[k] = tries;
        entryQueue.push(k);
        setTimeout(pumpEntryLoaders, 1200);
      }
      pumpEntryLoaders();
    });
  }
  function onTab(e){
    if(busy) return;
    var id = e.currentTarget.getAttribute('data-tab');
    if(id === currentTab) return;
    currentTab = id;
    loadPage(1, false);
  }
  function bindTabs(){
    if(!tabsBox) return;
    tabsBox.querySelectorAll('[data-tab]').forEach(function(t){
      t.addEventListener('click', onTab);
    });
  }

  if(refreshBtn) refreshBtn.addEventListener('click', function(){ loadPage(page, true); });
  if(pager) pager.addEventListener('click', function(e){
    if(busy) return;
    var b = e.target.closest('.pg');
    if(!b || b.disabled) return;
    var n = parseInt(b.getAttribute('data-page'),10);
    if(!isNaN(n) && n !== page) loadPage(n, false);
  });

  renderStats();
  bindTabs();
  pageEmptyText = emptyWrap ? emptyWrap.textContent : '';
  applyTab(currentTab);
  renderPager();
  collectLazyEntries();
  var at = parseInt(document.body.getAttribute('data-refreshed-at')||'0',10);
  setRefreshAt(at);

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
