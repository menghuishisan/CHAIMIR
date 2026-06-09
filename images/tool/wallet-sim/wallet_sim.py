"""钱包模拟器,提供确定性签名教学接口。"""

from __future__ import annotations

import hashlib
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(os.environ.get("CHAIMIR_IMAGE_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "1048576"))


class WalletSimHandler(BaseHTTPRequestHandler):
    """处理钱包模拟请求。"""

    def do_GET(self) -> None:
        """返回健康检查和模拟账户。"""
        if self.path == "/healthz":
            self.write_json({"status": "ok"})
            return
        if self.path == "/accounts":
            self.write_json({"accounts": ["student-demo"]})
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:
        """根据请求体生成确定性教学签名。"""
        if self.path != "/sign":
            self.send_error(404, "not found")
            return
        length = int(self.headers.get("Content-Length", "0"))
        if length > MAX_BODY_BYTES:
            self.send_error(413, "payload too large")
            return
        body = self.rfile.read(length)
        signature = hashlib.sha256(b"wallet-sim:" + body).hexdigest()
        self.write_json({"signature": signature})

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
