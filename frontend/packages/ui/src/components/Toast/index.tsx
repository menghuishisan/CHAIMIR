/**
 * Toast:全局轻提示(sonner 封装)。
 * Toaster 在应用根部挂载一次;toast 直接复用 sonner API(成功用 toast.success,失败用 toast.error)。
 * 错误提示只放用户向文案,有报障编号时作为 description 附上(CLAUDE.md §8)。
 * 无障碍:aria-live(polite)区域由 sonner 内建保证,无需额外处理。
 */
import { Toaster as SonnerToaster, toast } from "sonner";

// re-export:业务侧统一从本组件取 toast,不直接依赖 sonner
export { toast };

/**
 * Toaster:品牌化 toast 容器。
 * unstyled + token 类名接管全部视觉(零裸值);右上角堆叠、间距 10。
 * 浮层可以用发丝边把自己从未知背景上分离(规范 §6.5.1)。
 */
export function Toaster() {
  return (
    <SonnerToaster
      position="top-right"
      gap={10}
      toastOptions={{
        unstyled: true,
        classNames: {
          toast:
            "flex w-full items-center gap-3 rounded-lg border border-line bg-surface p-4 text-sm text-ink shadow-lg",
          content: "flex min-w-0 flex-col",
          title: "font-medium",
          description: "mt-1 text-xs text-ink-sub",
          icon: "shrink-0",
        },
      }}
    />
  );
}
