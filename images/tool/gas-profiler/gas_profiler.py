#!/usr/bin/env python
"""Gas 消耗分析 CLI,从标准输入读取交易 receipt JSON 并输出统计结果。"""

from __future__ import annotations

import argparse
import json
import statistics
import sys


def load_gas_values(raw: str) -> list[int]:
    """从标准输入 JSON 读取 gasUsed 数值列表。"""
    payload = json.loads(raw)
    receipts = payload if isinstance(payload, list) else payload.get("receipts", [])
    return [int(item["gasUsed"]) for item in receipts if "gasUsed" in item]


def main() -> int:
    """解析参数并输出 gas 统计 JSON。"""
    parser = argparse.ArgumentParser(description="Chaimir gas profiler")
    parser.parse_args()
    values = load_gas_values(sys.stdin.read())
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
