/**
 * BrandLockup:平台 Logo 锁定组合 A2(主标志 + Chaimir 名称)。
 * 三档只改尺寸不改关系:标志高度与名称字高相当,间距为标志宽度的 1/3。
 * 名称用 font-display(站酷庆科黄油体,已自托管)而非定制字形 —— 少一个维护对象。
 * 组合旁不加角色名、课程名或第三方 Logo;业务页不重复 tagline(规范 §1.3)。
 * 这是布局约定而非图片:文本走 DOM,故字体缩放与多语言天然生效。
 */
import { cn } from "../lib/cn";
import { BrandMark, type BrandMarkSize } from "./BrandMark";

/**
 * 三档锁定组合:标志尺寸与字号成对定义,不允许调用方拆开搭配。
 * 字号刻意压在 text-2xl 及以下 —— 认证页正文标题已经用 font-display text-4xl,
 * 品牌名若也取 4xl 就与页面标题争夺注意力,标志只承担识别、不承担宣讲(规范 §1.3)。
 */
const LOCKUP_VARIANT = {
  /** 宽版:认证页品牌区、≥1024px 顶栏 */
  wide: { mark: "md", text: "text-2xl" },
  /** 窄版:侧栏 224px、窄屏认证页 */
  narrow: { mark: "sm", text: "text-lg" },
  /** 浅底版:导出文件页眉、打印 */
  paper: { mark: "lg", text: "text-3xl" },
} as const satisfies Record<string, { mark: BrandMarkSize; text: string }>;

export type BrandLockupVariant = keyof typeof LOCKUP_VARIANT;

export interface BrandLockupProps {
  /** 尺寸档:wide 认证页 / narrow 侧栏 / paper 浅底导出 */
  variant?: BrandLockupVariant;
  /** 标志的额外类名(通常用来给色,如 text-accent);名称颜色由外层继承 */
  markClassName?: string;
  className?: string;
}

export function BrandLockup({ variant = "narrow", markClassName, className }: BrandLockupProps) {
  const spec = LOCKUP_VARIANT[variant];
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <BrandMark size={spec.mark} className={markClassName} />
      {/* 名称即读屏内容,故标志保持装饰性(不传 label),避免读屏重复念两遍 */}
      <span className={cn("font-display leading-none", spec.text)}>Chaimir</span>
    </span>
  );
}
