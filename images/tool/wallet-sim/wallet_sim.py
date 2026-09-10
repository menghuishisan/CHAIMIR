"""钱包模拟器,提供确定性的教学钱包签名接口。"""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_account import Account
from eth_account.messages import encode_defunct
from eth_utils import to_checksum_address

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


PORT = required_positive_int("CHAIMIR_IMAGE_PORT")
MAX_BODY_BYTES = required_positive_int("CHAIMIR_MAX_BODY_BYTES")
HTTP_TIMEOUT_SECONDS = required_positive_int("CHAIMIR_HTTP_TIMEOUT_SECONDS")
MAX_HEADER_BYTES = 16 * 1024
MAX_HEADER_COUNT = 64
ACCOUNT = Account.create(os.urandom(32))

INDEX_HTML = """<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Chaimir Wallet Simulator</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f7faf9; color: #17211f; }
    main { max-width: 980px; margin: 0 auto; padding: 24px; }
    h1 { font-size: 24px; margin: 0 0 18px; }
    section { background: #fff; border: 1px solid #d8e4e0; border-radius: 8px; padding: 16px; margin-bottom: 14px; }
    label { display: block; font-size: 13px; font-weight: 600; margin: 10px 0 6px; }
    input, textarea { box-sizing: border-box; width: 100%; border: 1px solid #b8c8c4; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; }
    textarea { min-height: 72px; resize: vertical; }
    button { border: 0; border-radius: 6px; background: #007c89; color: #fff; font-weight: 700; padding: 10px 14px; margin-top: 12px; cursor: pointer; }
    button.secondary { background: #40514d; }
    pre { overflow: auto; background: #0d1715; color: #d7fff8; padding: 12px; border-radius: 6px; min-height: 44px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 14px; }
    .muted { color: #536662; font-size: 13px; }
  </style>
</head>
<body>
<main>
  <h1>Chaimir Wallet Simulator</h1>
  <section>
    <button onclick="refreshAccount()">刷新账户</button>
    <pre id="account"></pre>
  </section>
  <div class="grid">
    <section>
      <h2>导入测试钱包</h2>
      <label for="privateKey">私钥</label>
      <input id="privateKey" type="password" placeholder="0x...">
      <button onclick="importWallet()">导入</button>
      <p class="muted">只用于当前沙箱会话内的教学签名,不会写入平台数据库。</p>
    </section>
    <section>
      <h2>签名消息</h2>
      <label for="message">消息</label>
      <textarea id="message">chaimir-wallet-test</textarea>
      <button onclick="signMessage()">签名</button>
    </section>
  </div>
  <section>
    <h2>签名交易</h2>
    <div class="grid">
      <label>接收地址<input id="to" placeholder="0x..."></label>
      <label>金额 Wei<input id="valueWei" value="0"></label>
      <label>Nonce<input id="nonce" value="0"></label>
      <label>Gas<input id="gas" value="21000"></label>
      <label>Gas Price Wei<input id="gasPrice" value="1000000000"></label>
      <label>Chain ID<input id="chainId" value="31337"></label>
    </div>
    <label for="data">Data</label>
    <input id="data" value="0x">
    <button class="secondary" onclick="signTxOnly()">签名交易</button>
    <p class="muted">交易签名结果由工作台提交到当前组合的链运行时,工具本身不直连节点。</p>
  </section>
  <section>
    <h2>结果</h2>
    <pre id="result"></pre>
  </section>
</main>
<script>
async function requestJSON(path, options = {}) {
  const response = await fetch(path, options);
  const text = await response.text();
  let payload;
  try { payload = text ? JSON.parse(text) : {}; } catch { payload = { raw: text }; }
  if (!response.ok) throw payload;
  return payload;
}
function show(id, value) {
  document.getElementById(id).textContent = JSON.stringify(value, null, 2);
}
async function refreshAccount() {
  try { show('account', await requestJSON('/accounts')); } catch (e) { show('account', e); }
}
async function importWallet() {
  try {
    const private_key = document.getElementById('privateKey').value;
    const data = await requestJSON('/import', { method: 'POST', body: JSON.stringify({ private_key }) });
    show('result', data); await refreshAccount();
  } catch (e) { show('result', e); }
}
async function signMessage() {
  try {
    const message = document.getElementById('message').value;
    show('result', await requestJSON('/sign', { method: 'POST', body: JSON.stringify({ message }) }));
  } catch (e) { show('result', e); }
}
function txPayload() {
  return {
    to: document.getElementById('to').value,
    value_wei: document.getElementById('valueWei').value,
    nonce: document.getElementById('nonce').value,
    gas: document.getElementById('gas').value,
    gas_price_wei: document.getElementById('gasPrice').value,
    chain_id: document.getElementById('chainId').value,
    data: document.getElementById('data').value
  };
}
async function signTxOnly() {
  try { show('result', await requestJSON('/tx', { method: 'POST', body: JSON.stringify(txPayload()) })); }
  catch (e) { show('result', e); }
}
refreshAccount();
</script>
</body>
</html>"""


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict[str, Any]:
    """读取并校验 JSON 请求体,避免超大载荷进入签名流程。"""
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
    if not body:
        return {}
    try:
        payload = json.loads(body.decode("utf-8"))
    except UnicodeDecodeError as exc:
        raise ValueError("body must be UTF-8 JSON") from exc
    if not isinstance(payload, dict):
        raise ValueError("body must be a json object")
    return payload


def sign_message(message: str) -> dict[str, str]:
    """按 Ethereum personal_sign 语义生成教学签名。"""
    signable = encode_defunct(text=message)
    signed = Account.sign_message(signable, private_key=ACCOUNT.key)
    recovered = Account.recover_message(signable, signature=signed.signature)
    return {
        "address": ACCOUNT.address,
        "message": message,
        "signature": "0x" + signed.signature.hex(),
        "message_hash": "0x" + signed.message_hash.hex(),
        "recovered_address": recovered,
    }


def import_wallet(private_key: str) -> dict[str, str]:
    """导入教学测试私钥并返回当前账户地址。"""
    global ACCOUNT
    private_key = private_key.strip()
    if not private_key:
        raise ValueError("private_key is required")
    ACCOUNT = Account.from_key(private_key)
    return {"address": ACCOUNT.address, "scheme": "ethereum-personal-sign"}


def parse_int_field(payload: dict[str, Any], name: str, default: int | None = None) -> int:
    """解析交易数字字段,拒绝负数和无法解析的输入。"""
    value = payload.get(name, default)
    if value is None or value == "":
        raise ValueError(f"{name} is required")
    if isinstance(value, bool) or not isinstance(value, (int, str)):
        raise ValueError(f"{name} must be an integer")
    try:
        out = int(str(value), 0)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if out < 0:
        raise ValueError(f"{name} must be non-negative")
    return out


def sign_transaction(payload: dict[str, Any]) -> dict[str, str]:
    """签名一笔 EVM 交易,广播由 M2 链能力负责。"""
    to_addr = str(payload.get("to", "")).strip()
    if not to_addr:
        raise ValueError("to is required")
    try:
        checksum_to = to_checksum_address(to_addr)
    except (TypeError, ValueError) as exc:
        raise ValueError("to must be a valid EVM address") from exc
    data = payload.get("data", "0x") or "0x"
    if not isinstance(data, str) or len(data) % 2 != 0 or not data.startswith("0x"):
        raise ValueError("data must be 0x-prefixed hex")
    try:
        bytes.fromhex(data[2:])
    except ValueError as exc:
        raise ValueError("data must be 0x-prefixed hex") from exc
    tx = {
        "to": checksum_to,
        "value": parse_int_field(payload, "value_wei", 0),
        "nonce": parse_int_field(payload, "nonce"),
        "gas": parse_int_field(payload, "gas", 21000),
        "gasPrice": parse_int_field(payload, "gas_price_wei", 1000000000),
        "chainId": parse_int_field(payload, "chain_id"),
        "data": data,
    }
    try:
        signed = Account.sign_transaction(tx, ACCOUNT.key)
    except (TypeError, ValueError, OverflowError) as exc:
        raise ValueError("transaction fields are invalid") from exc
    raw = getattr(signed, "raw_transaction", getattr(signed, "rawTransaction", b""))
    return {
        "address": ACCOUNT.address,
        "raw_transaction": "0x" + raw.hex(),
        "transaction_hash": "0x" + signed.hash.hex(),
    }


class WalletSimHandler(BaseHTTPRequestHandler):
    """处理钱包模拟请求。"""

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
        """返回健康检查和模拟账户。"""
        if self.path == "/":
            self.write_html(INDEX_HTML.encode("utf-8"))
            return
        if self.path == "/healthz":
            self.write_json({"status": "ok", "address": ACCOUNT.address})
            return
        if self.path == "/accounts":
            self.write_json(
                {
                    "accounts": [
                        {
                            "label": "student-wallet",
                            "address": ACCOUNT.address,
                            "scheme": "ethereum-personal-sign",
                        }
                    ]
                }
            )
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:
        """根据请求体生成可验证的教学钱包签名。"""
        self.close_connection = True
        if self.path not in {"/sign", "/import", "/tx"}:
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
        if self.path == "/import":
            try:
                private_key = payload.get("private_key", "")
                if not isinstance(private_key, str):
                    raise ValueError("private_key must be a string")
                self.write_json(import_wallet(private_key))
            except (TypeError, ValueError) as exc:
                self.send_error(400, str(exc))
            return
        if self.path == "/tx":
            try:
                signed = sign_transaction(payload)
                self.write_json(signed)
            except (TypeError, ValueError, OverflowError) as exc:
                self.write_json({"status": "failed", "reason": str(exc)}, status=422)
            return
        message = payload.get("message", "")
        if not isinstance(message, str):
            self.send_error(400, "message must be a string")
            return
        if not message:
            self.send_error(400, "message is required")
            return
        self.write_json(sign_message(message))

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

    def write_html(self, body: bytes) -> None:
        """写出钱包模拟器操作页面。"""
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


if __name__ == "__main__":
    BoundedThreadingHTTPServer(("0.0.0.0", PORT), WalletSimHandler).serve_forever()
