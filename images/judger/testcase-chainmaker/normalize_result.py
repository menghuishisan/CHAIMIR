"""判题结果归一化工具,把执行器输出收敛为 M3 统一 JSON 契约。"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


def detail(passed: bool, source: str, message: str) -> dict[str, object]:
    """生成不包含答案正本的单条判题详情。"""
    return {"passed": passed, "source": source, "message": message[:500]}


def from_exit_code(exit_code: int, source: str, stdout_path: Path | None) -> dict[str, object]:
    """按命令退出码生成统一判题结果。"""
    passed = exit_code == 0
    message = "判题命令通过" if passed else f"判题命令退出码 {exit_code}"
    if stdout_path and stdout_path.is_file():
        message = stdout_path.read_text(encoding="utf-8", errors="replace")[:500]
    return {"passed": passed, "score": 1 if passed else 0, "max_score": 1, "details": [detail(passed, source, message)]}


def from_slither(path: Path) -> dict[str, object]:
    """按 Slither JSON 报告生成静态检查判题结果。"""
    payload = json.loads(path.read_text(encoding="utf-8"))
    detectors = payload.get("results", {}).get("detectors", [])
    failed = len(detectors) > 0
    details = [
        detail(False, item.get("check", "slither"), item.get("description", "发现静态检查风险"))
        for item in detectors[:50]
    ]
    if not details:
        details = [detail(True, "slither", "未发现阻断级静态检查风险")]
    return {"passed": not failed, "score": 0 if failed else 1, "max_score": 1, "details": details}


def main() -> int:
    """解析归一化模式并输出统一 JSON。"""
    parser = argparse.ArgumentParser(description="Chaimir judge result normalizer")
    parser.add_argument("--mode", choices=("exit-code", "slither"), required=True)
    parser.add_argument("--exit-code", type=int, default=0)
    parser.add_argument("--source", default="testcase")
    parser.add_argument("--stdout", type=Path)
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()

    if args.mode == "slither":
        if not args.report:
            raise ValueError("--report is required for slither mode")
        result = from_slither(args.report)
    else:
        result = from_exit_code(args.exit_code, args.source, args.stdout)
    json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0 if result["passed"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
