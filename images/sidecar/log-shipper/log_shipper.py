"""沙箱日志采集 sidecar,按行读取日志文件并输出脱敏 JSON。"""

from __future__ import annotations

import json
import os
import re
import time
from pathlib import Path


LOG_PATH = Path(os.environ.get("CHAIMIR_LOG_PATH", "/workspace/logs/runtime.log"))
TOKEN_PATTERN = re.compile(r"(?i)(token|secret|password|private_key)=\S+")


def sanitize(line: str) -> str:
    """移除日志中的常见敏感键值。"""
    return TOKEN_PATTERN.sub(r"\1=<redacted>", line.rstrip("\n"))


def follow(path: Path) -> None:
    """持续读取日志文件并输出结构化日志。"""
    while not path.exists():
        time.sleep(1)
    with path.open("r", encoding="utf-8", errors="replace") as handle:
        handle.seek(0, os.SEEK_END)
        while True:
            line = handle.readline()
            if not line:
                time.sleep(1)
                continue
            print(json.dumps({"line": sanitize(line)}, ensure_ascii=False, separators=(",", ":")), flush=True)


def main() -> None:
    """启动日志采集流程。"""
    follow(LOG_PATH)


if __name__ == "__main__":
    main()
