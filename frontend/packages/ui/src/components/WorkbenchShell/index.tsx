/**
 * WorkbenchShell:沉浸式工作台壳(深色语境,规范 §7.1 三区骨架)。
 *
 * 主体优先:中间**主舞台**是主体,左**辅助区**放解释与操作,右**事件流**放随时间增长的消息。
 * 两侧各自可独立收起为 `--wb-rail-w` 宽的竖向把手,收起后主舞台自适应占满释放的宽度
 * (舞台取 flex-1,不做宽度计算);把手上保留面板名与未读计数 —— 收起不等于失联。
 * 折叠状态本机持久化(键名 chaimir.workbench.*),只在用户显式收放过之后才生效;
 * 没有偏好时按视口给默认值:宽屏展开、窄屏收起(舞台优先)。
 *
 * 主舞台永不滚动:<main> 不给 overflow,舞台内的图形取剩余高度、清单自行滚动。
 * 槽位内容的滚动同样归内容自己 —— 壳把面板做成定高的 flex 列,好让「手风琴占满剩余高度 +
 * 操作常驻底部」这类布局成立;壳自己套一层 overflow 会让它们失效。
 *
 * 使用契约:沉浸态必须独占路由挂载(规范 §7/§4.4 光面退场),不得作为覆盖层叠在
 * 日常页之上——否则背景可被 Tab 穿透与滚动。全屏墨色底层与进出场由沉浸路由壳提供,
 * 本组件填满该壳(不自行 fixed 定位),避免两处争夺同一块全屏层。
 */
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  PanelLeftClose,
  PanelRightClose,
} from "lucide-react";
import { useCallback, useId, useState, type ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { useMediaQuery } from "../../hooks/useMediaQuery";

/** 横排断点:与 Tailwind lg(1024px)一致,禁止另造魔法断点 */
const DESKTOP_QUERY = "(min-width: 64rem)";

/** 折叠偏好键前缀(规范 §7.1:折叠状态本机持久化,键名 chaimir.workbench.*) */
const STORAGE_PREFIX = "chaimir.workbench.";

/**
 * 四个工作台的栏位构成各不相同(§7.1「栏位构成按各台主体而定」):
 * 仿真右侧是消息流,代码实验右侧是检查点判分,解题赛右侧是提交与榜单,对抗回放右侧是榜单。
 * 折叠偏好因此必须按台各存一份 —— 一处收起处处收起等于把四台当成同一台。
 */
export type WorkbenchId = "experiment" | "sim" | "replay" | "contest";

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

/**
 * storageKey:某台某一侧的折叠偏好键。
 * 键里同时带工作台标识与侧位,四台互不影响(见 WorkbenchId)。
 */
function storageKey(workbench: WorkbenchId, side: "left" | "right"): string {
  return `${STORAGE_PREFIX}${workbench}.${side}_collapsed`;
}

/**
 * readStoredCollapsed:读取用户显式收放过的偏好。
 * 返回 undefined 表示「没表达过」—— 那时按视口给默认值,并随视口变化,
 * 免得窄屏默认收起的状态被带进宽屏后一直是两条把手。
 */
function readStoredCollapsed(key: string): boolean | undefined {
  try {
    const raw = window.localStorage.getItem(key);
    if (raw === "true") return true;
    if (raw === "false") return false;
    return undefined;
  } catch (error) {
    // 折叠偏好不是业务草稿;存储被策略禁用时按当前视口默认值继续可用。
    console.warn("workbench_preference_read_failed", {
      operation: "read_collapsed_preference",
      error: { kind: error instanceof Error ? error.name : typeof error },
    });
    return undefined;
  }
}

/** writeStoredCollapsed 持久化可选布局偏好,存储不可用不影响当前交互。 */
function writeStoredCollapsed(key: string, collapsed: boolean): void {
  try {
    window.localStorage.setItem(key, String(collapsed));
  } catch (error) {
    console.warn("workbench_preference_write_failed", {
      operation: "write_collapsed_preference",
      error: { kind: error instanceof Error ? error.name : typeof error },
    });
  }
}

interface WorkbenchPanelProps {
  workbench: WorkbenchId;
  side: "left" | "right";
  /** 面板名:展开时在头部,收起时竖排在把手上 */
  label: string;
  /** 内容条数(如消息数);收起后按它算未读徽标 */
  count?: number;
  /** 视口是否达到横排断点 */
  wide: boolean;
  children: ReactNode;
}

/**
 * WorkbenchPanel(内部):一侧的可收起面板。
 * 宽屏收起成 --wb-rail-w 宽的竖向把手(面板名竖排 + 未读徽标),展开为令牌栏宽
 * (左 --wb-aside-w / 右 --wb-stream-w:令牌名取自仿真台的典型角色,值即左右两栏的宽度);
 * 窄屏收起成舞台上/下的一条标题栏,展开为限高区(舞台优先)。
 * 内容区是定高 flex 列且不自带滚动:滚动归内容自己(见文件头)。
 */
function WorkbenchPanel({ workbench, side, label, count, wide, children }: WorkbenchPanelProps) {
  const key = storageKey(workbench, side);
  const [choice, setChoice] = useState<boolean | undefined>(() => readStoredCollapsed(key));
  // 收起时的条数基线:之后新增多少条就是把手上的未读数
  const [seen, setSeen] = useState(count ?? 0);
  const contentId = useId();
  const collapsed = choice ?? !wide;
  const unread = collapsed && count !== undefined ? Math.max(0, count - seen) : 0;

  const toggle = useCallback(() => {
    const next = !collapsed;
    if (next) setSeen(count ?? 0);
    setChoice(next);
    writeStoredCollapsed(key, next);
  }, [collapsed, count, key]);

  const isLeft = side === "left";
  const edgeClass = wide
    ? isLeft
      ? "border-r border-dark-line"
      : "border-l border-dark-line"
    : isLeft
      ? "border-b border-dark-line"
      : "border-t border-dark-line";

  // 宽屏收起态:整条把手就是那个按钮,面板名竖排,未读数贴在名字前面
  if (wide && collapsed) {
    return (
      <aside className={cn("flex w-wb-rail shrink-0 flex-col bg-dark-bg", edgeClass)}>
        <button
          type="button"
          aria-expanded={false}
          aria-controls={contentId}
          onClick={toggle}
          // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
          className="pressable flex h-full w-full flex-col items-center gap-2 py-3 text-on-dark-sub hover:bg-dark-surface hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
        >
          <Icon icon={isLeft ? ChevronRight : ChevronLeft} size="sm" className="shrink-0" />
          {unread > 0 ? (
            <span className="shrink-0 rounded-full bg-accent px-1.5 font-mono text-xs tabular-nums text-substrate">
              {unread}
            </span>
          ) : null}
          <span className="text-vertical min-h-0 flex-1 overflow-hidden text-xs">{label}</span>
        </button>
        <div id={contentId} hidden>
          {children}
        </div>
      </aside>
    );
  }

  return (
    <aside
      className={cn(
        "flex min-h-0 shrink-0 flex-col bg-dark-bg",
        edgeClass,
        wide && (isLeft ? "w-wb-aside" : "w-wb-stream"),
      )}
    >
      <div className="flex shrink-0 items-center justify-between gap-2 px-3 py-1.5">
        <span className="min-w-0 truncate text-xs font-medium text-on-dark-sub">{label}</span>
        {unread > 0 ? (
          <span className="shrink-0 rounded-full bg-accent px-1.5 font-mono text-xs tabular-nums text-substrate">
            {unread}
          </span>
        ) : null}
        <button
          type="button"
          aria-expanded={!collapsed}
          aria-controls={contentId}
          onClick={toggle}
          // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
          className="pressable hit-target relative shrink-0 rounded-sm text-on-dark-sub hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
        >
          <span className="sr-only">{collapsed ? `展开${label}` : `收起${label}`}</span>
          <Icon
            icon={wide ? (isLeft ? PanelLeftClose : PanelRightClose) : collapsed ? panelOpenIcon(isLeft) : panelCloseIcon(isLeft)}
            size="sm"
          />
        </button>
      </div>
      <div
        id={contentId}
        hidden={collapsed}
        className={cn(
          "flex min-h-0 flex-col",
          // 窄屏给限高,保证舞台仍有空间;宽屏吃满剩余高度
          wide ? "flex-1" : "max-h-52",
        )}
      >
        {children}
      </div>
    </aside>
  );
}

/** 窄屏折叠方向:上方面板向上收、下方面板向下收,箭头指向内容去处 */
function panelCloseIcon(isLeft: boolean) {
  return isLeft ? ChevronUp : ChevronDown;
}

/** 窄屏展开方向:与收起相反 */
function panelOpenIcon(isLeft: boolean) {
  return isLeft ? ChevronDown : ChevronUp;
}

/* ---------------------------------------------------------------- 壳 */

export interface WorkbenchShellProps {
  /**
   * 工作台标识:折叠偏好按台分开存(§7.1 四台栏位构成不同,不能一处收起处处收起)。
   * 必填 —— 没有它就只能共用一份偏好,那正是把四台当成同一台。
   */
  workbench: WorkbenchId;
  /** 顶部工作台条插槽(通常放 WorkbenchTopbar) */
  topbar: ReactNode;
  /** 左辅助区:该台的解释与操作(各台内容不同,壳不规定) */
  left?: ReactNode;
  /** 中间主舞台(必填,占满剩余空间,永不滚动) */
  stage: ReactNode;
  /** 右栏:仿真/回放是事件流,代码实验是检查点判分,解题赛是提交与榜单;不需要就不传 */
  right?: ReactNode;
  /** 底部控制区(如仿真播放控制) */
  footer?: ReactNode;
  /** 左辅助区名称,默认「说明与操作」 */
  leftLabel?: string;
  /** 右栏名称,默认「消息流」;非事件流的台必须显式传自己的名字 */
  rightLabel?: string;
  /** 右栏内容条数;收起时壳按它在把手上给未读计数(不传则不显示徽标) */
  rightCount?: number;
  className?: string;
}

/**
 * WorkbenchShell:沉浸工作台外壳 —— 纵向 topbar / 三区 / footer。
 * 壳本身与主舞台都不产生滚动条,滚动只发生在槽位内容自己声明的容器里。
 * 壳只统一深色令牌、顶条契约、折叠行为、窄屏堆叠与无障碍;哪一侧放什么由该台的主体决定。
 */
export function WorkbenchShell({
  workbench,
  topbar,
  left,
  stage,
  right,
  footer,
  leftLabel = "说明与操作",
  rightLabel = "消息流",
  rightCount,
  className,
}: WorkbenchShellProps) {
  const wide = useMediaQuery(DESKTOP_QUERY);

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col bg-substrate text-on-dark", className)}>
      {topbar}
      {/* 三区:宽屏横排(辅助区-舞台-右栏),窄屏纵排,舞台始终 flex-1 优先 */}
      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        {left && (
          <WorkbenchPanel workbench={workbench} side="left" label={leftLabel} wide={wide}>
            {left}
          </WorkbenchPanel>
        )}
        <main className="flex min-h-0 min-w-0 flex-1 flex-col">{stage}</main>
        {right && (
          <WorkbenchPanel
            workbench={workbench}
            side="right"
            label={rightLabel}
            count={rightCount}
            wide={wide}
          >
            {right}
          </WorkbenchPanel>
        )}
      </div>
      {footer && <div className="shrink-0 border-t border-dark-line bg-dark-bg">{footer}</div>}
    </div>
  );
}
