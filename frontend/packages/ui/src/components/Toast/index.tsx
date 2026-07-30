/**
 * Toast:全局轻提示(sonner 封装)。
 * Toaster 在应用根部挂载一次;toast 直接复用 sonner API(成功用 toast.success);
 * toastError 统一错误提示形态:用户向文案 + 可选 trace_id 报障编号(CLAUDE.md §8:
 * 技术细节只进日志,用户只看得到自然语言提示与报障编号)。
 * 无障碍:aria-live(polite)区域由 sonner 内建保证,无需额外处理。
 */
import { Toaster as SonnerToaster, toast } from "sonner";

// re-export:业务侧统一从本组件取 toast,不直接依赖 sonner
export { toast };

/**
 * Toaster:品牌化 toast 容器。
 * unstyled + token 类名接管全部视觉(零裸值);右上角堆叠、间距 10。
 * 当前仅光面(浅)语境;深色语境(沉浸态)后续按需扩展 on-dark 变体。
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

/**
 * toastError:错误提示统一入口。
 * message 必须是用户向文案(不暴露技术细节);traceId 存在时以等宽小字
 * 第二行展示报障编号,把定位能力留给运维。返回 toast id 供手动关闭。
 */
export function toastError(message: string, traceId?: string): string | number {
  return toast.error(message, {
    description: traceId ? `如需帮助,请提供编号 ${traceId}` : undefined,
    // trace_id 是标识符,等宽字体便于辨认与抄录
    classNames: { description: "mt-1 font-mono text-xs text-ink-sub" },
  });
}
