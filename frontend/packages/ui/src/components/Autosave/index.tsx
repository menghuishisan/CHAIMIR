/**
 * Autosave:自动保存状态指示(§5.1,配 FE-7 服务端草稿)。
 * saving=旋转指示(加载允许旋转)/ saved=对勾 + 保存时刻 / error=告警 + 重试入口;
 * idle 不渲染内容但保留占位高度,避免状态切换时布局跳动。
 * 状态经文字 + 图标双通道表达,aria-live 让读屏用户获知保存结果。
 */
import { Check, CircleAlert, LoaderCircle } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export type AutosaveState = "idle" | "saving" | "saved" | "error";

export interface AutosaveProps {
  /** 保存状态机:idle 静默 / saving 保存中 / saved 已保存 / error 失败 */
  state: AutosaveState;
  /** 最近一次保存成功的时刻(saved 态展示 HH:mm) */
  savedAt?: Date;
  /** 保存失败的重试回调;提供时 error 态展示重试按钮 */
  onRetry?: () => void;
  className?: string;
}

/** 格式化为 HH:mm(本地时间) */
function formatTime(date: Date): string {
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${hours}:${minutes}`;
}

export function Autosave({ state, savedAt, onRetry, className }: AutosaveProps) {
  return (
    // aria-live 常驻容器:内容变化时读屏播报;min-h 保证 idle 态占位不塌陷
    <div aria-live="polite" className={cn("flex min-h-5 items-center gap-1.5 text-xs", className)}>
      {state === "saving" && (
        <>
          <Icon icon={LoaderCircle} size="sm" className="animate-spin text-warning" />
          <span className="text-warning">正在保存…</span>
        </>
      )}
      {state === "saved" && (
        <>
          <Icon icon={Check} size="sm" className="text-success" />
          <span className="text-ink-sub">
            已保存{savedAt ? ` ${formatTime(savedAt)}` : ""}
          </span>
        </>
      )}
      {state === "error" && (
        <>
          <Icon icon={CircleAlert} size="sm" className="text-danger" />
          <span className="text-danger">保存失败</span>
          {onRetry && (
            <button
              type="button"
              onClick={onRetry}
              className="rounded-sm text-danger underline underline-offset-2 transition-colors duration-fast hover:text-danger-hover focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
            >
              重试
            </button>
          )}
        </>
      )}
    </div>
  );
}
