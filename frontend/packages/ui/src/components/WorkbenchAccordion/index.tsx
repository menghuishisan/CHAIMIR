/**
 * WorkbenchAccordion:沉浸态左辅助区的手风琴阅读区(规范 §7.2 B)。
 *
 * 各段标题常驻可见,等于一份目录;一次只展开一段,展开的那段占满剩余高度并自行滚动 ——
 * 执行追踪、证据矩阵这类内容因此才读得清。折起的段不消失,学生随时知道还有什么可看。
 *
 * 只管「哪一段展开、展开的那段有多高」;段内画什么由页面填,壳与本组件都不认识业务内容。
 */
import { useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface WorkbenchAccordionSection {
  /** 段编号,用于展开态与 aria 关联 */
  id: string;
  /** 段标题(常驻可见) */
  title: string;
  /** 标题右侧的极简状态字(如「3/5 已达成」);颜色不承载信息 */
  hint?: string;
  content: ReactNode;
}

export interface WorkbenchAccordionProps {
  sections: WorkbenchAccordionSection[];
  /** 默认展开的段编号;不传则展开第一段 */
  defaultOpenId?: string;
  className?: string;
}

export function WorkbenchAccordion({ sections, defaultOpenId, className }: WorkbenchAccordionProps) {
  const [openId, setOpenId] = useState<string | undefined>(defaultOpenId ?? sections[0]?.id);
  // 段可能随帧变化而增减:展开的那段没了就退回第一段。
  // openId 为 undefined 是「用户主动全部折起」,那是合法状态,不能被这条回退撤销。
  const activeId =
    openId === undefined || sections.some((section) => section.id === openId)
      ? openId
      : sections[0]?.id;

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col", className)}>
      {sections.map((section) => {
        const open = section.id === activeId;
        return (
          <section
            key={section.id}
            className={cn(
              "flex flex-col border-b border-dark-line",
              // 展开的段吃掉剩余高度,其余只占标题行
              open ? "min-h-0 flex-1" : "shrink-0",
            )}
          >
            <h2 className="shrink-0">
              <button
                type="button"
                aria-expanded={open}
                aria-controls={`${section.id}-panel`}
                onClick={() => setOpenId(open ? undefined : section.id)}
                // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
                className="pressable flex w-full items-center gap-1.5 px-3 py-2 text-left hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
              >
                <Icon
                  icon={ChevronRight}
                  size="xs"
                  className={cn(
                    "shrink-0 text-on-dark-sub transition-transform duration-fast",
                    open && "rotate-90",
                  )}
                />
                <span
                  className={cn(
                    "min-w-0 flex-1 truncate text-sm",
                    open ? "font-medium text-on-dark" : "text-on-dark-sub",
                  )}
                >
                  {section.title}
                </span>
                {section.hint ? (
                  <span className="shrink-0 text-xs text-on-dark-sub">{section.hint}</span>
                ) : null}
              </button>
            </h2>
            <div
              id={`${section.id}-panel`}
              hidden={!open}
              className={cn("min-h-0 overflow-y-auto", open && "flex-1")}
            >
              {section.content}
            </div>
          </section>
        );
      })}
    </div>
  );
}
