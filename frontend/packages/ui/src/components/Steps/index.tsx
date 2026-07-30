/**
 * Steps:向导步骤指示(§5.1)。
 * 横向排布:完成步=玉实底圆点 + 对勾,当前步=玉描边 + 玉浅底外圈 + aria-current,
 * 未来步=墨描边;连接线完成段染玉色。步骤状态由文字 + 图标双通道表达,色非唯一。
 */
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
  className?: string;
}

export function Steps({ steps, current, className }: StepsProps) {
  return (
    <ol className={cn("flex items-center", className)}>
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
            <span className="flex items-center gap-2">
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
            </span>
          </li>
        );
      })}
    </ol>
  );
}
