"""合约交互面板服务,根据 ABI 描述生成方法表单和调用载荷。"""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_abi import encode
from eth_utils import function_signature_to_4byte_selector, to_hex

PORT = int(os.environ.get("CHAIMIR_IMAGE_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "1048576"))
INDEX_HTML = """<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>Chaimir Contract UI</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;background:#f7faf9;color:#10201e}
    main{max-width:960px;margin:0 auto;padding:24px}
    textarea,input,button{font:inherit}
    textarea,input{width:100%;box-sizing:border-box;border:1px solid #b9c8c4;border-radius:6px;padding:10px;background:#fff}
    button{border:0;border-radius:6px;background:#007c78;color:#fff;padding:10px 14px;cursor:pointer}
    pre{white-space:pre-wrap;background:#101918;color:#d9fffb;border-radius:6px;padding:14px;overflow:auto}
    .grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
    @media(max-width:720px){.grid{grid-template-columns:1fr}}
  </style>
</head>
<body>
<main>
  <h1>合约交互面板</h1>
  <div class="grid">
    <section>
      <label>ABI JSON</label>
      <textarea id="abi" rows="18">[]</textarea>
    </section>
    <section>
      <label>调用参数 JSON</label>
      <textarea id="args" rows="8">{"method":"","args":[]}</textarea>
      <button id="build">生成调用数据</button>
      <pre id="output"></pre>
    </section>
  </div>
</main>
<script>
document.getElementById('build').onclick=async()=>{
  const abi=JSON.parse(document.getElementById('abi').value);
  const payload=JSON.parse(document.getElementById('args').value);
  payload.abi=abi;
  const res=await fetch('/call-data',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});
  document.getElementById('output').textContent=JSON.stringify(await res.json(),null,2);
};
</script>
</body>
</html>
"""


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict[str, Any]:
    """读取并校验 JSON 请求体。"""
    length = int(handler.headers.get("Content-Length", "0"))
    if length > MAX_BODY_BYTES:
        raise ValueError("payload too large")
    body = handler.rfile.read(length)
    payload = json.loads(body.decode("utf-8") or "{}")
    if not isinstance(payload, dict):
        raise ValueError("body must be a json object")
    return payload


def abi_methods(abi: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """提取 ABI 中可调用函数的表单描述。"""
    methods = []
    for item in abi:
        if item.get("type") != "function" or not item.get("name"):
            continue
        inputs = item.get("inputs") or []
        methods.append(
            {
                "name": item["name"],
                "stateMutability": item.get("stateMutability", "nonpayable"),
                "inputs": [{"name": field.get("name", ""), "type": field.get("type", "")} for field in inputs],
            }
        )
    return methods


def build_call_data(abi: list[dict[str, Any]], method_name: str, args: list[Any]) -> dict[str, Any]:
    """根据 ABI 方法和参数生成 EVM calldata。"""
    candidates = [item for item in abi if item.get("type") == "function" and item.get("name") == method_name]
    if len(candidates) != 1:
        raise ValueError("method must match exactly one ABI function")
    method = candidates[0]
    inputs = method.get("inputs") or []
    types = [field["type"] for field in inputs]
    if len(args) != len(types):
        raise ValueError("argument count mismatch")
    signature = f"{method_name}({','.join(types)})"
    selector = function_signature_to_4byte_selector(signature)
    encoded_args = encode(types, args)
    return {
        "method": method_name,
        "signature": signature,
        "calldata": to_hex(selector + encoded_args),
        "stateMutability": method.get("stateMutability", "nonpayable"),
    }


class ContractUIHandler(BaseHTTPRequestHandler):
    """处理合约 UI 请求。"""

    def do_GET(self) -> None:
        """返回健康检查和默认面板信息。"""
        if self.path == "/healthz":
            self.write_json({"status": "ok"})
            return
        if self.path in {"/", "/index.html"}:
            self.write_html(INDEX_HTML)
            return
        if self.path == "/schema":
            self.write_json({"inputs": ["abi", "method", "args"], "mode": "evm-calldata"})
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:
        """解析 ABI JSON 并返回方法表单或调用载荷。"""
        if self.path not in {"/abi/methods", "/call-data"}:
            self.send_error(404, "not found")
            return
        try:
            payload = parse_json_body(self)
        except json.JSONDecodeError:
            self.send_error(400, "invalid json")
            return
        except ValueError as exc:
            self.send_error(413 if str(exc) == "payload too large" else 400, str(exc))
            return
        abi = payload.get("abi", [])
        if not isinstance(abi, list) or not all(isinstance(item, dict) for item in abi):
            self.send_error(400, "abi must be an array")
            return
        if self.path == "/abi/methods":
            self.write_json({"methods": abi_methods(abi)})
            return
        try:
            result = build_call_data(abi, str(payload.get("method", "")), list(payload.get("args", [])))
        except (TypeError, ValueError) as exc:
            self.send_error(400, str(exc))
            return
        self.write_json(result)

    def log_message(self, format: str, *args: object) -> None:
        """关闭默认访问日志。"""
        return

    def write_json(self, payload: dict[str, object]) -> None:
        """写出 JSON 响应。"""
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def write_html(self, html: str) -> None:
        """写出内置合约交互页面。"""
        body = html.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", PORT), ContractUIHandler).serve_forever()
