/**
 * Skeleton:骨架屏占位(§5.1)。
 * skeleton-shimmer 微光扫过(加载指示,允许循环);变体高度对应真实内容:
 * line=正文行 / title=标题行 / block=内容块 / stat=统计数字。
 * 占位必须预留与真实内容一致的空间,防止加载完成后布局跳动(CLS)。
 */
import { cva } from "class-variance-authority";
import { cn } from "../../lib/cn";

export type SkeletonVariant = "line" | "title" | "block" | "stat";
export type SkeletonWidth = "full" | "half" | "third";

const skeletonVariants = cva("skeleton-shimmer rounded-md", {
  variants: {
    variant: {
      line: "h-4",
      title: "h-6",
      block: "h-24",
      stat: "h-16",
    },
    width: {
      full: "w-full",
      half: "w-1/2",
      third: "w-1/3",
    },
  },
  defaultVariants: {
    width: "full",
  },
});

export interface SkeletonProps {
  /** 占位形态:line 正文行 / title 标题行 / block 内容块 / stat 统计数字 */
  variant: SkeletonVariant;
  /** 宽度阶,默认撑满 */
  width?: SkeletonWidth;
  /** 行数;大于 1 时渲染多行,末行 3/4 宽模拟自然段落 */
  lines?: number;
  className?: string;
}

export function Skeleton({ variant, width = "full", lines = 1, className }: SkeletonProps) {
  // 骨架纯装饰,对读屏隐藏;加载语义由容器(aria-busy 等)表达
  if (lines <= 1) {
    return <div aria-hidden="true" className={cn(skeletonVariants({ variant, width }), className)} />;
  }
  return (
    <div aria-hidden="true" className={cn("flex flex-col gap-2", className)}>
      {Array.from({ length: lines }, (_, i) => (
        <div
          key={i}
          className={cn(
            skeletonVariants({ variant, width }),
            // 末行收窄为 3/4 宽,呈现自然的文本收尾
            i === lines - 1 && "w-3/4",
          )}
        />
      ))}
    </div>
  );
}
