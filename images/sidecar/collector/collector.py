"""混合实验数据采集器,从受控内部 HTTP 目标采集状态摘要。"""

from __future__ import annotations

import json
import os
import time
from pathlib import Path
from urllib.parse import urlparse
from urllib.request import Request, urlopen


def required_config(name: str) -> str:
    """读取由 M2 注入的 sidecar 配置,缺失时拒绝启动。"""
    value = os.environ.get(name, "").strip()
    if not value:
        raise RuntimeError(f"{name} is required")
    return value


TARGETS = [target.strip() for target in required_config("CHAIMIR_COLLECTOR_TARGETS").split(",") if target.strip()]
OUTPUT = Path(required_config("CHAIMIR_COLLECTOR_OUTPUT"))
try:
    INTERVAL = float(required_config("CHAIMIR_COLLECTOR_INTERVAL_SECONDS"))
except ValueError as exc:
    raise RuntimeError("CHAIMIR_COLLECTOR_INTERVAL_SECONDS must be positive") from exc
if INTERVAL <= 0:
    raise RuntimeError("CHAIMIR_COLLECTOR_INTERVAL_SECONDS must be positive")
ALLOWED_PREFIXES = [prefix.strip() for prefix in required_config("CHAIMIR_COLLECTOR_ALLOWED_PREFIXES").split(",") if prefix.strip()]
if not TARGETS or not ALLOWED_PREFIXES:
    raise RuntimeError("CHAIMIR_COLLECTOR_TARGETS and CHAIMIR_COLLECTOR_ALLOWED_PREFIXES must contain values")


def validate_target(target: str) -> None:
    """校验采集目标只能落在 M2 注入的同环境 HTTP 白名单内。"""
    parsed = urlparse(target)
    if parsed.scheme not in {"http", "https"}:
        raise ValueError("collector target scheme is not allowed")
    if not ALLOWED_PREFIXES or not any(target.startswith(prefix) for prefix in ALLOWED_PREFIXES):
        raise ValueError("collector target is outside allowed prefixes")


def fetch(target: str) -> dict[str, object]:
    """请求单个受控目标并返回状态摘要。"""
    validate_target(target)
    request = Request(target, headers={"User-Agent": "chaimir-collector"})
    started = time.time()
    with urlopen(request, timeout=3) as response:  # noqa: S310 - 目标由 M2 manifest 白名单控制。
        body = response.read(4096)
        return {
            "target": target,
            "status": response.status,
            "bytes": len(body),
            "elapsed_ms": int((time.time() - started) * 1000),
        }


def collect_once() -> list[dict[str, object]]:
    """采集所有目标,失败项以结构化错误写入输出。"""
    results: list[dict[str, object]] = []
    for target in TARGETS:
        try:
            results.append(fetch(target))
        except Exception as exc:  # noqa: BLE001 - sidecar 需要记录每个目标失败原因。
            results.append({"target": target, "error": exc.__class__.__name__})
    return results


def main() -> None:
    """循环采集并把结果写入运行时状态卷。"""
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    while True:
        payload = {"collected_at": int(time.time()), "targets": collect_once()}
        OUTPUT.write_text(json.dumps(payload, separators=(",", ":")), encoding="utf-8")
        time.sleep(INTERVAL)


if __name__ == "__main__":
    main()
