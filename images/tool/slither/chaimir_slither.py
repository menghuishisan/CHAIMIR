#!/usr/bin/env python3
"""tool/slither 的平台固定入口：按组合选择的版本扫描整个工作区。"""

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import sys


SUPPORTED_VERSIONS = ("0.8.20", "0.8.24", "0.8.26")


def main() -> int:
    """验证组合注入的版本后，对工作区执行结构化 Slither 扫描。"""
    version = os.environ.get("SOLIDITY_VERSION", "").strip()
    if version not in SUPPORTED_VERSIONS:
        print("SOLIDITY_VERSION 必须是组合声明的已支持版本", file=sys.stderr)
        return 64
    solc_path = Path(f"/usr/local/bin/solc-{version}")
    if not solc_path.is_file():
        print(f"Solidity 编译器 {version} 不存在", file=sys.stderr)
        return 64
    version_check = subprocess.run([str(solc_path), "--version"], capture_output=True, text=True, check=False)
    if version_check.returncode != 0 or f"Version: {version}" not in version_check.stdout:
        print(f"Solidity 编译器 {version} 版本校验失败", file=sys.stderr)
        return 65
    if not any(path.is_file() for path in Path("/workspace").rglob("*.sol")):
        print("工作区没有可扫描的 Solidity 源文件", file=sys.stderr)
        return 64
    result = subprocess.run(
        ["/opt/slither/bin/slither", "/workspace", "--solc", str(solc_path), "--json", "-"],
        capture_output=True, text=True, check=False,
    )
    sys.stdout.write(result.stdout)
    sys.stderr.write(result.stderr)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
