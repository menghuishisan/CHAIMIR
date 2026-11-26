#!/usr/bin/env python
"""Gas 消耗分析 CLI,从标准输入读取交易 receipt JSON 并输出统计结果。"""

from __future__ import annotations

import argparse
import json
import re
import statistics
import sys


def load_gas_values(raw: str) -> list[int]:
    payload = json.loads(raw)
    if isinstance(payload, list):
        receipts = payload
    elif isinstance(payload, dict) and isinstance(payload.get("receipts"), list):
        receipts = payload["receipts"]
    else:
        raise ValueError("input must be a receipt array or an object with a receipts array")

    values: list[int] = []
    for item in receipts:
        if not isinstance(item, dict) or "gasUsed" not in item:
            raise ValueError("every receipt must contain gasUsed")
        value = item["gasUsed"]
        if isinstance(value, bool) or not isinstance(value, (int, str)):
            raise ValueError("gasUsed must be an integer or JSON-RPC quantity")
        if isinstance(value, int):
            parsed = value
        elif re.fullmatch(r"0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)", value):
            parsed = int(value, 16)
        elif re.fullmatch(r"0|[1-9][0-9]*", value):
            parsed = int(value, 10)
        else:
            raise ValueError("gasUsed must be a non-negative integer")
        if parsed < 0:
            raise ValueError("gasUsed must be non-negative")
        values.append(parsed)
    return values


def main() -> int:
    parser = argparse.ArgumentParser(description="Chaimir gas profiler")
    parser.parse_args()
    try:
        values = load_gas_values(sys.stdin.read())
    except ValueError as error:
        parser.error(str(error))
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
