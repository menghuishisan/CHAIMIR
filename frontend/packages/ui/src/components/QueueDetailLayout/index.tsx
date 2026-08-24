/**
 * QueueDetailLayout:队列 + 详情双栏骨架(规范 §6.5.3 第 ⑤ 族「审阅队列」)。
 *
 * 这一族的动作不是「找一条记录」,是「逐条处理完一批」——批改、成绩审核、申诉、
 * 实验报告、作弊复核。所以左边常驻队列、右边是当前条的详情与处理表单,
 * 处理完直接进下一条,视线不必在列表与详情之间来回跳。指标带在这一族没有位置:
 * 待办数量放标题下一行就够,它是上下文而不是主体。
 *
 * 响应式(§6.4.1 规则 4):`≥lg` 两栏并排;`<lg` 换成**两级页面**而不是压扁 ——
 * 1024 以下两栏各自都会窄到没法用。两级页面的当前层由调用方用 `view` 控制,
 * 组件不持有导航状态:那是路由/页面的职责,藏进布局组件会让「返回」的行为分散在两处。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface QueueDetailLayoutProps {
  /** 队列侧内容:筛选 + 条目列表(通常包在一块抬起片里) */
  queue: ReactNode;
  /** 详情侧内容:当前条的正文与处理表单 */
  detail: ReactNode;
  /**
   * 详情侧底部动作条(通过/驳回/保存并看下一条)。
   * `<lg` 时钉在屏幕底部,拇指可达;`≥lg` 贴在详情片底边。
   */
  detailActions?: ReactNode;
  /**
   * `<lg` 当前显示哪一级:'queue' 只显示队列、'detail' 只显示详情。
   * `≥lg` 忽略此值,两栏始终并排。
   */
  view?: "queue" | "detail";
  /**
   * `<lg` 详情态的顶部条:返回按钮 + 当前对象 + `第 n 条 / 共 m 条`。
   * `≥lg` 不渲染 —— 那时队列就在旁边,不需要返回也不需要位置指示。
   */
  detailHeader?: ReactNode;
  /** 追加类;队列侧默认宽度为 `lg:w-84`(21rem 标准刻度),需要更宽时在此覆盖 */
  className?: string;
}

export function QueueDetailLayout({
  queue,
  detail,
  detailActions,
  view = "queue",
  detailHeader,
  className,
}: QueueDetailLayoutProps) {
  const showQueue = view === "queue";
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col gap-4 lg:flex-row", className)}>
      {/* 队列侧:≥lg 固定宽常驻;<lg 仅在队列层显示 */}
      <section
        aria-label="待处理队列"
        className={cn(
          "flex min-w-0 flex-col lg:w-84 lg:shrink-0",
          !showQueue && "hidden lg:flex",
        )}
      >
        {queue}
      </section>

      {/* 详情侧:<lg 仅在详情层显示 */}
      <section
        aria-label="当前条目详情"
        className={cn(
          "flex min-w-0 flex-1 flex-col gap-3",
          showQueue && "hidden lg:flex",
        )}
      >
        {detailHeader && <div className="lg:hidden">{detailHeader}</div>}
        <div className="flex min-h-0 flex-1 flex-col">{detail}</div>
        {detailActions && (
          // <lg 钉屏幕底部(sticky bottom-0 + 令牌层级);≥lg 随详情片自然收尾
          <div className="sticky bottom-0 z-sticky rounded-lg bg-surface px-4 py-3 shadow-md lg:static lg:z-auto lg:rounded-none lg:bg-transparent lg:px-0 lg:py-0 lg:shadow-none">
            {detailActions}
          </div>
        )}
      </section>
    </div>
  );
}
