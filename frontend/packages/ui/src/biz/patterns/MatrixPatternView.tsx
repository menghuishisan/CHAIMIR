/**
 * MatrixPatternView:矩阵视图(matrix 模式,如投票矩阵/权限矩阵)。
 * 宽舞台渲染真表格(行列表头 + 单元格图标 + 文字);窄栏改为按行分组的清单,
 * 因为 288px 放不下多列表格,横向滚动在侧栏不可用 —— 两种形态数据完全等价。
 * 单元格编号按「行-列」合成,供焦点与标注引用。
 */
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { matrixCellId, resolveEmphasis, type FrameEmphasis } from "../frameVisual";
import { TONE_ICON, TONE_TEXT, type DarkTone } from "./darkTone";
import type { PatternViewProps } from "./types";
import type { MatrixCell, MatrixPattern } from "@chaimir/sim-sdk";

/** 单元状态 → 墨底色阶 */
const CELL_TONE: Record<MatrixCell["status"], DarkTone> = {
  empty: "neutral",
  pending: "active",
  yes: "success",
  no: "warning",
  fault: "danger",
};

/** 单元状态词:表格里图标 + 文字并存,颜色不是唯一表达 */
const CELL_STATUS_TEXT: Record<MatrixCell["status"], string> = {
  empty: "无",
  pending: "等待中",
  yes: "已满足",
  no: "未满足",
  fault: "异常",
};

/** 强调档 → 单元底色:焦点单元加深底并配玉色描边 */
const CELL_EMPHASIS: Record<FrameEmphasis, string> = {
  focus: "bg-dark-elevated ring-1 ring-accent",
  context: "",
  history: "opacity-70",
  ghost: "opacity-40",
};

export function MatrixPatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<MatrixPattern>) {
  const { rows, columns, cells } = pattern.data;

  /** cellNode:单元内容 —— 图标 + 标签 + 状态词,三重表达 */
  const cellNode = (cell: MatrixCell) => (
    <span className="flex items-center gap-1.5">
      <Icon icon={TONE_ICON[CELL_TONE[cell.status]]} size="xs" className={TONE_TEXT[CELL_TONE[cell.status]]} />
      <span className="truncate text-xs text-on-dark">{cell.label}</span>
      <span className={cn("shrink-0 text-xs", TONE_TEXT[CELL_TONE[cell.status]])}>
        {CELL_STATUS_TEXT[cell.status]}
      </span>
    </span>
  );

  if (density === "panel") {
    // 窄栏:按行分组的清单,每行下列出各列结果
    return (
      <div className="flex flex-col gap-2" aria-label={`${pattern.title} 矩阵`}>
        {rows.map((row, rowIndex) => (
          <div key={row} className="rounded-md border border-dark-line bg-dark-elevated p-2">
            <div className="mb-1 text-xs font-medium text-on-dark">{row}</div>
            <ul className="flex flex-col gap-1">
              {columns.map((column, columnIndex) => {
                const cell = cells[rowIndex]?.[columnIndex];
                if (!cell) return null;
                const id = matrixCellId(row, column);
                const emphasis = resolveEmphasis(id, focus, cell.meta);
                return (
                  <li key={column} className={cn("flex flex-col", CELL_EMPHASIS[emphasis])}>
                    <span className="text-xs text-on-dark-sub">{column}</span>
                    {cellNode(cell)}
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-md border border-dark-line">
      <table className="w-full border-collapse text-sm">
        <caption className="sr-only">{pattern.title}</caption>
        <thead>
          <tr>
            <th scope="col" className="border-b border-dark-line bg-dark-elevated px-3 py-2 text-left text-xs font-medium text-on-dark-sub">
              对象
            </th>
            {columns.map((column) => (
              <th
                key={column}
                scope="col"
                className="border-b border-dark-line bg-dark-elevated px-3 py-2 text-left text-xs font-medium text-on-dark-sub"
              >
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={row} className="border-b border-dark-line last:border-b-0">
              <th scope="row" className="px-3 py-2 text-left text-xs font-medium text-on-dark">
                {row}
              </th>
              {columns.map((column, columnIndex) => {
                const cell = cells[rowIndex]?.[columnIndex];
                if (!cell) return null;
                const id = matrixCellId(row, column);
                const emphasis = resolveEmphasis(id, focus, cell.meta);
                const selected = id === selectedElementId;
                return (
                  <td key={column} className={cn("px-3 py-2", CELL_EMPHASIS[emphasis], selected && "ring-1 ring-accent")}>
                    {onSelectElement ? (
                      <button
                        type="button"
                        aria-pressed={selected}
                        onClick={() => onSelectElement(id)}
                        // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
                        className="pressable block w-full rounded-sm text-left hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
                      >
                        {cellNode(cell)}
                      </button>
                    ) : (
                      cellNode(cell)
                    )}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
