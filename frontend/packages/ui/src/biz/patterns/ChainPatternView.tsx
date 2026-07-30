/**
 * ChainPatternView:区块链条视图(chain 模式)。
 * 主链横向排布,分叉链在主链下方另起一行并以缩进 + 虚线连接表达「从某高度分出」;
 * 规范链尖(canonicalTip)用玉色标记,孤块/攻击者块用形状(虚线框)+ 文字双重区分。
 * 窄栏下切换为纵向堆叠,不做横向滚动(侧栏横滚不可用)。
 */
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { EMPHASIS_BOX, resolveEmphasis, shortLabel } from "../frameVisual";
import { TONE_BORDER, TONE_ICON, TONE_TEXT, type DarkTone } from "./darkTone";
import type { PatternViewProps } from "./types";
import type { ChainBlock, ChainPattern } from "@chaimir/sim-sdk";

/** 区块状态 → 墨底色阶 */
const BLOCK_TONE: Record<ChainBlock["status"], DarkTone> = {
  genesis: "neutral",
  pending: "active",
  canonical: "success",
  orphaned: "warning",
  attacker: "danger",
};

/** 区块状态词(用户向:不写 orphaned/canonical 这类术语) */
const BLOCK_STATUS_TEXT: Record<ChainBlock["status"], string> = {
  genesis: "创世块",
  pending: "待确认",
  canonical: "已进主链",
  orphaned: "已被抛弃",
  attacker: "攻击者块",
};

/** 虚线框:被抛弃与攻击者块用虚线,灰度下也能与主链块区分 */
const BLOCK_OUTLINE: Record<ChainBlock["status"], string> = {
  genesis: "border-solid",
  pending: "border-dashed",
  canonical: "border-solid",
  orphaned: "border-dashed",
  attacker: "border-dashed",
};

interface BlockCardProps {
  block: ChainBlock;
  emphasis: ReturnType<typeof resolveEmphasis>;
  isTip: boolean;
  selected: boolean;
  onSelect?: () => void;
}

/**
 * BlockCard:单个区块卡 —— 高度 + 名称 + 状态词 + 哈希片段。
 * 可选中时渲染为按钮(键盘可达),只读时渲染为静态块。
 */
function BlockCard({ block, emphasis, isTip, selected, onSelect }: BlockCardProps) {
  const tone = BLOCK_TONE[block.status];
  const content = (
    <>
      <span className="flex items-center gap-1.5">
        <Icon icon={TONE_ICON[tone]} size="xs" className={TONE_TEXT[tone]} />
        <span className="text-xs tabular-nums text-on-dark-sub">#{block.height}</span>
        {isTip && <span className="text-xs text-accent">链尖</span>}
      </span>
      <span className="mt-0.5 block truncate text-xs text-on-dark">{block.label}</span>
      <span className={cn("mt-0.5 block text-xs", TONE_TEXT[tone])}>{BLOCK_STATUS_TEXT[block.status]}</span>
      <span className="mt-0.5 block font-mono text-xs text-on-dark-sub">{shortLabel(block.hash, 8)}</span>
    </>
  );
  const boxClass = cn(
    "block w-32 shrink-0 rounded-md border bg-dark-elevated px-2 py-1.5 text-left",
    BLOCK_OUTLINE[block.status],
    TONE_BORDER[tone],
    EMPHASIS_BOX[emphasis],
    (selected || isTip) && "border-accent",
  );
  return onSelect ? (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onSelect}
      // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
      className={cn(
        boxClass,
        "pressable hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
      )}
    >
      {content}
    </button>
  ) : (
    <div className={boxClass}>{content}</div>
  );
}

export function ChainPatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<ChainPattern>) {
  const { blocks, forks, canonicalTip } = pattern.data;
  const compact = density === "panel";
  // 窄栏纵向堆叠(侧栏内横向滚动不可用),宽舞台横向成链
  const laneClass = compact ? "flex flex-col gap-2" : "flex flex-wrap items-stretch gap-2";

  return (
    <div className="flex flex-col gap-3">
      <div className={laneClass} role="list" aria-label={`${pattern.title} 主链区块`}>
        {blocks.map((block) => (
          <div key={block.id} role="listitem" className="flex items-center gap-2">
            <BlockCard
              block={block}
              emphasis={resolveEmphasis(block.id, focus, block.meta)}
              isTip={block.id === canonicalTip}
              selected={block.id === selectedElementId}
              onSelect={onSelectElement ? () => onSelectElement(block.id) : undefined}
            />
          </div>
        ))}
      </div>

      {forks.map((fork, forkIndex) => {
        const firstBlock = fork[0];
        if (!firstBlock) return null;
        return (
          <div key={firstBlock.id} className="rounded-md border border-dashed border-dark-line p-2">
            <div className="mb-1.5 text-xs text-on-dark-sub">
              分叉 {forkIndex + 1}:从高度 {firstBlock.height - 1} 分出,共 {fork.length} 个块
            </div>
            <div className={laneClass} role="list" aria-label={`${pattern.title} 分叉 ${forkIndex + 1}`}>
              {fork.map((block) => (
                <div key={block.id} role="listitem">
                  <BlockCard
                    block={block}
                    emphasis={resolveEmphasis(block.id, focus, block.meta)}
                    isTip={block.id === canonicalTip}
                    selected={block.id === selectedElementId}
                    onSelect={onSelectElement ? () => onSelectElement(block.id) : undefined}
                  />
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
