/**
 * cn():类名合并工具 —— clsx 组合条件类,tailwind-merge 去重冲突工具类。
 * 全组件库统一经此拼接类名。
 * 经 extendTailwindMerge 注册本库自定义工具类/令牌类组:否则消费方经 className
 * 覆盖 rounded-pane / duration-* / z-* 等自定义类时无法正确去重,胜负会落到
 * 样式表生成顺序上(不可控)。新增自定义工具类时必须同步登记到这里。
 */
import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      rounded: [{ rounded: ["pane"] }],
      shadow: [{ shadow: ["pane"] }],
      duration: [{ duration: ["press", "fast", "base", "slow"] }],
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
    },
  },
});

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
