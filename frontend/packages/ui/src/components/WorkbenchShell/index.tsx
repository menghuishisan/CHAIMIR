/**
 * WorkbenchShell:沉浸式工作台壳(深色语境,规范 §7.1 通用范式)。
 * 全屏底层(z-immersive)+ 顶部工作台条 + 左(说明/上下文)· 中(主舞台)· 右(状态/操作)
 * 三栏骨架 + 可选底部控制区。只负责深色壳/槽位/响应式/无障碍,业务布局由上层填充。
 * 响应式:max-lg 时左右栏收起为舞台上下的可折叠区(舞台优先),各面板内部独立滚动,
 * 整壳不产生页面级滚动。
 * 使用契约:沉浸态必须独占路由挂载(规范 §7/§4.4 光面退场),不得作为覆盖层叠在
 * 日常页之上——否则背景可被 Tab 穿透与滚动。全屏墨色底层与进出场由沉浸路由壳提供,
 * 本组件填满该壳(不自行 fixed 定位),避免两处争夺同一块全屏层。
 */
import { ChevronDown, ChevronLeft } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

/* ---------------------------------------------------------------- 顶部工作台条 */

export interface WorkbenchTopbarProps {
  /** 退出回调(保存并退出,禁止销毁式退出作为唯一出口) */
  onExit: () => void;
  /** 退出按钮文字,默认「保存并退出」 */
  exitLabel?: string;
  /** 工作台标题 */
  title: string;
  /** 副标题(如所属课程/关卡) */
  subtitle?: string;
  /** ChainProgress 进度插槽,居中展示 */
  progress?: ReactNode;
  /** 单一主 CTA 插槽,靠右 */
  cta?: ReactNode;
}

/**
 * WorkbenchTopbar:工作台顶条 —— 最左退出、标题/副标题、居中进度、右侧单一主 CTA。
 * 焦点环用负向 offset 内收,避免在 48px 高的条内被裁切。
 */
export function WorkbenchTopbar({
  onExit,
  exitLabel = "保存并退出",
  title,
  subtitle,
  progress,
  cta,
}: WorkbenchTopbarProps) {
  return (
    <header className="flex h-12 shrink-0 items-center border-b border-dark-line bg-dark-bg">
      <button
        type="button"
        onClick={onExit}
        // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
        className="pressable flex h-full shrink-0 items-center gap-1.5 border-r border-dark-line px-4 text-sm text-on-dark-sub hover:bg-dark-surface hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
      >
        <Icon icon={ChevronLeft} size="sm" />
        {exitLabel}
      </button>
      <div className="min-w-0 px-4">
        <div className="truncate text-sm font-medium leading-tight text-on-dark">{title}</div>
        {subtitle && <div className="truncate text-xs leading-tight text-on-dark-sub">{subtitle}</div>}
      </div>
      <div className="flex min-w-0 flex-1 items-center justify-center px-2">{progress}</div>
      {cta && <div className="flex shrink-0 items-center pl-2 pr-4">{cta}</div>}
    </header>
  );
}

/* ---------------------------------------------------------------- 侧栏面板(内部) */

interface WorkbenchPanelProps {
  side: "left" | "right";
  /** 窄屏折叠按钮上的面板名 */
  label: string;
  children: ReactNode;
}

/**
 * WorkbenchPanel(内部):侧栏面板 —— 桌面为固定宽侧栏(左 256 / 右 288,细线分隔),
 * max-lg 收起为舞台上/下的受控折叠区(默认收起,舞台优先);内容区独立滚动。
 */
function WorkbenchPanel({ side, label, children }: WorkbenchPanelProps) {
  // 受控折叠(仅窄屏可见按钮);桌面断点强制展开,与折叠态互不干扰
  const [open, setOpen] = useState(false);
  const contentId = useId();
  const isLeft = side === "left";
  return (
    <aside
      className={cn(
        "flex shrink-0 flex-col border-dark-line bg-dark-bg lg:min-h-0",
        isLeft ? "border-b lg:w-64 lg:border-b-0 lg:border-r" : "border-t lg:w-72 lg:border-t-0 lg:border-l",
      )}
    >
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentId}
        onClick={() => setOpen((v) => !v)}
        // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
        className="pressable flex items-center justify-between gap-2 px-4 py-2 text-sm text-on-dark-sub hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2 lg:hidden"
      >
        <span>{label}</span>
        <Icon
          icon={ChevronDown}
          size="sm"
          className={cn("transition-transform duration-fast", open && "rotate-180")}
        />
      </button>
      <div
        id={contentId}
        className={cn(
          "min-h-0 overflow-y-auto lg:block lg:max-h-none lg:flex-1",
          // 窄屏展开时限高滚动,保证舞台仍有空间;收起时完全隐藏
          open ? "max-h-48" : "hidden",
        )}
      >
        {children}
      </div>
    </aside>
  );
}

/* ---------------------------------------------------------------- 壳 */

export interface WorkbenchShellProps {
  /** 顶部工作台条插槽(通常放 WorkbenchTopbar) */
  topbar: ReactNode;
  /** 左栏:说明/上下文 */
  left?: ReactNode;
  /** 中间主舞台(必填,占满剩余空间) */
  stage: ReactNode;
  /** 右栏:状态/结果/操作 */
  right?: ReactNode;
  /** 底部控制区(如仿真播放控制) */
  footer?: ReactNode;
  /** 窄屏左栏折叠按钮文字,默认「说明」 */
  leftLabel?: string;
  /** 窄屏右栏折叠按钮文字,默认「状态与操作」 */
  rightLabel?: string;
  className?: string;
}

/**
 * WorkbenchShell:沉浸工作台外壳 —— fixed 全屏深色底层,纵向 topbar / 三栏 / footer;
 * 所有面板内部滚动,壳本身不出现页面级滚动条。
 */
export function WorkbenchShell({
  topbar,
  left,
  stage,
  right,
  footer,
  leftLabel = "说明",
  rightLabel = "状态与操作",
  className,
}: WorkbenchShellProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col bg-substrate text-on-dark", className)}>
      {topbar}
      {/* 三栏区:桌面横排,窄屏纵排(左栏-舞台-右栏),舞台始终 flex-1 优先 */}
      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        {left && (
          <WorkbenchPanel side="left" label={leftLabel}>
            {left}
          </WorkbenchPanel>
        )}
        <main className="min-h-0 min-w-0 flex-1 overflow-y-auto">{stage}</main>
        {right && (
          <WorkbenchPanel side="right" label={rightLabel}>
            {right}
          </WorkbenchPanel>
        )}
      </div>
      {footer && <div className="shrink-0 border-t border-dark-line bg-dark-bg">{footer}</div>}
    </div>
  );
}
