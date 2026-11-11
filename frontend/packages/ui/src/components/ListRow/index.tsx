/**
 * ListRow:不需要按列对齐时代替表格的列表行(规范 §5.1 / §6.5.0)。
 * 左侧状态图标或徽记(leading)、中间主文字 + 一行次要信息(meta)、右侧状态与操作(trailing)。
 * 课时、通知、公告、待办、登录会话、队列项用它。
 *
 * 整行可点的做法:主文字是一个真实的链接(href)或按钮(onSelect),它的 ::before 铺满整行,
 * 点行内任何位置都是点它 —— 链接支持中键与 Ctrl/⌘ 点击在新标签页打开、右键复制链接。
 * trailing 里的按钮浮在覆盖层之上(relative z-base),单独可点、不触发整行;
 * 整行的键盘焦点环画在覆盖层上,只在主链接/按钮获得焦点时出现。
 */
import type { MouseEvent, ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { linkActivation } from "../../lib/linkActivation";

/** 行的去向:链接(须同时给 onNavigate,交给路由做本页跳转)/ 选择(队列切换、就地展开)/ 静态 */
type ListRowAction =
  | { href: string; onNavigate: (href: string) => void; onSelect?: never }
  | { onSelect: () => void; href?: never; onNavigate?: never }
  | { href?: never; onNavigate?: never; onSelect?: never };

export type ListRowProps = {
  /** 主文字(一行,超长截断);链接或按钮的无障碍名称就是它 */
  title: ReactNode;
  /** 次要信息一行,如「第 2 章 · 视频 · 12 分钟」 */
  meta?: ReactNode;
  /** 左侧状态图标或徽记 */
  leading?: ReactNode;
  /** 右侧状态徽记、时间或操作按钮(按钮可单独点击,不触发整行) */
  trailing?: ReactNode;
  /** 当前选中(队列当前项):玉浅底 + aria-current */
  selected?: boolean;
  className?: string;
} & ListRowAction;

/** 主链接/按钮的共同样式:::before 铺满整行作点击区,焦点环画在 ::before 上 */
const PRIMARY_HIT_CLASS =
  "block w-full min-w-0 truncate text-left text-base font-medium text-ink outline-hidden before:absolute before:inset-0 before:rounded-md focus-visible:before:outline-2 focus-visible:before:-outline-offset-2 focus-visible:before:outline-accent";

/** ListRow 渲染一行;有 href 或 onSelect 时整行可点,悬停有底色,链接行右侧有箭头。 */
export function ListRow({
  title,
  meta,
  leading,
  trailing,
  selected = false,
  className,
  href,
  onNavigate,
  onSelect,
}: ListRowProps) {
  const interactive = href !== undefined || onSelect !== undefined;

  /** 普通左键交给路由做本页跳转;修饰键与中键保留浏览器默认行为(新标签页打开) */
  const handleLinkClick = (event: MouseEvent<HTMLAnchorElement>) => {
    if (!href || !onNavigate || event.defaultPrevented) return;
    if (linkActivation(event) !== "same-tab") return;
    event.preventDefault();
    onNavigate(href);
  };

  let primary: ReactNode;
  if (href !== undefined) {
    primary = (
      <a
        href={href}
        onClick={handleLinkClick}
        aria-current={selected ? "true" : undefined}
        className={PRIMARY_HIT_CLASS}
      >
        {title}
      </a>
    );
  } else if (onSelect !== undefined) {
    primary = (
      <button type="button" onClick={onSelect} aria-current={selected ? "true" : undefined} className={PRIMARY_HIT_CLASS}>
        {title}
      </button>
    );
  } else {
    primary = <div className="truncate text-base font-medium text-ink">{title}</div>;
  }

  return (
    <div
      className={cn(
        "relative flex min-h-14 items-center gap-3 px-4 py-3 transition-colors duration-fast",
        interactive && "hover:bg-surface-hover",
        selected && "bg-primary-soft hover:bg-primary-soft",
        className,
      )}
    >
      {leading && <div className="flex shrink-0 items-center">{leading}</div>}
      <div className="min-w-0 flex-1">
        {primary}
        {meta !== undefined && meta !== null && <div className="mt-0.5 truncate text-sm text-ink-sub">{meta}</div>}
      </div>
      {trailing !== undefined && trailing !== null && (
        <div className="relative z-base flex shrink-0 items-center gap-2">{trailing}</div>
      )}
      {href !== undefined && <Icon icon={ChevronRight} size="sm" className="shrink-0 text-ink-faint" />}
    </div>
  );
}
