/**
 * Tooltip:悬浮提示(@radix-ui/react-tooltip 封装)。
 * 深色小浮层,只用于解释控件用途(图标按钮、缩写、被截断文字等);
 * 不承载必读信息 —— 触屏与键盘/读屏用户可能触达不到,必读内容必须放常显文案。
 * 动效:pop-in / pop-out;transform-origin 对齐触发点(Radix 变量)。
 * transform 所有权(§4.3):定位 transform 由 Radix popper wrapper 独占,
 * 动画 transform 只写在 Content 本体,二者不同元素、互不冲突。
 */
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import type { ReactElement, ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface TooltipProviderProps {
  children: ReactNode;
  /** 首个 Tooltip 出现前的悬停延迟(毫秒),默认 300 */
  delayDuration?: number;
  /** 相邻 Tooltip 免延迟的时间窗(毫秒),默认 600:首个延迟、相邻即时 */
  skipDelayDuration?: number;
}

/**
 * TooltipProvider:应用级挂载一次,统一延迟策略 ——
 * 第一个 Tooltip 延迟出现避免误触,600ms 内移向相邻控件时即时显示。
 */
export function TooltipProvider({
  children,
  delayDuration = 300,
  skipDelayDuration = 600,
}: TooltipProviderProps) {
  return (
    <TooltipPrimitive.Provider delayDuration={delayDuration} skipDelayDuration={skipDelayDuration}>
      {children}
    </TooltipPrimitive.Provider>
  );
}

export interface TooltipProps {
  /** 提示内容:简短解释文字,不放必读信息与交互元素 */
  content: ReactNode;
  /** 出现方位,默认 top;空间不足时 Radix 自动翻转 */
  side?: "top" | "right" | "bottom" | "left";
  /** 触发器:必须是单个可接收 ref 的可聚焦元素(Radix asChild 注入,ReactNode 会运行时崩溃,故类型收紧为 ReactElement) */
  children: ReactElement;
  /** 追加到 Content 的类名 */
  className?: string;
}

export function Tooltip({ content, side = "top", children, className }: TooltipProps) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          side={side}
          sideOffset={6}
          className={cn(
            "z-dropdown rounded-md bg-ink px-2.5 py-1.5 text-xs text-canvas shadow-md",
            "animate-pop-in data-[state=closed]:animate-pop-out",
            className,
          )}
          /* 动态定位值:缩放原点跟随触发源(Radix 计算),属定位计算允许内联 */
          style={{ transformOrigin: "var(--radix-tooltip-content-transform-origin)" }}
        >
          {content}
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
