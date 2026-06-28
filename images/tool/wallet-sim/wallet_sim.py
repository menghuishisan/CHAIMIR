"""钱包模拟器,提供确定性的教学钱包签名接口。"""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

from eth_account import Account
from eth_account.messages import encode_defunct

PORT = int(os.environ.get("CHAIMIR_IMAGE_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "1048576"))
ACCOUNT = Account.create(os.urandom(32))


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


class WalletSimHandler(BaseHTTPRequestHandler):
    """处理钱包模拟请求。"""

    def do_GET(self) -> None:
        """返回健康检查和模拟账户。"""
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
        if self.path != "/sign":
            self.send_error(404, "not found")
            return
        try:
            payload = parse_json_body(self)
            message = str(payload.get("message", ""))
        except json.JSONDecodeError:
            self.send_error(400, "invalid json")
            return
        except ValueError as exc:
            self.send_error(413 if str(exc) == "payload too large" else 400, str(exc))
            return
        if not message:
            self.send_error(400, "message is required")
            return
        self.write_json(sign_message(message))

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


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", PORT), WalletSimHandler).serve_forever()
