/**
 * DensityMatrix:双维度密度矩阵(热力图,规范 §8.1「双维度交叉密度」)。
 *
 * 用途是找「哪一块塌了」:两个离散维度交叉,读者要在一屏里定位低谷或热点。
 * 换成长表格就退化成几十行数字逐行比对,那正是这个图形存在的理由。
 *
 * **实现用语义 `<table>` 而非 SVG。** 理由是矩阵本身就是表格数据:
 * 用 table 天然拿到行列表头关联、读屏可按行列朗读、键盘可达(§8.2 三项要求),
 * 而 SVG 版这三样都要手工补。热度只是单元格底色,不是图形。
 * 这也是 §8 里「图表库统一 Recharts」的例外之一 —— Recharts 无热力图元,
 * 且此处不需要坐标系与比例尺,引第二个图表库不合算(CLAUDE.md §4 自研需说明取舍)。
 *
 * 底色由 useChartColors 的玉色派生透明度,不引入新裸色(FE-1);
 * 图例给出刻度条与最小/最大值,单元格 title 给精确值(§8.2 hover 出精确值)。
 * 应包在 ChartContainer 内使用。
 */
import { cn } from "../lib/cn";
import { useChartColors, type ChartContext } from "./palette";

export interface DensityMatrixProps {
  /** 行标题(纵向维度),如班级 */
  rows: string[];
  /** 列标题(横向维度),如章节 */
  columns: string[];
  /** 数值矩阵,values[行][列];缺测点传 null */
  values: Array<Array<number | null>>;
  /** 数值的用户向名称,如「完成率」——进单元格 title 与图例说明 */
  valueLabel: string;
  /** 数值单位后缀,如「%」 */
  unit?: string;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
  className?: string;
}

/** 矩阵的维度下界(规范 §8.1:小于 3×3 退回分组柱状) */
const MIN_SIDE = 3;
/** 热度透明度区间:下界留一点底色以区分「有数据但很低」与「没有数据」 */
const ALPHA_MIN = 0.08;
const ALPHA_MAX = 0.95;

/** 两种语境的语义类 */
const STYLES: Record<
  ChartContext,
  { head: string; cellText: string; empty: string; legend: string; line: string }
> = {
  paper: {
    head: "text-ink-sub",
    cellText: "text-ink",
    empty: "text-ink-faint",
    legend: "text-ink-sub",
    line: "border-line",
  },
  dark: {
    head: "text-on-dark-sub",
    cellText: "text-on-dark",
    empty: "text-on-dark-faint",
    legend: "text-on-dark-sub",
    line: "border-dark-line",
  },
};

export function DensityMatrix({
  rows,
  columns,
  values,
  valueLabel,
  unit = "",
  context = "paper",
  className,
}: DensityMatrixProps) {
  const colors = useChartColors(context);
  const styles = STYLES[context];

  // 维度与形状校验前置:形状不对时静默补空会让读者以为那些格子「值为 0」
  if (rows.length < MIN_SIDE || columns.length < MIN_SIDE) {
    throw new Error(
      `DensityMatrix 需要至少 ${MIN_SIDE}×${MIN_SIDE},收到 ${rows.length}×${columns.length}:` +
        `格子太少时密度读不出来(规范 §8.1),请改用分组 CompareBarChart。`,
    );
  }
  if (values.length !== rows.length || values.some((row) => row.length !== columns.length)) {
    throw new Error(
      `DensityMatrix 的 values 必须是 ${rows.length}×${columns.length} 的矩阵:` +
        `行列数与表头不一致时无法确定缺的是哪一格。`,
    );
  }

  // 归一化区间取实测最小/最大值:固定 0–100 会让「都在 80–90」的矩阵看起来一片深色,看不出差异
  const present = values.flat().filter((value): value is number => value !== null);
  const min = present.length > 0 ? Math.min(...present) : 0;
  const max = present.length > 0 ? Math.max(...present) : 0;
  const span = max - min;

  /** 数值 → 底色透明度;全部相等时统一取中位浓度,避免除零 */
  const alphaOf = (value: number) =>
    span === 0 ? (ALPHA_MIN + ALPHA_MAX) / 2 : ALPHA_MIN + ((value - min) / span) * (ALPHA_MAX - ALPHA_MIN);

  /**
   * 底色:用 color-mix 把玉色按浓度混向透明。
   * 不拼 `#rrggbbaa` —— 令牌值的书写格式(hex / rgb() / oklch())不由本组件决定,
   * 字符串拼接会在换格式那天静默出错;color-mix 对任何合法颜色都成立。
   */
  const heatOf = (value: number) =>
    `color-mix(in srgb, ${colors.jade} ${Math.round(alphaOf(value) * 100)}%, transparent)`;

  /** 深底上的文字要反白:浓度过半时白字才读得清 */
  const isDeep = (value: number) => alphaOf(value) > 0.55;

  return (
    <div className={cn("flex min-w-0 flex-col gap-3", className)}>
      <div className="min-w-0 overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <caption className="sr-only">
            {valueLabel}矩阵,纵向为{rows.length}行,横向为{columns.length}列
          </caption>
          <thead>
            <tr>
              <th scope="col" className="sr-only">
                分组
              </th>
              {columns.map((column) => (
                <th
                  key={column}
                  scope="col"
                  className={cn("px-2 pb-2 text-center text-xs font-medium", styles.head)}
                >
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, rowIndex) => (
              <tr key={row}>
                <th
                  scope="row"
                  className={cn("whitespace-nowrap pr-3 text-right text-xs font-medium", styles.head)}
                >
                  {row}
                </th>
                {columns.map((column, columnIndex) => {
                  const value = values[rowIndex][columnIndex];
                  return (
                    <td key={column} className="p-0.5">
                      {value === null ? (
                        <div
                          className={cn(
                            "flex h-8 items-center justify-center rounded-sm border border-dashed text-xs",
                            styles.line,
                            styles.empty,
                          )}
                          title={`${row} · ${column}:暂无数据`}
                        >
                          —
                        </div>
                      ) : (
                        <div
                          className={cn(
                            "flex h-8 items-center justify-center rounded-sm text-xs tabular-nums",
                            isDeep(value) ? "text-on-solid" : styles.cellText,
                          )}
                          // 底色是连续量,只能算出来;色相来自玉色令牌,不是新增裸色(FE-1)
                          style={{ backgroundColor: heatOf(value) }}
                          title={`${row} · ${column}:${valueLabel} ${value}${unit}`}
                        >
                          {value}
                          {unit}
                        </div>
                      )}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* 图例刻度:密度图没有刻度条就只能看出「有深有浅」,读不出方向与量级(§8.2) */}
      <div className={cn("flex flex-wrap items-center gap-2 text-xs", styles.legend)}>
        <span className="tabular-nums">
          {min}
          {unit}
        </span>
        <span
          aria-hidden="true"
          className="h-2 w-28 rounded-full"
          style={{
            backgroundImage: `linear-gradient(to right, ${heatOf(min)}, ${heatOf(max)})`,
          }}
        />
        <span className="tabular-nums">
          {max}
          {unit}
        </span>
        <span>· {valueLabel}越高颜色越深,悬停单元格看精确值</span>
      </div>
    </div>
  );
}
