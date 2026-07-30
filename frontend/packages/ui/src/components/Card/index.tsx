/**
 * Card:光面卡片容器(§5.1)。
 * Card 静置为宣纸面 + 细边 + 最低海拔;interactive 时整卡可点可聚焦(Enter/Space 触发 onPress),
 * hover 抬升海拔、按压走 pressable 反馈。CardHeader/CardBody/CardFooter 提供统一的卡内排版。
 * 交互卡内的交互子元素(按钮/链接/输入等)会被事件守卫排除,不会触发整卡动作。
 */
import type { HTMLAttributes, KeyboardEvent, MouseEvent, ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  /** 可交互卡:整卡可点、可聚焦、可回车/空格触发 */
  interactive?: boolean;
  /** 交互卡的触发回调(点击或键盘 Enter/Space) */
  onPress?: () => void;
}

export function Card({
  interactive = false,
  onPress,
  className,
  onClick,
  onKeyDown,
  children,
  ...rest
}: CardProps) {
  // 交互元素守卫:事件源自卡内按钮/链接/输入等交互子元素时,整卡动作不响应,
  // 防止子元素操作冒泡误触 onPress、防止输入框内空格被 preventDefault 吞掉。
  // 注意与 currentTarget 比较的是 closest 命中结果:交互卡自身带 role="button",
  // 普通文本子元素向上找会命中卡本身,此时不应拦截整卡动作
  const isFromInteractiveChild = (event: MouseEvent<HTMLDivElement> | KeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    const hit = target.closest(
      'button, a, input, select, textarea, [role="button"], [role="menuitem"]',
    );
    return hit !== null && hit !== event.currentTarget;
  };

  // 交互卡:点击与键盘触发统一走 onPress,不吞消费方自带的 onClick/onKeyDown
  const handleClick = (event: MouseEvent<HTMLDivElement>) => {
    if (isFromInteractiveChild(event)) return;
    onClick?.(event);
    onPress?.();
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (isFromInteractiveChild(event)) return;
    onKeyDown?.(event);
    if (event.key === "Enter" || event.key === " ") {
      // 空格默认会滚动页面,需阻止(仅守卫通过后才阻止,不影响卡内输入框)
      event.preventDefault();
      onPress?.();
    }
  };

  return (
    <div
      {...rest}
      className={cn(
        "bg-surface border border-line rounded-lg shadow-xs",
        // pressable 已含 box-shadow 过渡,不再叠加 transition-shadow
        interactive &&
          "pressable cursor-pointer hover:shadow-md hover:border-line-strong focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2",
        className,
      )}
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      onClick={interactive ? handleClick : onClick}
      onKeyDown={interactive ? handleKeyDown : onKeyDown}
    >
      {children}
    </div>
  );
}

export interface CardHeaderProps {
  /** 卡头标题 */
  title: ReactNode;
  /** 标题下的辅助说明 */
  description?: ReactNode;
  /** 右侧操作区插槽(按钮/菜单等) */
  actions?: ReactNode;
  className?: string;
}

/** 卡头:标题 + 说明 + 右侧操作区,下边线与卡体分隔 */
export function CardHeader({ title, description, actions, className }: CardHeaderProps) {
  return (
    <div
      className={cn(
        "flex items-start justify-between gap-4 border-b border-line px-5 py-4",
        className,
      )}
    >
      <div className="min-w-0">
        <div className="text-md font-medium text-ink">{title}</div>
        {description !== undefined && description !== null && (
          <div className="mt-0.5 text-sm text-ink-sub">{description}</div>
        )}
      </div>
      {actions !== undefined && actions !== null && (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      )}
    </div>
  );
}

export interface CardBodyProps extends HTMLAttributes<HTMLDivElement> {}

/** 卡体:标准内边距的内容区 */
export function CardBody({ className, ...rest }: CardBodyProps) {
  return <div {...rest} className={cn("p-5", className)} />;
}

export interface CardFooterProps extends HTMLAttributes<HTMLDivElement> {}

/** 卡脚:上边线分隔,默认操作靠右排布 */
export function CardFooter({ className, ...rest }: CardFooterProps) {
  return (
    <div
      {...rest}
      className={cn("flex items-center justify-end gap-3 border-t border-line px-5 py-4", className)}
    />
  );
}
