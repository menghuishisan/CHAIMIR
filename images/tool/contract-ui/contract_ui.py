"""合约交互面板服务,根据 ABI 描述生成方法表单和调用载荷。"""

from __future__ import annotations

import json
import os
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_abi import encode
from eth_abi.exceptions import EncodingError, ParseError
from eth_abi.grammar import ABIType, TupleType, normalize, parse as parse_abi_type
from eth_abi.registry import registry
from eth_utils import function_signature_to_4byte_selector, to_hex
from eth_utils.abi import collapse_if_tuple

def required_positive_int(name: str) -> int:
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
    textarea,select,button{font:inherit}
    textarea,select{width:100%;box-sizing:border-box;border:1px solid #b9c8c4;border-radius:6px;padding:10px;background:#fff}
    label{display:block;margin:12px 0 6px}
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
      <label for="abi">ABI JSON</label>
      <textarea id="abi" rows="18">[]</textarea>
      <button id="load">读取方法</button>
    </section>
    <section>
      <label for="method">合约方法</label>
      <select id="method"><option value="">先读取 ABI 方法</option></select>
      <label for="args">参数 JSON 数组</label>
      <textarea id="args" rows="8">[]</textarea>
      <p>大整数用字符串，二进制用 0x 十六进制，tuple 按字段顺序填写数组。</p>
      <button id="build">生成调用数据</button>
      <pre id="output" role="status" aria-live="polite"></pre>
    </section>
  </div>
</main>
<script>
const output = document.getElementById('output');
const method = document.getElementById('method');
async function requestJSON(path, payload) {
  const response = await fetch(path, {method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload)});
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || '请求失败');
  return result;
}
document.getElementById('load').onclick = async () => {
  try {
    const {methods} = await requestJSON('./abi/methods', {abi: JSON.parse(document.getElementById('abi').value)});
    method.replaceChildren(...methods.map(item => new Option(item.signature, item.signature)));
    output.textContent = methods.length ? '选择方法并填写参数。' : 'ABI 中没有可调用的方法。';
  } catch (error) { output.textContent = error.message; }
};
document.getElementById('build').onclick = async () => {
  try {
    if (!method.value) throw new Error('请先读取 ABI 并选择方法。');
    const result = await requestJSON('./call-data', {abi: JSON.parse(document.getElementById('abi').value), method: method.value, args: JSON.parse(document.getElementById('args').value)});
    output.textContent = JSON.stringify(result, null, 2);
  } catch (error) { output.textContent = error.message; }
};
</script>
</body>
</html>
"""


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict[str, Any]:
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
    if not isinstance(abi, list):
        raise ValueError("abi must be an array")
    methods = []
    for item in abi:
        if not isinstance(item, dict):
            raise ValueError("abi entries must be objects")
        if item.get("type") != "function":
            continue
        name = item.get("name")
        inputs = item.get("inputs", [])
        if not isinstance(name, str) or not re.fullmatch(r"[A-Za-z_$][A-Za-z0-9_$]*", name) or not isinstance(inputs, list):
            raise ValueError("abi function declaration is invalid")
        types = [abi_input_type(field) for field in inputs]
        mutability = item.get("stateMutability", "nonpayable")
        if mutability not in ("pure", "view", "nonpayable", "payable"):
            raise ValueError("abi stateMutability is invalid")
        methods.append(
            {
                "name": name,
                "signature": f"{name}({','.join(types)})",
                "stateMutability": mutability,
                "inputs": [{"name": field.get("name", ""), "type": type_name} for field, type_name in zip(inputs, types)],
            }
        )
    return methods


def abi_input_type(field: dict[str, Any]) -> str:
    if not isinstance(field, dict) or not isinstance(field.get("type"), str):
        raise ValueError("abi function input declaration is invalid")
    if field["type"].startswith("tuple"):
        components = field.get("components")
        if not isinstance(components, list):
            raise ValueError("tuple components must be an array")
        for component in components:
            abi_input_type(component)
    try:
        type_name = normalize(collapse_if_tuple(field))
        parse_abi_type(type_name).validate()
    except (ParseError, ValueError) as exc:
        raise ValueError("abi type is invalid") from exc
    if not registry.has_encoder(type_name):
        raise ValueError("abi type has no encoder")
    return type_name


def abi_json_value(abi_type: ABIType, value: Any) -> Any:
    """只适配 JSON 无法表达的 bytes 和大整数，结构与范围仍由 eth-abi 校验。"""
    if abi_type.is_array:
        if not isinstance(value, list):
            raise ValueError("array argument must be an array")
        return [abi_json_value(abi_type.item_type, item) for item in value]
    if isinstance(abi_type, TupleType):
        if not isinstance(value, list) or len(value) != len(abi_type.components):
            raise ValueError("tuple argument count mismatch")
        return [abi_json_value(component, item) for component, item in zip(abi_type.components, value)]
    if abi_type.base in ("bytes", "function") and isinstance(value, str):
        if not re.fullmatch(r"0x(?:[0-9a-fA-F]{2})*", value):
            raise ValueError("bytes argument must be 0x-prefixed hex")
        return bytes.fromhex(value[2:])
    if abi_type.base in ("int", "uint") and isinstance(value, str):
        if not re.fullmatch(r"-?(?:0|[1-9][0-9]*)|0x[0-9a-fA-F]+", value):
            raise ValueError("integer argument must be decimal or hex")
        return int(value, 16 if value.startswith("0x") else 10)
    return value


def build_call_data(abi: list[dict[str, Any]], method_name: str, args: list[Any]) -> dict[str, Any]:
    if not isinstance(method_name, str) or not method_name:
        raise ValueError("method is required")
    if not isinstance(args, list):
        raise ValueError("args must be an array")
    candidates = [item for item in abi_methods(abi) if method_name in (item["name"], item["signature"])]
    if len(candidates) != 1:
        raise ValueError("method must match exactly one ABI function")
    method = candidates[0]
    types = [field["type"] for field in method["inputs"]]
    if len(args) != len(types):
        raise ValueError("argument count mismatch")
    signature = method["signature"]
    try:
        selector = function_signature_to_4byte_selector(signature)
        encoded_args = encode(types, [abi_json_value(parse_abi_type(type_name), value) for type_name, value in zip(types, args)])
    except (EncodingError, TypeError, ValueError, OverflowError) as exc:
        raise ValueError("arguments do not match ABI") from exc
    return {
        "method": method["name"],
        "signature": signature,
        "calldata": to_hex(selector + encoded_args),
        "stateMutability": method["stateMutability"],
    }


class ContractUIHandler(BaseHTTPRequestHandler):

    def parse_request(self) -> bool:
        if not super().parse_request():
            return False
        header_bytes = sum(len(key) + len(value) + 4 for key, value in self.headers.items())
        if len(self.headers) > MAX_HEADER_COUNT or header_bytes > MAX_HEADER_BYTES:
            self.send_error(431, "request headers too large")
            self.close_connection = True
            return False
        return True

    def send_error(self, code: int, message: str | None = None, explain: str | None = None) -> None:
        self.close_connection = True
        self.write_json({"status": "error", "error": message or "request failed"}, status=code)

    def do_GET(self) -> None:
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
            if self.path == "/abi/methods":
                self.write_json({"methods": abi_methods(abi)})
                return
            result = build_call_data(abi, payload.get("method", ""), payload.get("args", []))
        except (TypeError, ValueError, OverflowError, KeyError, ParseError) as exc:
            self.send_error(400, str(exc))
            return
        self.write_json(result)

    def log_message(self, format: str, *args: object) -> None:
        return

    def write_json(self, payload: dict[str, object], status: int = 200) -> None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        if self.close_connection:
            self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(body)

    def write_html(self, html: str) -> None:
        body = html.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class BoundedThreadingHTTPServer(ThreadingHTTPServer):

    daemon_threads = True
    allow_reuse_address = True

    def get_request(self):
        request, client_address = super().get_request()
        request.settimeout(HTTP_TIMEOUT_SECONDS)
        return request, client_address


def run_server() -> None:
    global PORT, MAX_BODY_BYTES, HTTP_TIMEOUT_SECONDS
    PORT = required_positive_int("CHAIMIR_IMAGE_PORT")
    MAX_BODY_BYTES = required_positive_int("CHAIMIR_MAX_BODY_BYTES")
    HTTP_TIMEOUT_SECONDS = required_positive_int("CHAIMIR_HTTP_TIMEOUT_SECONDS")
    BoundedThreadingHTTPServer(("0.0.0.0", PORT), ContractUIHandler).serve_forever()


if __name__ == "__main__":
    run_server()
