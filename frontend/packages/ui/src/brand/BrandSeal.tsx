/**
 * BrandSeal:独立品牌章 A3(朱砂实心方印,主标志的环与点反白挖除,缺口补上)。
 * 与 BrandMark 同源两态:开口表示「还能继续验证」,合上表示「已落印、不可逆」。
 * 只用于发布、确权、结算、签名类不可逆动作与认证页入场,不作导航图标、不替代主标志。
 * 错误状态永不使用本组件(错误走 danger 冷红,规范 §1.1)。
 */
import { useId } from "react";
import { cn } from "../lib/cn";

/** 品牌章的尺寸阶:20 为最小可读档(该档需加粗笔画,见 strokeFor) */
const SEAL_SIZE = { sm: 20, md: 30, lg: 38 } as const;

export type BrandSealSize = keyof typeof SEAL_SIZE;

/**
 * strokeFor 返回该尺寸档挖除环的笔画宽度。
 * 20px 档加粗到 3.4 并把点放大到 2.1:反白图形在实底上比描线更易被像素网格吞掉,
 * 与 favicon 同理 —— 保持视觉间隙而不是保持数值一致。
 */
function strokeFor(size: BrandSealSize): { stroke: number; dot: number } {
  return size === "sm" ? { stroke: 3.4, dot: 2.1 } : { stroke: 3.2, dot: 2 };
}

export interface BrandSealProps {
  /** 尺寸阶:sm20(最小)/ md30(落印)/ lg38(结算高光) */
  size?: BrandSealSize;
  /** 读屏标签;不传则视为纯装饰 */
  label?: string;
  /** 落印入场动效(§4.5):缩放盖下 + 一圈墨晕散开;reduced-motion 由 base.css 全局降级 */
  animated?: boolean;
  className?: string;
}

export function BrandSeal({ size = "md", label, animated = false, className }: BrandSealProps) {
  const px = SEAL_SIZE[size];
  const { stroke, dot } = strokeFor(size);
  // mask id 必须每实例唯一:同页出现多枚章时,重复 id 会让后者引用到前者的 mask
  const maskId = useId();

  const seal = (
    <svg
      viewBox="0 0 32 32"
      width={px}
      height={px}
      aria-hidden={label ? undefined : true}
      role={label ? "img" : undefined}
      aria-label={label}
      className={cn("shrink-0", animated && "animate-seal-drop", className)}
    >
      {/*
        mask 内的 white/black 不是主题颜色而是遮罩的亮度通道:白=保留、黑=挖除,
        令牌表里没有、也不该有对应项(换主题时这两个值永远不变)。故用颜色关键字而非
        令牌或裸 hex —— 章的实际颜色由下方 rect 的 currentColor 决定。
      */}
      <mask id={maskId}>
        <rect width="32" height="32" fill="white" />
        <rect
          x="6"
          y="6"
          width="20"
          height="20"
          rx="3"
          fill="none"
          stroke="black"
          strokeWidth={stroke}
        />
        <circle cx="20.5" cy="16" r={dot} fill="black" />
      </mask>
      <rect x="1" y="1" width="30" height="30" rx="4" fill="currentColor" mask={`url(#${maskId})`} />
    </svg>
  );

  if (!animated) return seal;

  // 墨晕:落印瞬间从章的轮廓散开一圈,晚于章本体起步(§4.5 签名时刻)
  return (
    <span className="relative inline-flex">
      {seal}
      <span
        aria-hidden="true"
        className="absolute -inset-0.5 rounded-md border border-seal/60 animate-seal-ring"
        style={{ animationDelay: "calc(var(--t-stagger) * 3)" }}
      />
    </span>
  );
}
