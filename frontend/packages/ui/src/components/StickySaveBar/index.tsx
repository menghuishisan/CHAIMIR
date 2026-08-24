/**
 * StickySaveBar:配置表单族的底部粘性保存条(规范 §6.5.3 第 ③ 族)。
 *
 * 配置页的表单往往比一屏长。把保存按钮放在表单末尾意味着:改完中间某一项后,
 * 用户必须滚到最底才能存,而且「我到底改了几处、存没存」全程无处可看。
 * 保存条常驻底部把这两件事同时解决 —— 状态与动作在同一条上,任何滚动位置都可达。
 *
 * 墨色底:它浮在光面之上、跨越整块内容区,用墨底与纸面区分层级 ——
 * 光面上再放一块近白条会与下方的抬起片糊在一起。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface StickySaveBarProps {
  /**
   * 未保存改动数;0 表示与服务端一致。
   * 用数量而不是布尔:「有改动」和「改了 3 处」对用户是两种信息量。
   */
  dirtyCount: number;
  /** 保存中:按钮进 loading,同时禁用放弃 */
  saving?: boolean;
  /** 保存动作槽位:传 Button(variant="primary" 或 on-dark 系) */
  saveAction: ReactNode;
  /** 放弃改动槽位;无改动时由本组件隐藏 */
  discardAction?: ReactNode;
  /** 额外状态说明,如「上次保存 12:04」或字段级错误摘要 */
  hint?: ReactNode;
  className?: string;
}

export function StickySaveBar({
  dirtyCount,
  saving = false,
  saveAction,
  discardAction,
  hint,
  className,
}: StickySaveBarProps) {
  const isDirty = dirtyCount > 0;
  return (
    <div
      // sticky 贴在滚动容器底部;z-sticky 走令牌阶(§2.4),不写裸数字
      className={cn(
        "sticky bottom-0 z-sticky mt-4 flex flex-wrap items-center justify-between gap-3",
        "rounded-lg bg-dark-surface px-4 py-3 text-sm text-on-dark shadow-md",
        className,
      )}
      // 保存态变化要让读屏知道,但不抢焦点
      role="status"
      aria-live="polite"
    >
      <div className="min-w-0">
        <span className="tabular-nums">
          {saving
            ? "正在保存…"
            : isDirty
              ? `有 ${dirtyCount} 处改动未保存`
              : "所有改动已保存"}
        </span>
        {hint && <span className="ml-3 text-on-dark-sub">{hint}</span>}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {/* 无改动时不给「放弃」:没有可放弃的东西,留着只会让人怀疑自己漏改了什么 */}
        {isDirty && !saving ? discardAction : null}
        {saveAction}
      </div>
    </div>
  );
}
