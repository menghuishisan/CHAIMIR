"""钱包模拟器,提供确定性的教学钱包签名接口。"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_account import Account
from eth_account.messages import encode_defunct
from eth_utils import to_checksum_address

PORT = int(os.environ.get("CHAIMIR_IMAGE_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "1048576"))
RPC_URL = os.environ.get("CHAIMIR_EVM_RPC_URL", "").strip()
ACCOUNT = Account.create(os.urandom(32))
DEFAULT_CHAIN_ID = int(os.environ.get("CHAIMIR_EVM_CHAIN_ID", "31337"))

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
    <h2>签名并发送交易</h2>
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
    <button onclick="sendTx()">广播交易</button>
    <button class="secondary" onclick="signTxOnly()">只签名</button>
    <p class="muted">广播需要镜像运行时配置 CHAIMIR_EVM_RPC_URL。未配置时会明确返回失败原因。</p>
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
function txPayload(broadcast) {
  return {
    to: document.getElementById('to').value,
    value_wei: document.getElementById('valueWei').value,
    nonce: document.getElementById('nonce').value,
    gas: document.getElementById('gas').value,
    gas_price_wei: document.getElementById('gasPrice').value,
    chain_id: document.getElementById('chainId').value,
    data: document.getElementById('data').value,
    broadcast
  };
}
async function sendTx() {
  try { show('result', await requestJSON('/tx', { method: 'POST', body: JSON.stringify(txPayload(true)) })); }
  catch (e) { show('result', e); }
}
async function signTxOnly() {
  try { show('result', await requestJSON('/tx', { method: 'POST', body: JSON.stringify(txPayload(false)) })); }
  catch (e) { show('result', e); }
}
refreshAccount();
</script>
</body>
</html>"""


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict[str, Any]:
    """读取并校验 JSON 请求体,避免超大载荷进入签名流程。"""
    length = int(handler.headers.get("Content-Length", "0"))
    if length > MAX_BODY_BYTES:
        raise ValueError("payload too large")
    body = handler.rfile.read(length)
    if not body:
        return {}
    payload = json.loads(body.decode("utf-8"))
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
    out = int(str(value), 0)
    if out < 0:
        raise ValueError(f"{name} must be non-negative")
    return out


def sign_transaction(payload: dict[str, Any]) -> dict[str, str]:
    """签名一笔 EVM 交易,广播由调用方显式决定。"""
    to_addr = str(payload.get("to", "")).strip()
    if not to_addr:
        raise ValueError("to is required")
    tx = {
        "to": to_checksum_address(to_addr),
        "value": parse_int_field(payload, "value_wei", 0),
        "nonce": parse_int_field(payload, "nonce"),
        "gas": parse_int_field(payload, "gas", 21000),
        "gasPrice": parse_int_field(payload, "gas_price_wei", 1000000000),
        "chainId": parse_int_field(payload, "chain_id", DEFAULT_CHAIN_ID),
        "data": str(payload.get("data", "0x") or "0x"),
    }
    signed = Account.sign_transaction(tx, ACCOUNT.key)
    raw = getattr(signed, "raw_transaction", getattr(signed, "rawTransaction", b""))
    return {
        "address": ACCOUNT.address,
        "raw_transaction": "0x" + raw.hex(),
        "transaction_hash": "0x" + signed.hash.hex(),
    }


def send_raw_transaction(raw_transaction: str) -> dict[str, str]:
    """通过配置的 EVM RPC 广播交易,未配置时显式失败。"""
    if not RPC_URL:
        raise RuntimeError("CHAIMIR_EVM_RPC_URL is not configured")
    body = json.dumps(
        {"jsonrpc": "2.0", "id": 1, "method": "eth_sendRawTransaction", "params": [raw_transaction]}
    ).encode("utf-8")
    req = urllib.request.Request(RPC_URL, data=body, headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            payload = json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        raise RuntimeError(f"rpc request failed: {exc}") from exc
    if "error" in payload:
        raise RuntimeError(str(payload["error"]))
    return {"tx_hash": str(payload.get("result", ""))}


class WalletSimHandler(BaseHTTPRequestHandler):
    """处理钱包模拟请求。"""

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
                self.write_json(import_wallet(str(payload.get("private_key", ""))))
            except ValueError as exc:
                self.send_error(400, str(exc))
            return
        if self.path == "/tx":
            try:
                signed = sign_transaction(payload)
                if bool(payload.get("broadcast", True)):
                    signed.update(send_raw_transaction(signed["raw_transaction"]))
                self.write_json(signed)
            except (RuntimeError, ValueError) as exc:
                self.write_json({"status": "failed", "reason": str(exc)}, status=422)
            return
        message = str(payload.get("message", ""))
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
        self.end_headers()
        self.wfile.write(body)

    def write_html(self, body: bytes) -> None:
        """写出钱包模拟器操作页面。"""
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", PORT), WalletSimHandler).serve_forever()
