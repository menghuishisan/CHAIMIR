/**
 * ReadingLayout:课时阅读布局 —— 目录 + 限宽正文(设计 §7.2 课时学习)。
 *
 * 这一页的任务是读,所以正文限宽:行太长时眼睛回到下一行的行首会找错位置。
 * 设计要求图文每行约 40 个汉字:正文 15px × 40 = 600px,再加正文卡片左右各 20px 内边距,
 * 栏宽取 max-w-160(40rem = 640px)。限宽写在这里而不做成全局令牌 —— 其余页面要的是铺满纸宽。
 *
 * 响应式:≥xl(1280)且传了 toc 时左侧目录常驻并吸顶;更窄时不渲染目录栏,
 * 由调用方在正文顶部给一个「课程目录」按钮打开抽屉(抽屉的焦点陷阱与 Esc 由 Drawer 保证)。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface ReadingLayoutProps {
  /** 目录。只在 ≥xl 作为左栏渲染;不传表示目录已收起 */
  toc?: ReactNode;
  /** 正文 */
  children: ReactNode;
  className?: string;
}

/** ReadingLayout 渲染可选的左侧目录与限宽正文 */
export function ReadingLayout({ toc, children, className }: ReadingLayoutProps) {
  return (
    <div className={cn("flex min-h-0 flex-1 flex-col gap-6 xl:flex-row xl:gap-8", className)}>
      {toc && (
        // ≥xl 左栏常驻并吸顶(顶栏之下);更窄时不显示,目录走抽屉
        <aside
          aria-label="课程目录"
          className="hidden min-w-0 xl:sticky xl:top-6 xl:block xl:w-68 xl:shrink-0 xl:self-start"
        >
          {toc}
        </aside>
      )}
      <div className="flex min-w-0 flex-1 justify-center">
        <div className="w-full min-w-0 max-w-160">{children}</div>
      </div>
    </div>
  );
}
