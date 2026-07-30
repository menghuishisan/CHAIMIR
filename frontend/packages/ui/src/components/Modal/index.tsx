/**
 * Modal:居中模态对话框(@radix-ui/react-dialog 组合封装)。
 * 组合导出:Modal(Root)/ ModalTrigger / ModalContent(size)/ ModalHeader /
 * ModalTitle / ModalDescription / ModalBody / ModalFooter / ModalClose。
 * 焦点陷阱、Esc 关闭、点击遮罩关闭、标题/描述 aria 关联由 Radix 保证。
 * 动效:遮罩 fade-in/out,内容 modal-in / modal-out(出场时长为入场的 ~64%)。
 * transform 所有权(§4.3):居中用 inset + margin auto 实现,不占用 transform,
 * 动画 transform 独占 Content 本体,二者不冲突。
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

export const Modal = DialogPrimitive.Root;
export const ModalTrigger = DialogPrimitive.Trigger;
export const ModalClose = DialogPrimitive.Close;

/* 尺寸变体:只控制面板最大宽度;inset-x-4 保证窄屏两侧留白 */
const modalContentVariants = cva(
  cn(
    "fixed inset-x-4 inset-y-0 z-modal m-auto flex h-fit flex-col overflow-hidden",
    "rounded-pane bg-surface text-ink shadow-lg",
    "data-[state=open]:animate-modal-in data-[state=closed]:animate-modal-out",
  ),
  {
    variants: {
      size: {
        sm: "max-w-md",
        md: "max-w-lg",
        lg: "max-w-2xl",
        xl: "max-w-4xl",
      },
    },
    defaultVariants: { size: "md" },
  },
);

export interface ModalContentProps
  extends ComponentPropsWithoutRef<typeof DialogPrimitive.Content>,
    VariantProps<typeof modalContentVariants> {}

/**
 * ModalContent:遮罩 + 居中面板 + 右上角关闭按钮。
 * 面板高度上限 88vh,超出部分由 ModalBody 内滚(高度值属尺寸计算,允许内联)。
 */
export const ModalContent = forwardRef<
  ElementRef<typeof DialogPrimitive.Content>,
  ModalContentProps
>(({ size, className, style, children, ...props }, ref) => (
  <DialogPrimitive.Portal>
    <DialogPrimitive.Overlay className="fixed inset-0 z-modal bg-substrate/60 animate-fade-in data-[state=closed]:animate-fade-out" />
    <DialogPrimitive.Content
      ref={ref}
      className={cn(modalContentVariants({ size }), className)}
      style={{ maxHeight: "88vh", ...style }}
      {...props}
    >
      {children}
      {/* 右上角关闭:纯图标按钮,aria-label 必填(FE 契约);hit-target 命中区扩到 44×44(§3.2) */}
      <DialogPrimitive.Close
        aria-label="关闭"
        className={cn(
          "hit-target absolute right-4 top-4 rounded-md p-1.5 text-ink-sub transition-colors duration-fast",
          "hover:bg-surface-hover hover:text-ink",
          "focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2",
        )}
      >
        <Icon icon={X} size="md" />
      </DialogPrimitive.Close>
    </DialogPrimitive.Content>
  </DialogPrimitive.Portal>
));
ModalContent.displayName = "ModalContent";

/** ModalHeader:标题区;右侧预留关闭按钮空间 */
export function ModalHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("flex shrink-0 flex-col gap-1 border-b border-line px-6 py-4 pr-12", className)}
      {...props}
    />
  );
}

export const ModalTitle = forwardRef<
  ElementRef<typeof DialogPrimitive.Title>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title
    ref={ref}
    className={cn("text-lg font-semibold text-ink", className)}
    {...props}
  />
));
ModalTitle.displayName = "ModalTitle";

export const ModalDescription = forwardRef<
  ElementRef<typeof DialogPrimitive.Description>,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Description
    ref={ref}
    className={cn("text-sm text-ink-sub", className)}
    {...props}
  />
));
ModalDescription.displayName = "ModalDescription";

/** ModalBody:主体内容区,面板超高时在此内滚 */
export function ModalBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("grow overflow-y-auto px-6 py-4", className)} {...props} />;
}

/** ModalFooter:操作按钮区,右对齐 */
export function ModalFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
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
