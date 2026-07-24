// PipelinePatternRenderer 渲染分阶段数据流。

import React from 'react';
import { clsx } from 'clsx';
import type { FrameFocus, PipelinePattern } from '../../types';
import { PatternHeader, PatternInsight } from '../PatternChrome';
import { elementVisualClasses, isActiveProcess, processSpanProgress, selectableElementProps } from '../patternUtils';
import './PipelinePatternRenderer.css';

/**
 * 渲染分阶段数据流,用于哈希、签名、交易生命周期等流水线场景。
 */
export function PipelinePatternRenderer({
  pattern,
  focus,
  selectedElementId,
  reducedMotion,
  onSelectElement,
}: {
  pattern: PipelinePattern;
  focus?: FrameFocus;
  selectedElementId?: string;
  reducedMotion: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const complete = pattern.data.steps.filter((step) => step.status === 'complete').length;
  const running = pattern.data.steps.find((step) => step.status === 'running');
  return (
    <section className="sim-pattern sim-pattern--pipeline" aria-label={pattern.title}>
      <PatternHeader mode="pipeline" title={pattern.title} meta={`${complete}/${pattern.data.steps.length} 步完成`} />
      {running && <PatternInsight items={[['当前步骤', running.label], ['状态', pipelineStatusLabel(running.status)]]} />}
      <ol className="sim-pipeline">
        {pattern.data.steps.map((step, index) => (
          <li
            className={clsx('sim-pipeline__step', `is-${step.status}`, elementVisualClasses(step, focus), selectedElementId === step.id && 'is-selected')}
            key={step.id}
            aria-current={step.status === 'running' ? 'step' : undefined}
            {...selectableElementProps(step.id, onSelectElement, 'step')}
          >
            <i>{index + 1}</i>
            <span>{step.label}</span>
            <small>{step.detail}</small>
            {step.process && isActiveProcess(step.process) && (
              <span className="sim-pipeline__progress" aria-label={`${step.process.label}${Math.round(step.process.progress * 100)}%`}>
                <span style={{ inlineSize: `${Math.round(processSpanProgress(step.process, reducedMotion) * 100)}%` }} />
                <em>{Math.round(processSpanProgress(step.process, reducedMotion) * 100)}%</em>
              </span>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}

/**
 * pipelineStatusLabel 返回流水线步骤的用户向状态名。
 */
function pipelineStatusLabel(status: PipelinePattern['data']['steps'][number]['status']): string {
  const labels = { pending: '等待', running: '运行中', complete: '完成', failed: '失败' };
  return labels[status];
}
