/**
 * EventTimeline:按天分组的事件轴(规范 §6.5.3 第 ⑥ 族「时间流」)。
 *
 * 通知收件箱、审计日志、系统告警、学业预警的读法是「从新到旧扫一遍」,
 * 不是「按列比对」。等宽多列表格逼读者横向读六个字段才拼出一件事,
 * 而事件本身的结构是:什么时候、发生了什么(一句人话)、技术细节(一行)。
 * 所以改成时间轴 —— 时间左对齐成一列、状态点作锚、主文与细节纵向分两级。
 *
 * 日期分组头用凹陷井色:它是流里的分节标记而不是一个条目,必须与条目区分开。
 * 本组件用在 DataPanel 内部(筛选井 + 事件轴 + 加载更多同处一块抬起片)。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

/** 事件语义:决定状态点颜色**与形状**,不靠颜色单一传达(FE-2) */
export type EventTone = "normal" | "success" | "warning" | "danger";

export interface TimelineEvent {
  id: string;
  /** 时刻,如 `09:48`;等宽显示保证纵向对齐 */
  time: string;
  /** 主文:一句用户向的话,说清发生了什么(§8 文案规范) */
  title: ReactNode;
  /** 技术细节一行:trace_id、IP、前后取值等;可选 */
  detail?: ReactNode;
  tone?: EventTone;
  /** 行尾动作槽位,如「标记已读」「查看详情」 */
  action?: ReactNode;
}

export interface TimelineDay {
  /** 分组标题,如「今天 · 2026/08/22」 */
  label: string;
  events: TimelineEvent[];
}

export interface EventTimelineProps {
  /** 无障碍名称,如「审计记录」 */
  label: string;
  days: TimelineDay[];
  className?: string;
}

/**
 * tone → 状态点的形状与颜色类。形状是主要载体、颜色是辅助(FE-2 色非唯一):
 * normal 小圆、success 实心圆、warning 三角、danger 方块。
 * 用 SVG 而非 border 构形:三角用 border 技巧必须写出带方括号的任意值类(违反 FE-1),
 * SVG 里形状由路径表达,颜色走 currentColor 由令牌类给。
 */
const DOT_SHAPE: Record<EventTone, { path: ReactNode; color: string }> = {
  normal: { path: <circle cx="5" cy="5" r="2.5" />, color: "text-ink-faint" },
  success: { path: <circle cx="5" cy="5" r="4" />, color: "text-success" },
  warning: { path: <path d="M5 1 l4.5 8 h-9 Z" />, color: "text-warning" },
  danger: { path: <rect x="1.5" y="1.5" width="7" height="7" rx="1.5" />, color: "text-danger" },
};

/** EventDot 渲染一个状态点:语义已在主文里,故对读屏隐藏 */
function EventDot({ tone }: { tone: EventTone }) {
  const shape = DOT_SHAPE[tone];
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 10 10"
      className={cn("mt-1.5 size-2.5 shrink-0 fill-current", shape.color)}
    >
      {shape.path}
    </svg>
  );
}

export function EventTimeline({ label, days, className }: EventTimelineProps) {
  return (
    <div aria-label={label} className={cn("min-w-0", className)}>
      {days.map((day) => (
        <section key={day.label} aria-label={day.label}>
          {/*
            分节标记:凹陷井色横贯一条,与条目区分。
            不做 sticky —— 本组件的落点是 `DataPanel`,而那层为了收住圆角带 `overflow-hidden`,
            这会建立一个不可滚动的 scrollport,里面的 sticky 静默失效。
            要做粘性日期头,得让事件轴自己拥有滚动容器,那是另一个决定。
          */}
          <h3 className="bg-surface-sunken px-4 py-1.5 font-mono text-xs text-ink-sub">
            {day.label}
          </h3>
          <ol>
            {day.events.map((event) => (
              <li
                key={event.id}
                className="flex items-start gap-3 border-b border-line px-4 py-3 last:border-b-0"
              >
                <time className="w-11 shrink-0 pt-0.5 font-mono text-xs text-ink-faint">
                  {event.time}
                </time>
                {/* 状态点:形状 + 色双载体 */}
                <EventDot tone={event.tone ?? "normal"} />
                <div className="min-w-0 flex-1">
                  <div className="text-sm text-ink">{event.title}</div>
                  {event.detail && (
                    <div className="mt-0.5 break-all font-mono text-xs text-ink-sub">
                      {event.detail}
                    </div>
                  )}
                </div>
                {event.action && <div className="shrink-0">{event.action}</div>}
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}
