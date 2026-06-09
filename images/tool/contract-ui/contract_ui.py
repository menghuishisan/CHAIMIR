"""合约交互面板服务,根据 ABI 描述提供可调用方法清单。"""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(os.environ.get("CHAIMIR_IMAGE_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "1048576"))


class ContractUIHandler(BaseHTTPRequestHandler):
    """处理合约 UI 请求。"""

    def do_GET(self) -> None:
        """返回健康检查和默认面板信息。"""
        if self.path == "/healthz":
            self.write_json({"status": "ok"})
            return
        if self.path == "/schema":
            self.write_json({"inputs": ["abi", "address", "method"], "mode": "teaching"})
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:
        """解析 ABI JSON 并返回方法名。"""
        if self.path != "/abi/methods":
            self.send_error(404, "not found")
            return
        length = int(self.headers.get("Content-Length", "0"))
        if length > MAX_BODY_BYTES:
            self.send_error(413, "payload too large")
            return
        body = self.rfile.read(length)
        abi = json.loads(body.decode("utf-8") or "[]")
        methods = [item.get("name") for item in abi if item.get("type") == "function" and item.get("name")]
        self.write_json({"methods": methods})

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
    ThreadingHTTPServer(("0.0.0.0", PORT), ContractUIHandler).serve_forever()
