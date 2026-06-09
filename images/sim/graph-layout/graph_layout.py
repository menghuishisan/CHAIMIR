"""图布局后端计算仿真,把节点均匀放置到圆形布局中。"""

from __future__ import annotations

import json
import math
import sys


def layout(graph: dict[str, object]) -> dict[str, object]:
    """根据输入节点列表生成确定性的圆形布局。"""
    nodes = graph.get("nodes", [])
    if not isinstance(nodes, list):
        raise ValueError("nodes must be a list")

    count = max(len(nodes), 1)
    positioned = []
    for index, node in enumerate(nodes):
        if not isinstance(node, dict):
            raise ValueError("each node must be an object")
        angle = (2 * math.pi * index) / count
        positioned.append(
            {
                **node,
                "x": round(math.cos(angle), 6),
                "y": round(math.sin(angle), 6),
            }
        )
    return {"nodes": positioned, "edges": graph.get("edges", [])}


def main() -> int:
    """从标准输入读取图数据并输出布局结果。"""
    graph = json.load(sys.stdin)
    result = layout(graph)
    json.dump(result, sys.stdout, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
