/**
 * ChainProgress:签名组件 —— 链式区块进度(§5.4,全站进度语言)。
 * 可枚举进度渲染为一节节区块(方块 + 短链节):已完成=玉实底、当前=玉描边半透、
 * 待完成=墨描边、失败=danger 描边。done 增加时最新完成块播放一次「铸块」玉色脉冲
 * (§4.5,单次非循环;reduced-motion 下仅状态切换)。必配 n/m 文字与 aria-label。
 * 不适用于不可枚举的百分比进度(那是 Progress 的职责)。
 */
import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import { cn } from "../../lib/cn";
import { useReducedMotion } from "../../hooks/useReducedMotion";

export interface ChainProgressProps {
  /** 区块总数 */
  total: number;
  /** 已完成数(0..total);增加时触发铸块动效 */
  done: number;
  /** 失败区块的下标(0 起),优先于完成/待完成状态 */
  failedIndexes?: number[];
  /** 区块尺寸:sm 10px / md 14px */
  size?: "sm" | "md";
  /** 深色语境(沉浸态/深底面板)配色 */
  onDark?: boolean;
  /** 无障碍与文字标签的进度名,默认「进度」 */
  label?: string;
  className?: string;
}

export function ChainProgress({
  total,
  done,
  failedIndexes = [],
  size = "md",
  onDark = false,
  label = "进度",
  className,
}: ChainProgressProps) {
  const reducedMotion = useReducedMotion();

  // 越界防御:total 至少为 0,done 截断在 [0, total]
  const safeTotal = Math.max(0, Math.floor(total));
  const safeDone = Math.min(Math.max(0, Math.floor(done)), safeTotal);

  // 铸块动效:对比上一次 done,增加时给最新完成块挂一次 animate-mint(单次动画)。
  // 有意设计:done 一次 +N 时只动画最后一块 —— 铸块脉冲标记「最新通过时刻」,
  // 批量补齐的中间块直接落位,避免连环脉冲喧宾夺主。
  const prevDoneRef = useRef(safeDone);
  const [mintIndex, setMintIndex] = useState<number | null>(null);
  useEffect(() => {
    if (safeDone > prevDoneRef.current && !reducedMotion) {
      setMintIndex(safeDone - 1);
    }
    prevDoneRef.current = safeDone;
  }, [safeDone, reducedMotion]);

  // 动画被中断(元素隐藏/display 切换等)不会触发 animationend,需同样清理 mintIndex,
  // 否则脉冲类滞留、下一次完成不再触发。React 无 onAnimationCancel 合成事件,走原生监听;
  // 监听器挂在块元素自身,随元素销毁一并回收;clearMint 引用稳定,重复 add 会被浏览器去重。
  const clearMint = useCallback(() => setMintIndex(null), []);
  const mintBlockRef = useCallback(
    (node: HTMLSpanElement | null) => {
      if (node) node.addEventListener("animationcancel", clearMint);
    },
    [clearMint],
  );

  /** 单个区块的状态类:失败 > 已完成 > 当前(done 后第一块)> 待完成 */
  const blockClass = (i: number): string => {
    if (failedIndexes.includes(i)) return "border-danger bg-danger-bg";
    if (i < safeDone) return onDark ? "border-jade-400 bg-jade-400" : "border-jade-500 bg-jade-500";
    if (i === safeDone) return "border-accent bg-on-dark-accent-soft";
    return onDark ? "border-dark-line bg-transparent" : "border-line-strong bg-transparent";
  };

  /** 区块 i-1 与 i 之间的链节:两端都完成时染玉色 */
  const linkClass = (i: number): string => {
    if (i < safeDone) return onDark ? "bg-jade-400" : "bg-jade-500";
    return onDark ? "bg-dark-line" : "bg-line-strong";
  };

  return (
    <div
      role="img"
      aria-label={`${label}:${safeDone}/${safeTotal} 已完成`}
      className={cn("flex items-center", className)}
    >
      <div className="flex items-center">
        {Array.from({ length: safeTotal }, (_, i) => (
          <Fragment key={i}>
            {i > 0 && <span className={cn("h-px w-2.5 shrink-0", linkClass(i))} />}
            <span
              className={cn(
                "shrink-0 rounded-sm border",
                size === "sm" ? "size-2.5" : "size-3.5",
                blockClass(i),
                // 单次玉色脉冲;动画结束即摘除类,保证下一次完成还能触发
                i === mintIndex && "animate-mint",
              )}
              // end 走合成事件、cancel 走原生监听(ref),两条路径同样清理
              ref={i === mintIndex ? mintBlockRef : undefined}
              onAnimationEnd={i === mintIndex ? clearMint : undefined}
            />
          </Fragment>
        ))}
      </div>
      <span
        className={cn(
          "ml-2.5 font-mono text-xs tabular-nums",
          onDark ? "text-on-dark-sub" : "text-ink-sub",
        )}
      >
        {safeDone}/{safeTotal}
      </span>
    </div>
  );
}
