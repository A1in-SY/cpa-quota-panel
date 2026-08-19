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
    for needle in ["额度面板", "全部", "OpenCode", "DeepSeek", "sk******ef", "sk******56", "codex", "claude"]:
        assert needle in html, f"dashboard missing {needle!r}"
    print("dashboard OK: contains 全部/OpenCode/DeepSeek/masked keys/type tags")

    os.unlink(host_cfg.name)
    print("ALL PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
