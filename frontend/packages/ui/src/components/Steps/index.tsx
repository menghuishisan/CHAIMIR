/**
 * Steps:向导步骤指示(§5.1)。
 * 横向排布:完成步=玉实底圆点 + 对勾,当前步=玉描边 + 玉浅底外圈 + aria-current,
 * 未来步=墨描边;连接线完成段染玉色。步骤状态由文字 + 图标双通道表达,色非唯一。
 */
import type { ReactNode } from "react";
import { Check } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface StepItem {
  /** 步骤唯一键 */
  key: string;
  /** 步骤名 */
  label: string;
  /** 可选辅助说明 */
  description?: string;
}

export interface StepsProps {
  steps: StepItem[];
  /** 当前步下标(0 起);之前为完成,之后为未来 */
  current: number;
  /** 传入时已完成的步骤可点回去(按钮);当前步与未来步不可点 */
  onSelect?: (index: number) => void;
  className?: string;
}

export function Steps({ steps, current, onSelect, className }: StepsProps) {
  return (
    <div className="min-w-0 max-w-full overflow-x-auto pb-1">
      <ol className={cn("flex min-w-max items-center", className)}>
        {steps.map((step, i) => {
          const isDone = i < current;
          const isCurrent = i === current;
          return (
            <li
              key={step.key}
              aria-current={isCurrent ? "step" : undefined}
              className={cn("flex items-center", i > 0 && "flex-1")}
            >
              {/* 连接线:前一步完成则染玉色 */}
              {i > 0 && (
                <span
                  aria-hidden="true"
                  className={cn("mx-3 h-px min-w-4 flex-1", i <= current ? "bg-primary" : "bg-line")}
                />
              )}
              <StepWrapper
                onSelect={isDone && onSelect ? () => onSelect(i) : undefined}
                label={`回到第 ${i + 1} 步:${step.label}`}
              >
                {/* 步骤圆点:完成=实底对勾 / 当前=描边+外圈 / 未来=灰描边序号 */}
                {isDone ? (
                  <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary text-on-dark">
                    <Icon icon={Check} size="sm" />
                  </span>
                ) : (
                  <span
                    className={cn(
                      "flex size-6 shrink-0 items-center justify-center rounded-full border text-xs font-medium tabular-nums",
                      isCurrent
                        ? "border-2 border-primary bg-surface text-primary ring-4 ring-primary-soft"
                        : "border-line-strong text-ink-sub",
                    )}
                  >
                    {i + 1}
                  </span>
                )}
                <span className="flex min-w-0 flex-col">
                  <span
                    className={cn(
                      "text-sm",
                      isCurrent ? "font-medium text-ink" : isDone ? "text-ink" : "text-ink-sub",
                    )}
                  >
                    {step.label}
                  </span>
                  {step.description && (
                    <span className="text-xs text-ink-sub">{step.description}</span>
                  )}
                </span>
              </StepWrapper>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

/** StepWrapper 可点的完成步是按钮,其余是普通容器 */
function StepWrapper({
  onSelect,
  label,
  children,
}: {
  onSelect: (() => void) | undefined;
  label: string;
  children: ReactNode;
}) {
  if (!onSelect) return <span className="flex items-center gap-2">{children}</span>;
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-label={label}
      className="flex items-center gap-2 rounded-md text-left transition-opacity duration-fast hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
    >
      {children}
    </button>
  );
}
