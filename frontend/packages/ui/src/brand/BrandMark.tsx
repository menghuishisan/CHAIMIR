/**
 * BrandMark:平台主标志 A1(缺口环 + 环内注册点)。
 * 32x32 网格,笔画 3.2,圆角 3,缺口 10.2(y 10.9→21.1),注册点 r=2 @ (20.5,16)。
 * 读作「输入进入 → 经过规则 → 留下可复核的点」:缺口表示过程还能继续验证,
 * 点表示已经留下的痕迹。缺口净宽减笔画得到的 7.0 光学间隙是全族常量 ——
 * 改笔画必须同步改缺口,否则小尺寸下缺口会被笔画吃掉(favicon.svg 同理)。
 * 颜色走 currentColor,由消费方用令牌类给色,不产出多份文件。
 */
import { cn } from "../lib/cn";

/** 主标志的尺寸阶:favicon 档另有独立文件(public/favicon.svg),此处最小 20 */
const MARK_SIZE = { sm: 20, md: 26, lg: 34, xl: 40 } as const;

export type BrandMarkSize = keyof typeof MARK_SIZE;

export interface BrandMarkProps {
  /** 尺寸阶:sm20(最小可用)/ md26(侧栏)/ lg34(浅底)/ xl40(认证页) */
  size?: BrandMarkSize;
  /** 读屏标签;不传则视为纯装饰(aria-hidden),用于旁边已有「Chaimir」文本的场合 */
  label?: string;
  className?: string;
}

export function BrandMark({ size = "md", label, className }: BrandMarkProps) {
  const px = MARK_SIZE[size];
  return (
    <svg
      viewBox="0 0 32 32"
      width={px}
      height={px}
      fill="none"
      aria-hidden={label ? undefined : true}
      role={label ? "img" : undefined}
      aria-label={label}
      className={cn("shrink-0", className)}
    >
      <path
        d="M26 21.1V23a3 3 0 0 1-3 3H9a3 3 0 0 1-3-3V9a3 3 0 0 1 3-3h14a3 3 0 0 1 3 3v1.9"
        stroke="currentColor"
        strokeWidth="3.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="20.5" cy="16" r="2" fill="currentColor" />
    </svg>
  );
}
