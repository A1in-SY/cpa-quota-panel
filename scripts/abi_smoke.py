#!/usr/bin/env python3
"""Standalone C ABI smoke test for the cpa-quota-panel plugin (no CPA needed).

Usage:  python3 scripts/abi_smoke.py [path/to/cpa-quota-panel.dylib]

Verifies:
  * cliproxy_plugin_init fills the plugin API table.
  * plugin.register succeeds and declares only the management_api capability.
  * plugin.reconfigure accepts config_yaml (base64 by ABI) pointing at a host config.
  * management.handle renders the dashboard with vendor tabs and scanned entries.
"""
import base64
import ctypes
import json
import os
import sys
import tempfile

LIB = sys.argv[1] if len(sys.argv) > 1 else "plugins/darwin/arm64/cpa-quota-panel.dylib"

HOST_CONFIG = """
codex-api-key:
  - api-key: "oc-real-key-abcdef"
    base-url: "https://opencode.ai/zen/go/v1"
  - api-key: "ds-real-key-123456"
    base-url: "https://api.deepseek.com"
claude-api-key:
  - api-key: "oc-real-key-abcdef"      # same key under claude too -> merged, type tags
    base-url: "https://opencode.ai/zen/go/v1"
"""


class Buffer(ctypes.Structure):
    _fields_ = [("ptr", ctypes.c_void_p), ("len", ctypes.c_size_t)]


PLUGIN_CALL = ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_char_p, ctypes.POINTER(ctypes.c_uint8),
                               ctypes.c_size_t, ctypes.POINTER(Buffer))
PLUGIN_FREE = ctypes.CFUNCTYPE(ctypes.c_void_p, ctypes.c_void_p, ctypes.c_size_t)
PLUGIN_SHUTDOWN = ctypes.CFUNCTYPE(ctypes.c_void_p)


class PluginAPI(ctypes.Structure):
    _fields_ = [("abi_version", ctypes.c_uint32),
                ("call", PLUGIN_CALL),
                ("free_buffer", PLUGIN_FREE),
                ("shutdown", PLUGIN_SHUTDOWN)]


class HostAPI(ctypes.Structure):
    _fields_ = [("abi_version", ctypes.c_uint32),
                ("host_ctx", ctypes.c_void_p),
                ("call", ctypes.c_void_p),
                ("free_buffer", ctypes.c_void_p)]


def body_str(value):
    if isinstance(value, str):
        return base64.b64decode(value).decode(errors="replace")
    return "".join(chr(b) for b in value)


def main() -> int:
    host_cfg = tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False)
    host_cfg.write(HOST_CONFIG)
    host_cfg.close()

    lib = ctypes.CDLL(LIB)
    lib.cliproxy_plugin_init.restype = ctypes.c_int
    lib.cliproxy_plugin_init.argtypes = [ctypes.POINTER(HostAPI), ctypes.POINTER(PluginAPI)]
    plugin = PluginAPI()
    if lib.cliproxy_plugin_init(None, ctypes.byref(plugin)) != 0 or not plugin.call:
        print("FATAL: init failed", file=sys.stderr)
        return 1

    def call(method, payload=b""):
        req, req_len = None, 0
        if payload:
            buf = ctypes.create_string_buffer(payload)
            req = ctypes.cast(buf, ctypes.POINTER(ctypes.c_uint8))
            req_len = len(payload)
        resp = Buffer()
        rc = plugin.call(method.encode(), req, req_len, ctypes.byref(resp))
        raw = b""
        if resp.ptr and resp.len:
            raw = ctypes.string_at(resp.ptr, resp.len)
            plugin.free_buffer(ctypes.c_void_p(resp.ptr), resp.len)
        return rc, json.loads(raw.decode() if raw else '{"ok":false}')

    rc, env = call("plugin.register")
    assert rc == 0 and env["ok"], f"register failed: {env}"
    caps = env["result"]["capabilities"]
    assert caps == {"management_api": True}, f"unexpected capabilities: {caps}"
    fields = [f["Name"] for f in env["result"]["metadata"].get("ConfigFields", [])]
    assert fields == ["config-path", "cache-ttl-seconds", "quota-sources"], f"fields: {fields}"
    print("register OK caps:", caps)

    cfg_yaml = ("config-path: %s\ncache-ttl-seconds: 300\n" % host_cfg.name).encode()
    rc, env = call("plugin.reconfigure", json.dumps({"config_yaml": base64.b64encode(cfg_yaml).decode()}).encode())
    assert rc == 0 and env["ok"], f"reconfigure failed: {env}"
    print("reconfigure OK")

    mreq = {"Method": "GET", "Path": "/v0/resource/plugins/cpa-quota-panel/status",
            "Headers": {}, "Query": {}, "Body": ""}
    rc, env = call("management.handle", json.dumps(mreq).encode())
    assert rc == 0 and env["ok"], f"dashboard failed: {env}"
    html = body_str(env["result"]["Body"])
    for needle in ["额度面板", "全部", "OpenCode", "Deepseek API", "智谱CodingPlan", "sk******ef", "sk******56", "codex", "claude"]:
        assert needle in html, f"dashboard missing {needle!r}"
    print("dashboard OK: contains 全部/OpenCode/Deepseek API/智谱/masked keys/type tags")

    # Opening must NOT block on quota: without cache the entries render as
    # "missing" skeleton cards (data-state=missing + sk-body), no sync refresh.
    assert 'data-state="missing"' in html and "sk-body" in html, \
        "opening page should render missing skeleton cards, got no sk-body"
    print("open-OK: opening page returns instantly with missing skeleton cards")

    # In-place refresh endpoint (?partial=1) powering the skeleton-screen flow.
    mreq = {"Method": "GET", "Path": "/v0/resource/plugins/cpa-quota-panel/status",
            "Headers": {}, "Query": {"refresh": ["1"], "partial": ["1"]}, "Body": ""}
    rc, env = call("management.handle", json.dumps(mreq).encode())
    assert rc == 0 and env["ok"], f"partial refresh failed: {env}"
    data = json.loads(body_str(env["result"]["Body"]))
    assert data.get("tabsHTML") and data.get("gridHTML"), f"partial fragments missing: {data}"
    assert isinstance(data.get("refreshedAt"), int) and data["refreshedAt"] > 0, f"bad refreshedAt: {data}"
    assert data.get("page") == 1 and data.get("pageSize") == 20 and data.get("total") == 2, f"bad paging: {data}"
    assert "sk******ef" in data["gridHTML"], "partial gridHTML missing masked key"
    print("partial OK: tabsHTML/gridHTML/refreshedAt/paging present")

    # Single-entry lazy endpoint: ?entry-idx=N refreshes only that card and
    # returns its standalone HTML (no full-page blocking).
    mreq = {"Method": "GET", "Path": "/v0/resource/plugins/cpa-quota-panel/status",
            "Headers": {}, "Query": {"refresh": ["1"], "partial": ["1"], "entry-idx": ["1"]}, "Body": ""}
    rc, env = call("management.handle", json.dumps(mreq).encode())
    assert rc == 0 and env["ok"], f"entry lazy failed: {env}"
    lazy = json.loads(body_str(env["result"]["Body"]))
    assert lazy.get("entryIdx") == 1 and lazy.get("entryHTML"), f"lazy entry missing: {lazy}"
    assert 'data-entry-idx="1"' in lazy["entryHTML"], f"lazy card lacks entry-idx: {lazy['entryHTML']}"
    # view order is sorted by vendor id: deepseek(0) < opencode(1)
    assert "sk******56" not in lazy["entryHTML"] and "sk******ef" in lazy["entryHTML"], \
        f"lazy card should be only entry 1 (opencode): {lazy['entryHTML']}"
    print("entry-lazy OK: ?entry-idx=1 returns only that one card")

    # Server-side pagination: page-size=1 page=2 returns only the second entry.
    mreq = {"Method": "GET", "Path": "/v0/resource/plugins/cpa-quota-panel/status",
            "Headers": {}, "Query": {"partial": ["1"], "page": ["2"], "page-size": ["1"]}, "Body": ""}
    rc, env = call("management.handle", json.dumps(mreq).encode())
    assert rc == 0 and env["ok"], f"paginated partial failed: {env}"
    data2 = json.loads(body_str(env["result"]["Body"]))
    assert data2.get("page") == 2 and data2.get("total") == 2, f"bad page 2 data: {data2}"
    assert "sk******ef" in data2["gridHTML"] and "sk******56" not in data2["gridHTML"], \
        f"page 2 should only contain the opencode entry: {data2['gridHTML']}"
    print("pagination OK: page 2 of 2 contains only the second entry")

    os.unlink(host_cfg.name)
    print("ALL PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
