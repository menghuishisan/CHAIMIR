/**
 * ElementList:图形化视图的文本化元素清单(§7.1 画布必须有文本替代)。
 * 每行 = 图标 + 名称 + 状态词 + 补充说明,并作为图元的键盘可达入口:
 * 画布里的图元不承担焦点,选择一律经本清单的按钮完成,读屏与键盘用户走同一条路径。
 */
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { EMPHASIS_LABEL, LIFECYCLE_LABEL, progressPercent, type FrameEmphasis } from "../frameVisual";
import { TONE_ICON, TONE_TEXT, type DarkTone } from "./darkTone";
import type { VisualElementMeta } from "@chaimir/sim-sdk";

export interface ElementListItem {
  /** 元素编号,同时用于选中态与标注归位 */
  id: string;
  /** 用户可读名称 */
  label: string;
  /** 状态色阶 */
  tone: DarkTone;
  /** 状态词(必填:颜色不是唯一表达) */
  statusText: string;
  /** 补充说明(如边上的说明、节点当前值) */
  detail?: string;
  /** 帧焦点解析后的强调档 */
  emphasis: FrameEmphasis;
  /** 过程进度(0~1);只有真的在推进(0<p<1)时才渲染进度条 */
  progress?: number;
  /** 过程说明(如「区块传播中」) */
  progressLabel?: string;
  /** 生命周期,用于历史元素标注「已归档」等 */
  lifecycle?: VisualElementMeta["lifecycle"]["state"];
}

export interface ElementListProps {
  /** 清单标题(读屏用,如「节点」「消息」) */
  label: string;
  items: ElementListItem[];
  selectedElementId?: string;
  onSelectElement?: (elementId: string) => void;
  className?: string;
}

/** 强调档 → 行样式:历史与淡出后退,焦点用玉色左边线前置 */
const ROW_EMPHASIS: Record<FrameEmphasis, string> = {
  focus: "border-l-accent bg-dark-elevated",
  context: "border-l-transparent",
  history: "border-l-transparent opacity-70",
  ghost: "border-l-transparent opacity-40",
};

export function ElementList({
  label,
  items,
  selectedElementId,
  onSelectElement,
  className,
}: ElementListProps) {
  const interactive = Boolean(onSelectElement);
  /** 过程条:只在过程真的在推进时出现(0=还没开始、1=已结束,状态词已经说清,画条只是噪声) */
  return (
    <ul aria-label={label} className={cn("flex flex-col gap-1", className)}>
      {items.map((item) => {
        const selected = item.id === selectedElementId;
        const body = (
          <>
            <span className="flex min-w-0 items-center gap-2">
              <Icon icon={TONE_ICON[item.tone]} size="xs" className={TONE_TEXT[item.tone]} />
              <span className="truncate text-xs text-on-dark">{item.label}</span>
              <span className={cn("shrink-0 text-xs", TONE_TEXT[item.tone])}>{item.statusText}</span>
            </span>
            {(item.detail || item.lifecycle || item.emphasis === "focus") && (
              <span className="mt-0.5 block truncate text-xs text-on-dark-sub">
                {[
                  item.detail,
                  item.lifecycle ? LIFECYCLE_LABEL[item.lifecycle] : undefined,
                  item.emphasis === "focus" ? EMPHASIS_LABEL.focus : undefined,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
            )}
            {item.progress !== undefined && item.progress > 0 && item.progress < 1 && (
              <span className="mt-1 flex items-center gap-2">
                {/* 静态进度条:过程用长度表达,不用脉冲动画 */}
                <span className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-dark-line">
                  <span
                    className="block h-full rounded-full bg-accent"
                    style={{ width: `${progressPercent(item.progress)}%` }}
                  />
                </span>
                <span className="shrink-0 text-xs tabular-nums text-on-dark-sub">
                  {item.progressLabel
                    ? `${item.progressLabel} ${progressPercent(item.progress)}%`
                    : `${progressPercent(item.progress)}%`}
                </span>
              </span>
            )}
          </>
        );

        return (
          <li key={item.id}>
            {interactive ? (
              <button
                type="button"
                aria-pressed={selected}
                onClick={() => onSelectElement?.(item.id)}
                // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
                className={cn(
                  "hit-target relative pressable block w-full border-l-2 px-2 py-1.5 text-left hover:bg-dark-elevated focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
                  ROW_EMPHASIS[item.emphasis],
                  selected && "bg-dark-elevated",
                )}
              >
                {body}
              </button>
            ) : (
              <div className={cn("block w-full border-l-2 px-2 py-1.5", ROW_EMPHASIS[item.emphasis])}>
                {body}
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
