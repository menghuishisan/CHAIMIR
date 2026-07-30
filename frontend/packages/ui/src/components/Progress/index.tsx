/**
 * Progress:线性百分比进度条(§5.1)。
 * 用于不可枚举的百分比进度(可枚举的检查点/步骤请用 ChainProgress,§5.4);
 * 必配文字百分比(tabular-nums),色非唯一信息载体。
 */
import { cn } from "../../lib/cn";

export interface ProgressProps {
  /** 进度值 0-100,越界自动截断 */
  value: number;
  /** 读屏与可视标签(如「上传进度」) */
  label?: string;
  /** 填充色:primary 常规 / success 完成语义 */
  tone?: "primary" | "success";
  className?: string;
}

export function Progress({ value, label, tone = "primary", className }: ProgressProps) {
  // 非法值(NaN/Infinity)归零后再越界截断,避免填充溢出轨道或渲染异常
  const finite = Number.isFinite(value) ? value : 0;
  const clamped = Math.min(100, Math.max(0, finite));
  const percent = Math.round(clamped);

  return (
    <div className={cn("flex items-center gap-3", className)}>
      {label && <span className="shrink-0 text-sm text-ink-sub">{label}</span>}
      <div
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
        // label 未传时提供缺省无障碍名,progressbar 不得匿名
        aria-label={label ?? "进度"}
        className="h-2 flex-1 overflow-hidden rounded-full bg-surface-sunken"
      >
        <div
          className={cn(
            // 满宽填充经 scaleX 收缩到进度比例:只动 transform(§4),origin-left 从左端生长;
            // 圆角由轨道 overflow-hidden 裁切,填充自身不再带 rounded(scaleX 会横向压扁圆角)
            "h-full w-full origin-left transition-transform duration-slow ease-out",
            tone === "success" ? "bg-success" : "bg-primary",
          )}
          // 动态比例:transform 数值由进度值计算,属内联 style 的允许例外
          style={{ transform: `scaleX(${clamped / 100})` }}
        />
      </div>
      <span className="shrink-0 text-xs text-ink-sub tabular-nums">{percent}%</span>
    </div>
  );
}
