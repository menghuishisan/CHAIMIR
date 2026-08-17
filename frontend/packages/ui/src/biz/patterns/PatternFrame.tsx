/**
 * PatternFrame:七种模式渲染器的统一外框,把「主舞台永不滚动」这条规矩落在一处(§7.1)。
 *
 * stage 密度(主舞台):图形区取剩余高度并按 viewBox 等比缩放,文本清单退到高度受限的一条带里
 * 自行滚动 —— 图形是主体,不能被增长的清单顶出视口。没有图形的模式(矩阵/流程/树)则由
 * 清单本身占满舞台高度。
 * panel 密度(辅助区手风琴内):不做高度分配,由外层手风琴决定可见高度。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import type { PatternDensity } from "../frameVisual";

export interface PatternFrameProps {
  density: PatternDensity;
  /** 图形区(SVG/图表);纯清单类模式不传 */
  canvas?: ReactNode;
  /** 文本清单区(元素清单/表格/步骤),画布的文本替代 */
  children?: ReactNode;
}

export function PatternFrame({ density, canvas, children }: PatternFrameProps) {
  if (density === "panel") {
    return (
      <div className="flex flex-col gap-3">
        {canvas}
        {children}
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      {canvas ? <div className="flex min-h-0 flex-1 flex-col justify-start">{canvas}</div> : null}
      {children ? (
        <div
          className={cn(
            "flex min-h-0 flex-col gap-3 overflow-y-auto",
            // 有图形时清单只占一条带(图形优先);没有图形时清单就是主体,占满剩余高度
            canvas ? "max-h-48 shrink" : "flex-1",
          )}
        >
          {children}
        </div>
      ) : null}
    </div>
  );
}
