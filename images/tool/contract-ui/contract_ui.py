"""合约交互面板服务,根据 ABI 描述生成方法表单和调用载荷。"""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_abi import encode
from eth_abi.exceptions import EncodingError, ParseError
from eth_abi.grammar import parse as parse_abi_type
from eth_utils import function_signature_to_4byte_selector, to_hex

def required_positive_int(name: str) -> int:
    """读取必填正整数配置,拒绝镜像自行选择运行参数。"""
    raw = os.environ.get(name, "").strip()
    if not raw:
        raise RuntimeError(f"{name} is required")
    try:
        value = int(raw, 10)
    except ValueError as exc:
        raise RuntimeError(f"{name} must be a positive integer") from exc
    if value <= 0:
        raise RuntimeError(f"{name} must be a positive integer")
    return value


# 运行参数在真正启动 HTTP 服务时读取,使纯函数自检可以在无服务配置的环境中执行。
PORT = 0
MAX_BODY_BYTES = 0
HTTP_TIMEOUT_SECONDS = 0
MAX_HEADER_BYTES = 16 * 1024
MAX_HEADER_COUNT = 64
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
    if handler.headers.get("Transfer-Encoding"):
        raise ValueError("transfer encoding is unsupported")
    content_lengths = handler.headers.get_all("Content-Length", [])
    if len(content_lengths) != 1:
        raise ValueError("content-length must be declared exactly once")
    raw_length = content_lengths[0].strip()
    if not raw_length or not raw_length.isdecimal():
        raise ValueError("content-length is invalid")
    try:
        length = int(raw_length, 10)
    except ValueError as exc:
        raise ValueError("content-length is invalid") from exc
    if length < 0:
        raise ValueError("content-length is invalid")
    if length > MAX_BODY_BYTES:
        raise ValueError("payload too large")
    body = handler.rfile.read(length)
    if len(body) != length:
        raise ValueError("incomplete body")
    try:
        payload = json.loads(body.decode("utf-8") or "{}")
    except UnicodeDecodeError as exc:
        raise ValueError("body must be UTF-8 JSON") from exc
    if not isinstance(payload, dict):
        raise ValueError("body must be a json object")
    return payload


def abi_methods(abi: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """提取 ABI 中可调用函数的表单描述。"""
    methods = []
    for item in abi:
        if not isinstance(item, dict):
            raise ValueError("abi entries must be objects")
        if item.get("type") != "function":
            continue
        name = item.get("name")
        inputs = item.get("inputs") or []
        if not isinstance(name, str) or not name or not isinstance(inputs, list):
            raise ValueError("abi function declaration is invalid")
        for field in inputs:
            if not isinstance(field, dict) or not isinstance(field.get("type"), str) or not field["type"]:
                raise ValueError("abi function input declaration is invalid")
            validate_abi_type(field["type"])
        methods.append(
            {
                "name": item["name"],
                "stateMutability": item.get("stateMutability", "nonpayable"),
                "inputs": [{"name": field.get("name", ""), "type": field.get("type", "")} for field in inputs],
            }
        )
    return methods


def validate_abi_type(type_name: str) -> None:
    """校验 ABI 类型确实是 eth-abi 支持的 Solidity 基础类型。"""
    if type_name.startswith("tuple"):
        raise ValueError("tuple ABI types are unsupported")
    try:
        parse_abi_type(type_name)
    except ParseError as exc:
        raise ValueError("abi type is invalid") from exc


def build_call_data(abi: list[dict[str, Any]], method_name: str, args: list[Any]) -> dict[str, Any]:
    """根据 ABI 方法和参数生成 EVM calldata。"""
    if not isinstance(method_name, str) or not method_name:
        raise ValueError("method is required")
    if not isinstance(args, list):
        raise ValueError("args must be an array")
    candidates = [item for item in abi if item.get("type") == "function" and item.get("name") == method_name]
    if len(candidates) != 1:
        raise ValueError("method must match exactly one ABI function")
    method = candidates[0]
    inputs = method.get("inputs") or []
    if not isinstance(inputs, list) or not all(isinstance(field, dict) and isinstance(field.get("type"), str) and field["type"] for field in inputs):
        raise ValueError("abi function input declaration is invalid")
    types = [field["type"] for field in inputs]
    for type_name in types:
        validate_abi_type(type_name)
    if len(args) != len(types):
        raise ValueError("argument count mismatch")
    signature = f"{method_name}({','.join(types)})"
    try:
        selector = function_signature_to_4byte_selector(signature)
        encoded_args = encode(types, args)
    except (EncodingError, TypeError, ValueError, OverflowError) as exc:
        raise ValueError("arguments do not match ABI") from exc
    return {
        "method": method_name,
        "signature": signature,
        "calldata": to_hex(selector + encoded_args),
        "stateMutability": method.get("stateMutability", "nonpayable"),
    }


class ContractUIHandler(BaseHTTPRequestHandler):
    """处理合约 UI 请求。"""

    def parse_request(self) -> bool:
        """拒绝过多或过大的请求头,避免连接长期占用工作线程。"""
        if not super().parse_request():
            return False
        header_bytes = sum(len(key) + len(value) + 4 for key, value in self.headers.items())
        if len(self.headers) > MAX_HEADER_COUNT or header_bytes > MAX_HEADER_BYTES:
            self.send_error(431, "request headers too large")
            self.close_connection = True
            return False
        return True

    def send_error(self, code: int, message: str | None = None, explain: str | None = None) -> None:
        """以结构化 JSON 返回协议和输入错误,避免默认 HTML 错误页泄漏实现细节。"""
        self.close_connection = True
        self.write_json({"status": "error", "error": message or "request failed"}, status=code)

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
        self.close_connection = True
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
        try:
            methods = abi_methods(abi)
            if self.path == "/abi/methods":
                self.write_json({"methods": methods})
                return
            result = build_call_data(abi, payload.get("method", ""), payload.get("args", []))
        except (TypeError, ValueError, OverflowError, KeyError, ParseError) as exc:
            self.send_error(400, str(exc))
            return
        self.write_json(result)

    def log_message(self, format: str, *args: object) -> None:
        """关闭默认访问日志。"""
        return

    def write_json(self, payload: dict[str, object], status: int = 200) -> None:
        """写出 JSON 响应。"""
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        if self.close_connection:
            self.send_header("Connection", "close")
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


class BoundedThreadingHTTPServer(ThreadingHTTPServer):
    """为工具 HTTP 端口设置有限连接超时和请求头上限。"""

    daemon_threads = True
    allow_reuse_address = True

    def get_request(self):
        request, client_address = super().get_request()
        request.settimeout(HTTP_TIMEOUT_SECONDS)
        return request, client_address


def run_server() -> None:
    """读取运行配置并启动合约交互服务。"""
    global PORT, MAX_BODY_BYTES, HTTP_TIMEOUT_SECONDS
    PORT = required_positive_int("CHAIMIR_IMAGE_PORT")
    MAX_BODY_BYTES = required_positive_int("CHAIMIR_MAX_BODY_BYTES")
    HTTP_TIMEOUT_SECONDS = required_positive_int("CHAIMIR_HTTP_TIMEOUT_SECONDS")
    BoundedThreadingHTTPServer(("0.0.0.0", PORT), ContractUIHandler).serve_forever()


if __name__ == "__main__":
    run_server()
