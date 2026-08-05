/**
 * CodeTracePanel:代码执行追踪面板(M4 08 文档 §2.3 的渲染落点)。
 *
 * 仿真每推进一步,reducer 会在状态里写下当前触发的代码行与变量值(`state._trace`),
 * 包本身则声明了教学代码、行注释与要监视的变量(`descriptor.codeTrace`)。本面板把两者对上:
 * 高亮正在执行的行、在行下给出该行的教学注释、把变量当前值列成表。
 * 这是「可视化现象 ↔ 代码逻辑」之间的因果锚点,也是仿真能替代 IDE 断点讲解的原因。
 *
 * 纯受控展示组件:不取数、不持有运行时,追踪数据由页面从仿真快照传入。
 */
import { cn } from "../lib/cn";
import type { CodeTraceDef, TraceInfo, VariableWatchDef } from "@chaimir/sim-sdk";

/** 高亮档 → 行样式:正常=玉色左线,达成=玉色底,异常=冷红底。灰度下靠左线宽度与文字区分。 */
const HIGHLIGHT_ROW: Record<NonNullable<CodeTraceDef["lineMapping"][number]["highlightStyle"]>, string> = {
  normal: "border-l-accent bg-dark-elevated",
  success: "border-l-accent bg-dark-elevated text-accent",
  error: "border-l-on-dark-danger bg-dark-elevated text-on-dark-danger",
};

/** 语言的用户向名称:界面不写 pseudocode 这类开发术语。 */
const LANGUAGE_LABEL: Record<CodeTraceDef["language"], string> = {
  solidity: "Solidity 合约",
  rust: "Rust 实现",
  go: "Go 实现",
  javascript: "JavaScript 实现",
  pseudocode: "教学伪代码",
};

/** 变量格式 → 展示文本:hex 补 0x 前缀,布尔换成中文,其余原样。 */
function formatWatchValue(value: unknown, format: VariableWatchDef["format"]): string {
  if (value === undefined || value === null || value === "") return "—";
  if (format === "bool" || typeof value === "boolean") return value ? "是" : "否";
  if (format === "hex") {
    const text = String(value);
    return text.startsWith("0x") ? text : `0x${text}`;
  }
  return String(value);
}

export interface CodeTracePanelProps {
  /** 包声明的追踪配置(源码、行映射、变量监视) */
  codeTrace: CodeTraceDef;
  /** 当前状态写下的追踪信息;仿真尚未产生追踪时为空,面板只展示静态源码 */
  trace?: TraceInfo;
  className?: string;
}

/**
 * CodeTracePanel 渲染源码、当前触发行的注释与变量监视表。
 */
export function CodeTracePanel({ codeTrace, trace, className }: CodeTracePanelProps) {
  const lines = codeTrace.sourceCode.split("\n");
  const triggered = new Set(trace?.triggeredLines ?? []);
  // 同一行可能有多条映射(不同触发条件),按行号取第一条已足够表达该行在讲什么
  const annotationByLine = new Map(
    codeTrace.lineMapping.map((mapping) => [mapping.line, mapping] as const).reverse(),
  );
  const watches = codeTrace.variableWatch ?? [];
  const activeLabel = triggered.size > 0 ? `正在执行第 ${[...triggered].sort((a, b) => a - b).join("、")} 行` : "尚未开始执行";

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <div className="flex items-baseline justify-between gap-2">
        <h3 className="min-w-0 truncate text-xs font-medium text-on-dark">
          {LANGUAGE_LABEL[codeTrace.language]}
        </h3>
        <span className="shrink-0 text-xs text-on-dark-sub">{activeLabel}</span>
      </div>

      <ol aria-label="教学代码与执行位置" className="flex flex-col overflow-hidden rounded-md border border-dark-line bg-terminal">
        {lines.map((line, index) => {
          const lineNumber = index + 1;
          const mapping = triggered.has(lineNumber) ? annotationByLine.get(lineNumber) : undefined;
          const style = mapping?.highlightStyle ?? "normal";
          const active = triggered.has(lineNumber);
          return (
            <li
              key={lineNumber}
              aria-current={active ? "step" : undefined}
              className={cn(
                "border-l-2 border-l-transparent px-2 py-0.5",
                active && HIGHLIGHT_ROW[style],
              )}
            >
              <span className="flex items-start gap-2">
                <span className="w-6 shrink-0 select-none text-right font-mono text-xs tabular-nums text-on-dark-faint">
                  {lineNumber}
                </span>
                <code className="min-w-0 whitespace-pre-wrap break-words font-mono text-xs text-on-dark">
                  {line || " "}
                </code>
              </span>
              {mapping?.annotation && (
                <span className="mt-0.5 block pl-8 text-xs text-on-dark-sub">{mapping.annotation}</span>
              )}
            </li>
          );
        })}
      </ol>

      {watches.length > 0 && (
        <dl aria-label="变量监视" className="flex flex-col gap-1 rounded-md border border-dark-line p-2">
          {watches.map((watch) => (
            <div key={watch.name} className="flex items-baseline justify-between gap-2">
              <dt className="min-w-0 truncate font-mono text-xs text-on-dark-sub">{watch.name}</dt>
              <dd className="shrink-0 font-mono text-xs tabular-nums text-on-dark">
                {formatWatchValue(trace?.variables?.[watch.name], watch.format)}
              </dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}
