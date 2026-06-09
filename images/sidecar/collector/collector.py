"""混合实验数据采集器,从受控内部 HTTP 目标采集状态摘要。"""

from __future__ import annotations

import json
import os
import time
from pathlib import Path
from urllib.request import Request, urlopen


TARGETS = [target for target in os.environ.get("CHAIMIR_COLLECTOR_TARGETS", "").split(",") if target]
OUTPUT = Path(os.environ.get("CHAIMIR_COLLECTOR_OUTPUT", "/runtime-state/collector.json"))
INTERVAL = float(os.environ.get("CHAIMIR_COLLECTOR_INTERVAL_SECONDS", "5"))


def fetch(target: str) -> dict[str, object]:
    """请求单个受控目标并返回状态摘要。"""
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
