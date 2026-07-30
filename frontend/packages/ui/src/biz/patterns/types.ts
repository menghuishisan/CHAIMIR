/**
 * patterns/types:七种封闭模式渲染器的公共入参契约。
 * 渲染器一律纯受控:只吃 TeachingFrame 里已有的数据,不取数、不持有运行时状态。
 * 标注与视图标题由舞台统一渲染,渲染器只负责本视图的可视化与元素选择。
 */
import type { FrameFocus, PatternBinding } from "@chaimir/sim-sdk";
import type { PatternDensity } from "../frameVisual";

export interface PatternViewProps<TPattern extends PatternBinding> {
  /** 本视图的绑定数据(含 id / mode / title / data) */
  pattern: TPattern;
  /** 当前帧焦点,决定哪些元素前置、哪些淡化 */
  focus: FrameFocus;
  /** 渲染密度:stage=宽主舞台,panel=窄侧栏 */
  density: PatternDensity;
  /** 当前选中的元素编号(来自仿真状态,非组件内部状态) */
  selectedElementId?: string;
  /** 元素选择回调;不传则元素不可交互(只读回放等场景) */
  onSelectElement?: (elementId: string) => void;
}
