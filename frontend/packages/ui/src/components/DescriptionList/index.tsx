/**
 * DescriptionList:键值详情列表(§5.1)。
 * 语义 dl/dt/dd 结构;1/2/3 列栅格,窄屏自动回落单列;
 * 数字/哈希类值可标 mono(font-mono tabular-nums)。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface DescriptionItem {
  /** 键名(用户向文案) */
  term: string;
  description: ReactNode;
  /** 数字/哈希/标识符值:等宽字体 + 等宽数字 */
  mono?: boolean;
}

export interface DescriptionListProps {
  items: DescriptionItem[];
  /** 桌面列数,默认 1;窄屏(<md)一律单列 */
  columns?: 1 | 2 | 3;
  /** 紧凑模式:缩小行距,用于侧栏/弹层等密集场景 */
  dense?: boolean;
  className?: string;
}

/** 列数 → 栅格类(窄屏统一回落单列) */
const COLUMNS_CLASS = {
  1: "grid-cols-1",
  2: "grid-cols-1 md:grid-cols-2",
  3: "grid-cols-1 md:grid-cols-3",
} as const;

export function DescriptionList({ items, columns = 1, dense = false, className }: DescriptionListProps) {
  return (
    <dl className={cn("grid", COLUMNS_CLASS[columns], dense ? "gap-x-6 gap-y-3" : "gap-x-6 gap-y-5", className)}>
      {items.map((item, index) => (
        // dl 内允许 div 包裹 dt/dd 成组(HTML 规范),便于栅格布局;
        // key 拼下标防 term 重名时冲突
        <div key={`${item.term}-${index}`}>
          <dt className="text-xs text-ink-sub">{item.term}</dt>
          <dd
            className={cn(
              dense ? "mt-0.5" : "mt-1",
              "text-base text-ink",
              item.mono && "font-mono tabular-nums",
            )}
          >
            {item.description}
          </dd>
        </div>
      ))}
    </dl>
  );
}
