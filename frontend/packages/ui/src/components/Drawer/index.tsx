/**
 * Drawer:抽屉/底部 Sheet(@radix-ui/react-dialog 变体封装)。
 * side=right(默认):右侧全高抽屉,承载详情/编辑等次级流程;
 * side=left:左侧全高抽屉,窄屏导航专用(应用壳侧栏抽屉化,§6.4);
 * side=bottom:窄屏底部 Sheet,顶部圆角,高度随内容、上限 88vh。
 * tone=light(默认)宣纸光面;tone=dark 墨色面板(导航抽屉与底层同世界)。
 * 组合导出:Drawer(Root)/ DrawerTrigger / DrawerContent(side/tone)/ DrawerHeader /
 * DrawerTitle / DrawerDescription / DrawerBody / DrawerFooter / DrawerClose。
 * 焦点陷阱、Esc、遮罩关闭由 Radix 保证;动效 slide-in/out(出场短于入场),
 * 位移动画独占 Content 的 transform,定位用 inset,不冲突(§4.3)。
 */
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { cva, type VariantProps } from "class-variance-authority";
import { X } from "lucide-react";
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ElementRef,
  type HTMLAttributes,
} from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export const Drawer = DialogPrimitive.Root;
export const DrawerTrigger = DialogPrimitive.Trigger;
export const DrawerClose = DialogPrimitive.Close;

/* 方位 × 色调变体:right/left 全高侧滑,bottom 底部上滑(窄屏 sheet) */
const drawerContentVariants = cva("fixed z-drawer flex flex-col shadow-lg", {
  variants: {
    side: {
      right: cn(
        "inset-y-0 right-0 h-full w-full max-w-lg",
        "data-[state=open]:animate-slide-in-right data-[state=closed]:animate-slide-out-right",
      ),
      left: cn(
        "inset-y-0 left-0 h-full w-full max-w-xs",
        "data-[state=open]:animate-slide-in-left data-[state=closed]:animate-slide-out-left",
      ),
      bottom: cn(
        "inset-x-0 bottom-0 w-full rounded-t-pane",
        "data-[state=open]:animate-slide-in-bottom data-[state=closed]:animate-slide-out-bottom",
      ),
    },
    tone: {
      light: "bg-surface text-ink",
      dark: "bg-dark-bg text-on-dark",
    },
  },
  defaultVariants: { side: "right", tone: "light" },
});

/* 关闭按钮双色调:命中区经 hit-target 扩到 44×44(§3.2) */
const drawerCloseVariants = cva(
  cn(
    "hit-target absolute right-4 top-4 rounded-md p-1.5 transition-colors duration-fast",
    "focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2",
  ),
  {
    variants: {
      tone: {
        light: "text-ink-sub hover:bg-surface-hover hover:text-ink",
        dark: "text-on-dark-sub hover:bg-dark-elevated hover:text-on-dark",
      },
    },
    defaultVariants: { tone: "light" },
  },
);

export interface DrawerContentProps
  extends ComponentPropsWithoutRef<typeof DialogPrimitive.Content>,
    VariantProps<typeof drawerContentVariants> {}

/**
 * DrawerContent:遮罩 + 抽屉面板 + 右上角关闭按钮。
 * bottom 变体限高 88vh 防占满全屏,超出由 DrawerBody 内滚(尺寸计算,允许内联)。
 */
export const DrawerContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  DrawerContentProps
>(({ side, tone, className, style, children, ...props }, ref) => {
  const resolvedSide = side ?? "right";
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-drawer bg-substrate/60 animate-fade-in data-[state=closed]:animate-fade-out" />
      <DialogPrimitive.Content
        ref={ref}
        className={cn(drawerContentVariants({ side: resolvedSide, tone }), className)}
        style={resolvedSide === "bottom" ? { maxHeight: "88vh", ...style } : style}
        {...props}
      >
        {children}
        {/* 右上角关闭:纯图标按钮,aria-label 必填(FE 契约) */}
        <DialogPrimitive.Close aria-label="关闭" className={drawerCloseVariants({ tone })}>
          <Icon icon={X} size="md" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
});
DrawerContent.displayName = "DrawerContent";

/** DrawerHeader:标题区;右侧预留关闭按钮空间。边线色随 tone 由调用方按需覆写 */
export function DrawerHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex shrink-0 flex-col gap-1 border-b border-line px-6 py-4 pr-12", className)}
      {...props}
    />
  );
}

export const DrawerTitle = forwardRef<
  ElementRef<typeof DialogPrimitive.Title>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title
    ref={ref}
    className={cn("text-lg font-semibold", className)}
    {...props}
  />
));
DrawerTitle.displayName = "DrawerTitle";

export const DrawerDescription = forwardRef<
  ElementRef<typeof DialogPrimitive.Description>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Description
    ref={ref}
    className={cn("text-sm text-ink-sub", className)}
    {...props}
  />
));
DrawerDescription.displayName = "DrawerDescription";

/** DrawerBody:主体内容区,超高时在此内滚 */
export function DrawerBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("grow overflow-y-auto px-6 py-4", className)} {...props} />;
}

/** DrawerFooter:操作按钮区,右对齐 */
export function DrawerFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-end gap-3 border-t border-line px-6 py-4",
        className,
      )}
      {...props}
    />
  );
}
