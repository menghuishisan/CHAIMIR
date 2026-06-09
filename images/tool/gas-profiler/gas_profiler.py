#!/usr/bin/env python
"""Gas 消耗分析 CLI,读取交易 receipt JSON 并输出统计结果。"""

from __future__ import annotations

import argparse
import json
import statistics
from pathlib import Path


def load_gas_values(path: Path) -> list[int]:
    """从 JSON 文件读取 gasUsed 数值列表。"""
    payload = json.loads(path.read_text(encoding="utf-8"))
    receipts = payload if isinstance(payload, list) else payload.get("receipts", [])
    return [int(item["gasUsed"]) for item in receipts if "gasUsed" in item]


def main() -> int:
    """解析参数并输出 gas 统计 JSON。"""
    parser = argparse.ArgumentParser(description="Chaimir gas profiler")
    parser.add_argument("receipts", type=Path)
    args = parser.parse_args()
    values = load_gas_values(args.receipts)
    result = {
        "count": len(values),
        "total": sum(values),
        "min": min(values) if values else 0,
        "max": max(values) if values else 0,
        "mean": statistics.mean(values) if values else 0,
    }
    print(json.dumps(result, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
