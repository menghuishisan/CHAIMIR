/**
 * cn():类名合并工具 —— clsx 组合条件类,tailwind-merge 去重冲突工具类。
 * 全组件库统一经此拼接类名。
 *
 * 本库有两类自定义工具类,处理方式不同:
 *
 * **A. 与 Tailwind 同名组的变体**(`rounded-pane`、`text-nav`、`text-stat`、`duration-fast`、`z-sticky`、
 * `animate-*`、`transition-sidebar`、`origin-radix-select`、`w-wb-*`、`min-w-popover-trigger`):它们和原生类设同一个属性,
 * 必须登记进对应 classGroup,否则消费方经 className 覆盖时 twMerge 认不出冲突,
 * 胜负会落到样式表生成顺序上(不可控)。**新增此类工具类时必须同步登记到这里。**
 *
 * **B. 独立复合工具类**(`hit-target`、`metric-band`、`pressable`、`skeleton-shimmer`、
 * `interactive-card`、`card-link`):名字在 Tailwind 里没有对应组,处理分三种。
 *
 * `metric-band` 设 `display` + `grid-template-columns`,后写的 `grid-cols-*` 或 `flex`
 * 是调用方明确要换排布,应当接管 —— 同样登记冲突。
 *
 * `hit-target` **不登记任何冲突**:它的 `::after` 用 `inset: calc((100% - 44px) / 2)` 向外撑命中区,
 * 而 `absolute` 自身也建立包含块,所以它在 `relative`/`absolute` 下都成立。
 * 若声明「后写的定位类移除 hit-target」,一个改定位的调用会静默把触达区缩回 44px 以下(违反 §3.2)。
 * 它与定位类是共存关系,不是冲突关系。
 *
 * `skeleton-shimmer`/`pressable`/`interactive-card`/`card-link` 是**多属性复合且取值由规范锁定**。
 * 这里的正确答案不是「让覆盖确定性地生效」,而是**拒绝覆盖** —— 复合类被整体移除时,
 * 只想改一个属性的调用会连带丢掉其它属性,静默出错比顺序不确定更糟。
 * 故改由静态门禁拦截:`scripts/audit/check-frontend-standards.mjs` 检查
 * `skeleton-shimmer` 是否与 `bg-*` 同处一个 `className`。
 * `pressable` 需要抑制按压时用显式 `active:transform-none!`(见 Button 注释)。
 */
import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * 新增的自定义 classGroup id 必须经泛型参数声明。
 * tailwind-merge 的 `extend.classGroups` 只接受**已存在**的组 id;
 * 自建组不走泛型时类型层直接报错(运行期虽然照样生效,但那就成了未声明的隐式约定)。
 */
type ChaimirClassGroupId = "chaimir-metric-band" | "chaimir-card-grid";

const twMerge = extendTailwindMerge<ChaimirClassGroupId>({
  extend: {
    classGroups: {
      /* A. 同名组变体 —— 与原生类设同一属性,登记进原组 */
      rounded: [{ rounded: ["pane"] }],
      // 字号令牌 nav/stat:与 text-sm 等同组,否则 text-ink 这类颜色类会被误判为字号冲突
      "font-size": [{ text: ["nav", "stat"] }],
      duration: [{ duration: ["press", "fast", "base", "slow"] }],
      transition: ["transition-sidebar", "transition-ring"],
      "transform-origin": ["origin-radix-select"],
      // 工作台栏宽(§2.4):设的是 width,必须与 w-* 同组,否则 w-full 覆盖不掉
      w: ["w-wb-aside", "w-wb-stream", "w-wb-rail", "w-topbar-popover", "w-picker-popover"],
      // 弹出层不窄于触发器:设的是 min-width,与 min-w-* 同组
      "min-w": ["min-w-popover-trigger"],
      z: [
        {
          z: [
            "below",
            "ground",
            "base",
            "sticky",
            "dropdown",
            "drawer",
            "immersive",
            "modal",
            "toast",
          ],
        },
      ],
      animate: [
        {
          animate: [
            "rise",
            "develop",
            "shimmer",
            "seal-drop",
            "seal-ring",
            "mint",
            "pop-in",
            "pop-out",
            "fade-in",
            "fade-out",
            "modal-in",
            "modal-out",
            "slide-in-right",
            "slide-out-right",
            "slide-in-bottom",
            "slide-out-bottom",
            "slide-in-left",
            "slide-out-left",
            "pane-in",
            "immersive-in",
            "immersive-out",
          ],
        },
      ],

      /* B. 只设单一属性、被覆盖后语义明确的独立工具类:成组以便声明冲突 */
      "chaimir-metric-band": ["metric-band"],
      "chaimir-card-grid": ["card-grid"],
    },
    conflictingClassGroups: {
      // 后写的 grid-cols-*/display 接管排布,metric-band 的 auto-fit 栅格应被移除
      "grid-cols": ["chaimir-metric-band", "chaimir-card-grid"],
      display: ["chaimir-metric-band", "chaimir-card-grid"],
    },
  },
});

/** cn 合并类名:clsx 组合条件类,tailwind-merge 去掉冲突的工具类 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
