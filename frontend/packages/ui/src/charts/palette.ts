/**
 * 图表配色:useChartColors —— 运行时读取 CSS 令牌变量,转为具体色值供 Recharts 使用。
 * 为什么运行时读取:Recharts 最终把颜色落在 SVG 属性(stroke/fill)上,SVG 属性无法解析
 * var();经 getComputedStyle(document.documentElement) 读取 :root 令牌再传入,既拿到具体
 * 色值又保持 theme.css 单一真相源 —— 令牌改色,图表随之生效,不产生第二份色板。
 * 令牌是内部契约:应用入口必须已引入 "@chaimir/ui/styles"。缺失即开发期错误,
 * 直接抛错显式失败,不做静默回退(CLAUDE.md §8 错误不静默失败)。
 *
 * 两种语境(光面 paper / 墨底 dark)由同一出口按 context 返回,不存在第二份色板:
 * 光面用 jade-500/warning/info/jade-700 + ink 系文字;墨底用 accent/on-dark-amber/
 * on-dark-blue/on-dark-violet + on-dark 系文字(§8.1 玉为首、辅色从令牌派生)。
 */
import { useMemo } from "react";

/** 图表语境:paper=宣纸光面页面,dark=墨色沉浸舞台/深色面板 */
export type ChartContext = "paper" | "dark";

export interface ChartColors {
  /** 系列色序列(§8.1:玉为首,琥珀/蓝/紫为辅),按索引循环分配 */
  series: string[];
  /** 玉:首选系列色/正向指标 */
  jade: string;
  /** 琥珀:第二系列/预警 */
  warning: string;
  /** 蓝:第三系列/中性对比 */
  info: string;
  /** 冷红:仅用于异常/告警语义,不作为普通系列色 */
  danger: string;
  /** 坐标轴刻度与图例文字 */
  axis: string;
  /** 网格线/轴线 */
  grid: string;
  /** Tooltip 底色 */
  surface: string;
  /** Tooltip 正文 */
  ink: string;
}

/** 各语境的令牌名映射:改色只改 theme.css,此处只声明「用哪个令牌」 */
const TOKENS: Record<ChartContext, Record<keyof ChartColors, string | string[]>> = {
  paper: {
    series: ["--color-jade-500", "--color-warning", "--color-info", "--color-jade-700"],
    jade: "--color-jade-500",
    warning: "--color-warning",
    info: "--color-info",
    danger: "--color-danger",
    axis: "--color-ink-sub",
    grid: "--color-line",
    surface: "--color-surface",
    ink: "--color-ink",
  },
  dark: {
    series: [
      "--color-accent",
      "--color-on-dark-amber",
      "--color-on-dark-blue",
      "--color-on-dark-violet",
    ],
    jade: "--color-accent",
    warning: "--color-on-dark-amber",
    info: "--color-on-dark-blue",
    danger: "--color-on-dark-danger",
    axis: "--color-on-dark-sub",
    grid: "--color-dark-line",
    surface: "--color-dark-elevated",
    ink: "--color-on-dark",
  },
};

/** 读取单个令牌;缺失立刻抛错定位问题,绝不静默回退 */
function readToken(styles: CSSStyleDeclaration, token: string): string {
  const value = styles.getPropertyValue(token).trim();
  if (!value) {
    throw new Error(
      `设计令牌 ${token} 未找到:请确认应用入口已引入 "@chaimir/ui/styles",且令牌名与 tokens/theme.css 一致`,
    );
  }
  return value;
}

/**
 * useChartColors:按语境读取令牌变量返回图表色值对象。
 * 值在语境不变时 memo(令牌是构建期常量,运行期不切主题)。
 */
export function useChartColors(context: ChartContext = "paper"): ChartColors {
  return useMemo<ChartColors>(() => {
    const styles = getComputedStyle(document.documentElement);
    const map = TOKENS[context];
    return {
      series: (map.series as string[]).map((token) => readToken(styles, token)),
      jade: readToken(styles, map.jade as string),
      warning: readToken(styles, map.warning as string),
      info: readToken(styles, map.info as string),
      danger: readToken(styles, map.danger as string),
      axis: readToken(styles, map.axis as string),
      grid: readToken(styles, map.grid as string),
      surface: readToken(styles, map.surface as string),
      ink: readToken(styles, map.ink as string),
    };
  }, [context]);
}
