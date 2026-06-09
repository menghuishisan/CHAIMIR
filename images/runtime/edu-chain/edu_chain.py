"""自研教学链节点,提供确定性的区块、交易和共识状态接口。"""

from __future__ import annotations

import hashlib
import json
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


PORT = int(os.environ.get("CHAIMIR_EDU_CHAIN_PORT", "8080"))
MAX_BODY_BYTES = int(os.environ.get("CHAIMIR_MAX_BODY_BYTES", "65536"))
CHAIN_ID = os.environ.get("CHAIMIR_EDU_CHAIN_ID", "chaimir-edu")
STARTED_AT = int(time.time())


def block_hash(height: int) -> str:
    """按高度生成确定性教学区块哈希。"""
    payload = f"{CHAIN_ID}:{height}".encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


class EduChainHandler(BaseHTTPRequestHandler):
    """处理教学链只读查询和受控交易提交请求。"""

    def do_GET(self) -> None:
        """返回健康检查、链信息或最新区块。"""
        if self.path == "/healthz":
            self.write_json({"status": "ok"})
            return
        if self.path == "/chain":
            self.write_json({"chain_id": CHAIN_ID, "consensus": "round-robin", "started_at": STARTED_AT})
            return
        if self.path == "/block/latest":
            height = max(1, int(time.time() - STARTED_AT) // 5 + 1)
            self.write_json({"height": height, "hash": block_hash(height), "previous_hash": block_hash(height - 1)})
            return
        self.send_error(404, "not found")

    def do_POST(self) -> None:
        """接收教学交易并返回确定性交易哈希。"""
        if self.path != "/tx":
            self.send_error(404, "not found")
            return
        length = int(self.headers.get("Content-Length", "0"))
        if length > MAX_BODY_BYTES:
            self.send_error(413, "payload too large")
            return
        body = self.rfile.read(length)
        tx_hash = hashlib.sha256(body).hexdigest()
        self.write_json({"accepted": True, "tx_hash": tx_hash})

    def log_message(self, format: str, *args: object) -> None:
        """关闭默认访问日志,避免输出学生请求正文。"""
        return

    def write_json(self, payload: dict[str, object]) -> None:
        """输出紧凑 JSON 响应。"""
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> None:
    """启动教学链 HTTP 节点。"""
    ThreadingHTTPServer(("0.0.0.0", PORT), EduChainHandler).serve_forever()


if __name__ == "__main__":
    main()
