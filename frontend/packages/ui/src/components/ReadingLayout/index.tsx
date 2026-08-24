/**
 * ReadingLayout:目录 + 限宽正文 + 侧栏(规范 §6.5.3 第 ⑦ 族「学习阅读」)。
 *
 * 这一族的任务是**读**,不是比对,所以它是八族里唯一需要限宽的:
 * 行太长时眼睛回到下一行的行首会找错位置。限宽写在这里而不做成全局令牌 ——
 * `--content-max` 已废止(§2.4),其余七族要的是铺满纸宽,把限宽提到全局会把它们一起锁住。
 *
 * 响应式(§6.4.1 规则 4 同类处理):`≥lg` 三栏;`<lg` 目录收成顶部抽屉触发条
 * (当前位置常显)、侧栏沉为底部常驻条。此时正文**不需要**限宽 ——
 * 视口本身就在 40 字/行以内,再限宽只会两侧再挖一刀。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface ReadingLayoutProps {
  /**
   * 目录/章节树。`≥lg` 为左栏常驻;`<lg` 由调用方改传「当前位置 + 展开触发」的一条,
   * 全屏目录走 Drawer —— 抽屉的焦点陷阱与 Esc 由 Drawer 组件保证,不在此重复实现。
   */
  toc?: ReactNode;
  /** 正文 */
  children: ReactNode;
  /** 右侧进度与本节动作。`<lg` 沉到底部常驻条 */
  aside?: ReactNode;
  className?: string;
}

export function ReadingLayout({ toc, children, aside, className }: ReadingLayoutProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:gap-6", className)}>
      {toc && (
        // ≥lg 左栏常驻并吸顶(top 取顶栏高度令牌);<lg 由调用方给一条触发器,自然排在正文之上
        <aside
          aria-label="本章目录"
          className="min-w-0 lg:sticky lg:w-60 lg:shrink-0 lg:self-start"
          style={{ top: "var(--topnav-h)" }}
        >
          {toc}
        </aside>
      )}

      {/* 正文:居中限宽。max-w-prose 是 Tailwind 的排版刻度(≈65 字符),
          中文按全角折算约 40 字/行,落在规范 60–75 字符区间内 */}
      <div className="flex min-w-0 flex-1 justify-center">
        <div className="min-w-0 flex-1 lg:max-w-prose">{children}</div>
      </div>

      {aside && (
        <aside
          aria-label="学习进度与本节动作"
          className="min-w-0 lg:sticky lg:w-60 lg:shrink-0 lg:self-start"
          style={{ top: "var(--topnav-h)" }}
        >
          {aside}
        </aside>
      )}
    </div>
  );
}
