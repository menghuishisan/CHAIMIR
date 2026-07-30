/**
 * Menu:下拉菜单(@radix-ui/react-dropdown-menu 组合封装)。
 * 组合导出:Menu(Root)/ MenuTrigger / MenuContent / MenuItem / MenuLabel / MenuSeparator。
 * 键盘导航、typeahead、焦点管理由 Radix 保证。
 * 动效:pop-in / pop-out,transform-origin 对齐触发点(Radix 变量);
 * 定位 transform 由 Radix popper wrapper 独占,动画只写在 Content(§4.3)。
 */
import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import type { LucideIcon } from "lucide-react";
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export const Menu = DropdownMenuPrimitive.Root;
export const MenuTrigger = DropdownMenuPrimitive.Trigger;

export interface MenuContentProps
  extends ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Content> {}

export const MenuContent = forwardRef<
  ElementRef<typeof DropdownMenuPrimitive.Content>,
  MenuContentProps
>(({ className, style, sideOffset = 6, ...props }, ref) => (
  <DropdownMenuPrimitive.Portal>
    <DropdownMenuPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      className={cn(
        "z-dropdown min-w-44 rounded-lg border border-line bg-surface p-1 shadow-md",
        "animate-pop-in data-[state=closed]:animate-pop-out",
        className,
      )}
      /* 动态定位值:缩放原点跟随触发源(Radix 计算),属定位计算允许内联 */
      style={{ transformOrigin: "var(--radix-dropdown-menu-content-transform-origin)", ...style }}
      {...props}
    />
  </DropdownMenuPrimitive.Portal>
));
MenuContent.displayName = "MenuContent";

export interface MenuItemProps
  extends ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Item> {
  /** 项前置图标(Lucide),统一经 Icon 封装渲染 */
  icon?: LucideIcon;
  /**
   * 危险项:删除/登出等破坏性操作。冷红(danger)呈现;
   * 使用时必须与普通项之间用 MenuSeparator 分隔成组,防止误触(§5.1)。
   */
  danger?: boolean;
}

export const MenuItem = forwardRef<ElementRef<typeof DropdownMenuPrimitive.Item>, MenuItemProps>(
  ({ icon, danger = false, className, children, ...props }, ref) => (
    <DropdownMenuPrimitive.Item
      ref={ref}
      className={cn(
        "flex cursor-default select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-sm",
        // 焦点反馈用高亮底色承担;outline-hidden 保留强制对比色模式下的系统焦点可见性
        "outline-hidden transition-colors duration-fast",
        "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        danger
          ? "text-danger data-[highlighted]:bg-danger-bg"
          : "text-ink data-[highlighted]:bg-surface-hover",
        className,
      )}
      {...props}
    >
      {icon ? <Icon icon={icon} size="sm" /> : null}
      {children}
    </DropdownMenuPrimitive.Item>
  ),
);
MenuItem.displayName = "MenuItem";

/** MenuLabel:分组标签,不可交互 */
export const MenuLabel = forwardRef<
  ElementRef<typeof DropdownMenuPrimitive.Label>,
  ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Label>
>(({ className, ...props }, ref) => (
  <DropdownMenuPrimitive.Label
    ref={ref}
    className={cn("px-2.5 py-1.5 text-xs text-ink-sub", className)}
    {...props}
  />
));
MenuLabel.displayName = "MenuLabel";

/** MenuSeparator:分组分隔线;危险项组与普通项组之间必用 */
export const MenuSeparator = forwardRef<
  ElementRef<typeof DropdownMenuPrimitive.Separator>,
  ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Separator>
>(({ className, ...props }, ref) => (
  <DropdownMenuPrimitive.Separator
    ref={ref}
    className={cn("my-1 h-px bg-line", className)}
    {...props}
  />
));
MenuSeparator.displayName = "MenuSeparator";
