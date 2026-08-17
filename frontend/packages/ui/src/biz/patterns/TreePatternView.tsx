/**
 * TreePatternView:树结构视图(tree 模式,如 Merkle 树)。
 * 用嵌套列表 + 缩进连线表达父子关系:树在窄栏里靠缩进而非画布,天然两种宽度都可读;
 * highlightedPath 上的节点用玉色描边并标「验证路径」文字,不靠颜色单独表达。
 */
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { EMPHASIS_BOX, resolveEmphasis, shortLabel } from "../frameVisual";
import { TONE_TEXT } from "./darkTone";
import { PatternFrame } from "./PatternFrame";
import type { PatternViewProps } from "./types";
import { GitBranch } from "lucide-react";
import type { TreeNode, TreePattern } from "@chaimir/sim-sdk";

interface TreeRowProps {
  node: TreeNode;
  depth: number;
  highlightedPath: string[];
  focus: PatternViewProps<TreePattern>["focus"];
  selectedElementId?: string;
  onSelectElement?: (elementId: string) => void;
  /** 哈希显示长度:窄栏更短 */
  hashLength: number;
}

/**
 * TreeRow:一行 = 一个节点,children 递归成嵌套 ul。
 * 缩进用左内边距表达层级,读屏借 ul/li 嵌套自然获得层级信息。
 */
function TreeRow({
  node,
  depth,
  highlightedPath,
  focus,
  selectedElementId,
  onSelectElement,
  hashLength,
}: TreeRowProps) {
  const onPath = highlightedPath.includes(node.id);
  const emphasis = resolveEmphasis(node.id, focus, node.meta);
  const selected = node.id === selectedElementId;
  const content = (
    <>
      <span className="flex min-w-0 items-center gap-1.5">
        <Icon icon={GitBranch} size="xs" className={onPath ? TONE_TEXT.success : TONE_TEXT.neutral} />
        <span className="truncate text-xs text-on-dark">{node.label}</span>
        {onPath && <span className="shrink-0 text-xs text-accent">验证路径</span>}
      </span>
      <span className="mt-0.5 block font-mono text-xs text-on-dark-sub">
        {shortLabel(node.hash, hashLength)}
      </span>
    </>
  );
  const boxClass = cn(
    "block w-full rounded-md border bg-dark-elevated px-2 py-1.5 text-left",
    EMPHASIS_BOX[emphasis],
    (onPath || selected) && "border-accent",
  );

  return (
    <li>
      <div style={{ paddingLeft: depth * 12 }}>
        {onSelectElement ? (
          <button
            type="button"
            aria-pressed={selected}
            onClick={() => onSelectElement(node.id)}
            // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
            className={cn(
              boxClass,
              "hit-target relative pressable hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
            )}
          >
            {content}
          </button>
        ) : (
          <div className={boxClass}>{content}</div>
        )}
      </div>
      {node.children && node.children.length > 0 && (
        <ul className="mt-1 flex flex-col gap-1">
          {node.children.map((child) => (
            <TreeRow
              key={child.id}
              node={child}
              depth={depth + 1}
              highlightedPath={highlightedPath}
              focus={focus}
              selectedElementId={selectedElementId}
              onSelectElement={onSelectElement}
              hashLength={hashLength}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

export function TreePatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<TreePattern>) {
  return (
    <PatternFrame density={density}>
      <ul aria-label={`${pattern.title} 树节点`} className="flex flex-col gap-1">
        <TreeRow
          node={pattern.data.root}
          depth={0}
          highlightedPath={pattern.data.highlightedPath}
          focus={focus}
          selectedElementId={selectedElementId}
          onSelectElement={onSelectElement}
          hashLength={density === "panel" ? 8 : 18}
        />
      </ul>
    </PatternFrame>
  );
}
