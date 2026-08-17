/**
 * CoverImage:内容封面 A6。有用户封面显示用户封面,没有则按资源 id 稳定落到四张纸材质之一,
 * 并在右侧压一枚大字题识(如「实验」「竞赛」)作为该类内容的视觉标记。
 *
 * 题识用 DOM/SVG 文字而非烘焙进图片:图像模型渲染不出可靠的汉字(缺笔、变形),
 * 且烘焙进去的字无法翻译、无法随业务改名、读屏取不到、裁切时会被切断。
 * 走 SVG 后题识随容器等比缩放,颜色由令牌给,四张材质仍是纯纹理、可通吃各类内容。
 *
 * 标题与元信息一律用 ink 而非 ink-sub:后者在干净宣纸上只有 5.10:1,任何墨晕都会压穿。
 * 三级回退:用户封面 → 纸材质 → 纯 CSS 同构面(布局尺寸不变,不出现破图)。
 */
import { useState } from 'react'
import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

/** 纸材质数量;资源 id 按此取模稳定分配,同一资源在列表/详情/分享预览永远同一张 */
const PAPER_COUNT = 4

/** 材质资源目录:平台级唯一来源,业务组件不各写一份路径 */
const PAPER_BASE = '/covers/paper'

/** 注册线与题识的语义色:调用方按业务类型选,材质本身不承担类型区分 */
const ACCENT_CLASS = {
  jade: { line: 'bg-primary', glyph: 'text-primary' },
  blue: { line: 'bg-info', glyph: 'text-info' },
  cinnabar: { line: 'bg-seal', glyph: 'text-seal' },
  graphite: { line: 'bg-ink-sub', glyph: 'text-ink-sub' },
} as const

export type CoverAccent = keyof typeof ACCENT_CLASS

/** 两档比例:列表缩略 16:9,详情页可用 3:2 */
const RATIO_CLASS = { '16/9': 'aspect-video', '3/2': 'aspect-3/2' } as const

export type CoverRatio = keyof typeof RATIO_CLASS

/* 题识版面(SVG 用户单位,与 16:9 视框对齐):字号、行距与右侧留白 */
const GLYPH_FONT_SIZE = 26
const GLYPH_STEP = 27
const GLYPH_X = 144
const GLYPH_CENTER_Y = 45

/**
 * paperIndexOf 由资源 id 稳定取一张材质。
 * id 是十进制字符串(雪花号超出 Number 安全整数,不能 Number() 后取模)。
 * 十进制下 100 能被 4 整除,故末两位即可决定 id % 4 —— 结果与大整数取模完全一致。
 */
function paperIndexOf(id: string): number {
  const tail = id.replace(/\D/g, '').slice(-2)
  if (tail === '') return 0
  return Number(tail) % PAPER_COUNT
}

export interface CoverImageProps {
  /** 资源 id:决定回退时用哪张材质,保证同一资源各处一致 */
  id: string
  /**
   * 已解析好的封面地址:课程封面投放授权换来的 storage 流式地址。
   * 本组件不发请求 —— UI 包不认识后端接口,换授权交给页面层,组件只负责显示与回落。
   */
  coverSrc?: string
  /** 读屏用的内容名称(如课程名);图上叠加的标题由 title 插槽渲染 */
  name: string
  /** 题识:两字为宜(如「实验」「竞赛」),竖排压在右侧;仅在使用默认材质时出现 */
  glyph?: string
  /** 注册线与题识的语义色:表达业务类型,由调用方映射 */
  accent?: CoverAccent
  /** 左下角标题(通常是内容名) */
  title?: ReactNode
  /** 标题下一行元信息 */
  meta?: ReactNode
  /** 比例:16/9 列表 / 3/2 详情 */
  ratio?: CoverRatio
  className?: string
}

export function CoverImage({
  id,
  coverSrc,
  name,
  glyph,
  accent = 'jade',
  title,
  meta,
  ratio = '16/9',
  className,
}: CoverImageProps) {
  // 两级失败分别记:用户封面失败要退到材质,材质再失败才退到纯 CSS 面
  const [failedCoverSrc, setFailedCoverSrc] = useState<string>()
  const [failedPaperSrc, setFailedPaperSrc] = useState<string>()
  const paper = `${PAPER_BASE}-0${paperIndexOf(id) + 1}`
  const hasCover = Boolean(coverSrc && coverSrc.trim() !== '') && failedCoverSrc !== coverSrc
  const paperFailed = failedPaperSrc === paper
  const tone = ACCENT_CLASS[accent]
  const glyphChars = glyph ? Array.from(glyph) : []

  return (
    <div
      className={cn(
        'relative overflow-hidden rounded-lg bg-surface',
        RATIO_CLASS[ratio],
        className
      )}
    >
      {hasCover ? (
        <img
          src={coverSrc}
          alt={`${name}封面`}
          loading="lazy"
          decoding="async"
          onError={() => setFailedCoverSrc(coverSrc)}
          className="absolute inset-0 size-full object-cover"
        />
      ) : paperFailed ? (
        // 第三级:材质图也取不到时留一块宣纸面,尺寸与层次不变,不出现破图图标
        <div className="absolute inset-0 bg-surface-sunken" />
      ) : (
        <picture>
          <source srcSet={`${paper}.avif`} type="image/avif" />
          <source srcSet={`${paper}.webp`} type="image/webp" />
          {/* 材质是装饰:内容名由 title 插槽与外层列表承载,alt 留空则读屏不会念「纸纹理」 */}
          <img
            src={`${paper}.webp`}
            alt=""
            loading="lazy"
            decoding="async"
            onError={() => setFailedPaperSrc(paper)}
            className="absolute inset-0 size-full object-cover"
          />
        </picture>
      )}

      {/* 注册线与题识只在使用默认材质时出现;用户封面已自带视觉,再叠一层是双重强调 */}
      {!hasCover && (
        <>
          <span aria-hidden="true" className={cn('absolute inset-y-0 left-0 w-1', tone.line)} />
          {glyphChars.length > 0 && (
            <svg
              aria-hidden="true"
              viewBox="0 0 160 90"
              preserveAspectRatio="xMaxYMid meet"
              className={cn('absolute inset-0 size-full opacity-20', tone.glyph)}
            >
              {glyphChars.map((char, index) => (
                <text
                  key={`${char}-${index}`}
                  x={GLYPH_X}
                  y={GLYPH_CENTER_Y + (index - (glyphChars.length - 1) / 2) * GLYPH_STEP}
                  textAnchor="middle"
                  dominantBaseline="central"
                  fontSize={GLYPH_FONT_SIZE}
                  fill="currentColor"
                  className="font-display"
                >
                  {char}
                </text>
              ))}
            </svg>
          )}
        </>
      )}

      {(title !== undefined && title !== null) || (meta !== undefined && meta !== null) ? (
        // 右侧留白避开题识:题识虽淡到不影响对比度,但压在字上会互相干扰阅读
        <div className="absolute inset-x-0 bottom-0 pb-2.5 pl-4 pr-14">
          {title !== undefined && title !== null && (
            <div className="line-clamp-2 text-sm font-semibold text-ink">{title}</div>
          )}
          {meta !== undefined && meta !== null && (
            <div className="mt-0.5 text-xs text-ink opacity-70">{meta}</div>
          )}
        </div>
      ) : null}
    </div>
  )
}
