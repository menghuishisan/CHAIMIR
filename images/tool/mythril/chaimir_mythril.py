#!/usr/bin/env python3
"""tool/mythril 的平台固定入口：按组合选择的版本扫描整个工作区。"""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess


SUPPORTED_VERSIONS = ("0.8.20", "0.8.24", "0.8.26")
WORKSPACE = Path("/workspace")


def emit_failure(message: str, exit_code: int = 64) -> int:
    print(json.dumps({"success": False, "error": message}, ensure_ascii=False))
    return exit_code


def main() -> int:
    version = os.environ.get("SOLIDITY_VERSION", "").strip()
    if version not in SUPPORTED_VERSIONS:
        return emit_failure("SOLIDITY_VERSION 必须是组合声明的已支持版本")
    solc_path = Path(f"/usr/local/bin/solc-{version}")
    if not solc_path.is_file():
        return emit_failure(f"Solidity 编译器 {version} 不存在")
    version_check = subprocess.run([str(solc_path), "--version"], capture_output=True, text=True, check=False)
    if version_check.returncode != 0 or f"Version: {version}" not in version_check.stdout:
        return emit_failure(f"Solidity 编译器 {version} 版本校验失败")
    sources = sorted(path for path in WORKSPACE.rglob("*.sol") if path.is_file())
    if not sources:
        return emit_failure("工作区没有可分析的 Solidity 源文件")

    reports = []
    for source in sources:
        result = subprocess.run(
            ["/opt/mythril/bin/myth", "analyze", "--solv", version, "-o", "json", str(source)],
            cwd=str(WORKSPACE), capture_output=True, text=True, check=False,
        )
        try:
            report = json.loads(result.stdout.strip())
        except json.JSONDecodeError:
            return emit_failure(f"Mythril 对 {source} 的输出不是 JSON: {result.stderr.strip()}")
        reports.append({"source": str(source), "exit_code": result.returncode, "report": report})

    # Mythril 的分析错误也可能以退出码 0 返回，必须同时检查 JSON。
    success = all(
        item["exit_code"] in (0, 1)
        and isinstance(item["report"], dict)
        and item["report"].get("success") is True
        and item["report"].get("error") is None
        and isinstance(item["report"].get("issues"), list)
        for item in reports
    )
    print(json.dumps({"success": success, "solidity_version": version, "files": reports}, ensure_ascii=False))
    return 0 if success else 1


if __name__ == "__main__":
    raise SystemExit(main())
