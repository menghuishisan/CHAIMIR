/**
 * Breadcrumb:kicker 面包屑(§5.1 / §6.5 页面头)。
 * Data 层等宽小字 + 全大写宽字距;分隔符「/」纯装饰(aria-hidden);
 * 末项为当前页(aria-current="page"),其余项可带链接。
 * href 经协议白名单校验(防存储型 XSS 的 javascript: 注入),不通过的降级为纯文本。
 */
import { cn } from "../../lib/cn";

/**
 * href 协议白名单校验:仅允许当前站点的 http(s) 地址。
 * 防存储型 XSS 与协议相对跳转:后端数据被注入 javascript:、//外站或反斜杠变体时不得渲染为链接。
 */
function isSafeHref(href: string): boolean {
  const normalized = href.trim();
  if (normalized === "") return false;
  if (normalized.includes("\\")) return false;

  try {
    const parsed = new URL(normalized, window.location.origin);
    return (
      (parsed.protocol === "http:" || parsed.protocol === "https:") &&
      parsed.username === "" &&
      parsed.password === "" &&
      parsed.origin === window.location.origin &&
      !normalized.startsWith("//")
    );
  } catch {
    return false;
  }
}

export interface BreadcrumbItem {
  /** 层级名称(用户向文案) */
  label: string;
  /** 有 href 渲染为链接;末项(当前页)通常不传 */
  href?: string;
}

export interface BreadcrumbProps {
  items: BreadcrumbItem[];
  className?: string;
}

export function Breadcrumb({ items, className }: BreadcrumbProps) {
  return (
    <nav aria-label="面包屑" className={className}>
      <ol className="flex flex-wrap items-center gap-2 font-mono text-xs uppercase">
        {items.map((item, index) => {
          const isCurrent = index === items.length - 1;
          return (
            <li key={`${item.label}-${index}`} className="flex items-center gap-2">
              {/* 分隔符仅视觉装饰,读屏跳过 */}
              {index > 0 && (
                <span aria-hidden="true" className="text-ink-faint">
                  /
                </span>
              )}
              {/* href 不过白名单时降级为纯文本,拒绝渲染可执行协议链接 */}
              {item.href && isSafeHref(item.href) ? (
                <a
                  href={item.href}
                  aria-current={isCurrent ? "page" : undefined}
                  className={cn(
                    "rounded-sm transition-colors duration-fast focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2",
                    isCurrent ? "text-ink" : "text-ink-sub hover:text-ink",
                  )}
                >
                  {item.label}
                </a>
              ) : (
                <span
                  aria-current={isCurrent ? "page" : undefined}
                  className={isCurrent ? "text-ink" : "text-ink-sub"}
                >
                  {item.label}
                </span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
