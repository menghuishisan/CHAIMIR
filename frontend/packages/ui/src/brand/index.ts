/**
 * 品牌资产层出口:主标志、锁定组合、品牌章、租户徽记、内容封面。
 * 这一层是平台默认视觉资产的唯一来源,业务组件不自绘品牌图形、不各写默认图路径。
 * 几何与令牌依据见 docs/总-前端设计规范.md §1.3。
 * 媒体/附件占位不在此层:那是「图标 + 文案 + 动作」的状态呈现,由 Empty 组件承担,
 * 与品牌资产无关,另建一个组件只会与 Empty 形成两套空态语法。
 */
export { BrandMark, type BrandMarkProps, type BrandMarkSize } from "./BrandMark";
export { BrandLockup, type BrandLockupProps, type BrandLockupVariant } from "./BrandLockup";
export { BrandSeal, type BrandSealProps, type BrandSealSize } from "./BrandSeal";
export { TenantCrest, type TenantCrestProps, type TenantCrestSize } from "./TenantCrest";
export { CoverImage, type CoverImageProps, type CoverAccent, type CoverRatio } from "./CoverImage";
