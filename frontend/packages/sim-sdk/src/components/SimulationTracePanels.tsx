// 本文件渲染仿真代码追踪与检查点结果，不承担交互命令处理。

import React from 'react';
import { AlertTriangle, CheckCircle2 } from 'lucide-react';
import type { CodeTraceDef, RuntimeSnapshot, SimPackageDescriptor } from '../types';
import './SimulationTracePanels.css';

/** CodeTracePanel 建立仿真现象、源码行与变量之间的教学联系。 */
export function CodeTracePanel({ codeTrace, snapshot }: { codeTrace: CodeTraceDef; snapshot: RuntimeSnapshot }): React.ReactElement {
  const lines = codeTrace.sourceCode.split('\n');
  const trace = snapshot.state._trace;
  const active = new Set(trace?.triggeredLines ?? []);
  return (
    <section className="sim-side-section">
      <h2>代码追踪</h2>
      <div className="sim-code">
        {lines.map((line, index) => {
          const lineNo = index + 1;
          const mapping = codeTrace.lineMapping.find((item) => item.line === lineNo);
          return (
            <div className={`sim-code__line ${active.has(lineNo) ? `is-${mapping?.highlightStyle ?? 'normal'}` : ''}`} key={lineNo}>
              <span>{lineNo}</span>
              <code>{line || ' '}</code>
              {active.has(lineNo) && mapping?.annotation && <em>{mapping.annotation}</em>}
            </div>
          );
        })}
      </div>
      <dl className="sim-watch">
        {(codeTrace.variableWatch ?? []).map((watch) => (
          <div key={watch.name}>
            <dt>{watch.name}</dt>
            <dd>{String(trace?.variables?.[watch.name] ?? '')}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/** CheckpointPanel 只展示 Worker 已计算的检查点状态，不在主线程重复判定。 */
export function CheckpointPanel({ descriptor, snapshot }: { descriptor: SimPackageDescriptor; snapshot: RuntimeSnapshot }): React.ReactElement {
  return (
    <section className="sim-side-section">
      <h2>检查点</h2>
      <div className="sim-checkpoints">
        {descriptor.checkpoints.map((checkpoint) => {
          const result = snapshot.checkpointResults[checkpoint.id];
          return (
            <article className={result?.achieved ? 'is-achieved' : 'is-pending'} key={checkpoint.id}>
              {result?.achieved ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
              <div>
                <strong>{checkpoint.label}</strong>
                <p>{result?.explanation ?? '等待仿真达到检查条件'}</p>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
