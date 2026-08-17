/**
 * PipelinePatternView:流程视图(pipeline 模式)。
 * 纵向步骤列表:序号 + 状态图标 + 名称 + 状态词 + 说明,进行中步骤用 ProcessSpan 静态进度条。
 * 纵向排布让宽窄两种容器都可读,不需要为窄栏另做一套形态;当前步骤用玉色左边线标记。
 */
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { EMPHASIS_BOX, progressPercent, resolveEmphasis } from "../frameVisual";
import { TONE_ICON, TONE_TEXT, type DarkTone } from "./darkTone";
import { PatternFrame } from "./PatternFrame";
import type { PatternViewProps } from "./types";
import type { PipelinePattern, PipelineStep } from "@chaimir/sim-sdk";

/** 步骤状态 → 墨底色阶 */
const STEP_TONE: Record<PipelineStep["status"], DarkTone> = {
  pending: "neutral",
  running: "active",
  complete: "success",
  failed: "danger",
};

/** 步骤状态词 */
const STEP_STATUS_TEXT: Record<PipelineStep["status"], string> = {
  pending: "未开始",
  running: "进行中",
  complete: "已完成",
  failed: "未通过",
};

export function PipelinePatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<PipelinePattern>) {
  const { steps, currentStepId } = pattern.data;
  const compact = density === "panel";

  return (
    <PatternFrame density={density}>
      <ol aria-label={`${pattern.title} 步骤`} className="flex flex-col gap-1.5">
        {steps.map((step, index) => {
        const tone = STEP_TONE[step.status];
        const emphasis = resolveEmphasis(step.id, focus, step.meta);
        const isCurrent = step.id === currentStepId;
        const selected = step.id === selectedElementId;
        const content = (
          <>
            <span className="flex min-w-0 items-center gap-2">
              <span className="shrink-0 text-xs tabular-nums text-on-dark-sub">{index + 1}</span>
              <Icon icon={TONE_ICON[tone]} size="xs" className={TONE_TEXT[tone]} />
              <span className="truncate text-xs font-medium text-on-dark">{step.label}</span>
              <span className={cn("shrink-0 text-xs", TONE_TEXT[tone])}>{STEP_STATUS_TEXT[step.status]}</span>
              {isCurrent && <span className="shrink-0 text-xs text-accent">当前</span>}
            </span>
            {step.detail && (
              <span className={cn("mt-0.5 block text-xs text-on-dark-sub", compact && "line-clamp-2")}>
                {step.detail}
              </span>
            )}
            {/* 进度条只给正在跑的那一步:未开始是 0%、已完成是 100%,画出来只是噪声,
                状态词已经说清了 */}
            {step.process && step.status === "running" && (
              <span className="mt-1 flex items-center gap-2">
                {/* 静态进度条:进行中的过程用长度表达,禁用脉冲/呼吸动画 */}
                <span className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-dark-line">
                  <span
                    className="block h-full rounded-full bg-accent"
                    style={{ width: `${progressPercent(step.process.progress)}%` }}
                  />
                </span>
                <span className="shrink-0 text-xs tabular-nums text-on-dark-sub">
                  {step.process.label
                    ? `${step.process.label} ${progressPercent(step.process.progress)}%`
                    : `${progressPercent(step.process.progress)}%`}
                </span>
              </span>
            )}
          </>
        );
        const boxClass = cn(
          "block w-full rounded-md border border-l-2 bg-dark-elevated px-2 py-1.5 text-left",
          EMPHASIS_BOX[emphasis],
          isCurrent ? "border-l-accent" : "border-l-dark-line",
          selected && "border-accent",
        );

        return (
          <li key={step.id}>
            {onSelectElement ? (
              <button
                type="button"
                aria-pressed={selected}
                onClick={() => onSelectElement(step.id)}
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
          </li>
        );
        })}
      </ol>
    </PatternFrame>
  );
}
