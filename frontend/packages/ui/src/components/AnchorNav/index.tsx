/**
 * AnchorNav:页内分组锚点导航(规范 §6.5.3 第 ③ 族「配置表单」)。
 *
 * 长配置页的问题不是「找不到某一项」,是「不知道这页一共有几块、我在第几块」。
 * 锚点导航把结构摊开在旁边,同时充当跳转入口。
 *
 * 响应式(§6.4.1 规则 4 的同类处理):`≥lg` 为左侧竖列并随滚动吸顶;
 * `<lg` 收成顶部横向可滚 chips —— 窄屏挤两栏会让表单只剩半个屏宽。
 *
 * 高亮由调用方控制(`activeId`):滚动监听涉及页面自己的滚动容器与阈值,
 * 放进组件会让它必须知道外部布局,那是把耦合藏进设计系统。
 */
import { cn } from "../../lib/cn";

export interface AnchorNavItem {
  /** 目标分组的 DOM id(不带 #) */
  id: string;
  /** 分组名(用户向) */
  label: string;
}

export interface AnchorNavProps {
  /** 无障碍名称,如「学校配置分组」 */
  label: string;
  items: AnchorNavItem[];
  /** 当前所在分组 id */
  activeId?: string;
  /** 点击回调;不传时退回原生锚点跳转 */
  onSelect?: (id: string) => void;
  className?: string;
}

export function AnchorNav({ label, items, activeId, onSelect, className }: AnchorNavProps) {
  return (
    <nav
      aria-label={label}
      className={cn(
        // <lg:横向一条可滚 chips;≥lg:竖列吸顶(top 取顶栏高度令牌,不写裸数字)
        "flex gap-2 overflow-x-auto pb-1",
        "lg:sticky lg:flex-col lg:gap-0.5 lg:overflow-visible lg:pb-0",
        className,
      )}
      style={{ top: "var(--topnav-h)" }}
    >
      {items.map((item) => {
        const isActive = item.id === activeId;
        return (
          <a
            key={item.id}
            href={`#${item.id}`}
            aria-current={isActive ? "true" : undefined}
            onClick={
              onSelect &&
              ((event) => {
                // 交给页面处理时阻止默认跳转:页面通常要做平滑滚动并同步 activeId
                event.preventDefault();
                onSelect(item.id);
              })
            }
            className={cn(
              "hit-target shrink-0 whitespace-nowrap rounded-full px-3 py-1.5 text-sm transition-colors duration-fast ease-out",
              // ≥lg 是竖列条目:去掉胶囊圆角,改用左侧激活条,与侧栏语言一致
              "lg:rounded-md lg:px-3 lg:py-2",
              isActive
                ? "bg-primary-soft font-medium text-primary"
                : "text-ink-sub hover:bg-surface-hover hover:text-ink",
            )}
          >
            {item.label}
          </a>
        );
      })}
    </nav>
  );
}
