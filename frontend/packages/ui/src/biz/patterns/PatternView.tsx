/**
 * PatternView:按 mode 把绑定分派到对应的封闭模式渲染器(七种模式的唯一入口)。
 * 分派集中在此,舞台与页面都不需要知道有哪些模式;switch 穷尽七种取值,
 * 新增模式会在此处产生类型错误而非静默不渲染。
 */
import { ChainPatternView } from "./ChainPatternView";
import { ChartPatternView } from "./ChartPatternView";
import { GraphPatternView } from "./GraphPatternView";
import { LanePatternView } from "./LanePatternView";
import { MatrixPatternView } from "./MatrixPatternView";
import { PipelinePatternView } from "./PipelinePatternView";
import { TreePatternView } from "./TreePatternView";
import type { PatternViewProps } from "./types";
import type { PatternBinding } from "@chaimir/sim-sdk";

export function PatternView({ pattern, ...rest }: PatternViewProps<PatternBinding>) {
  switch (pattern.mode) {
    case "graph":
      return <GraphPatternView pattern={pattern} {...rest} />;
    case "chain":
      return <ChainPatternView pattern={pattern} {...rest} />;
    case "tree":
      return <TreePatternView pattern={pattern} {...rest} />;
    case "matrix":
      return <MatrixPatternView pattern={pattern} {...rest} />;
    case "pipeline":
      return <PipelinePatternView pattern={pattern} {...rest} />;
    case "lane":
      return <LanePatternView pattern={pattern} {...rest} />;
    case "chart":
      return <ChartPatternView pattern={pattern} {...rest} />;
  }
}
