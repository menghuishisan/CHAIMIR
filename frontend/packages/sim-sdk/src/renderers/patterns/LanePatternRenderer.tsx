// LanePatternRenderer 渲染多参与方时序消息。

import React from 'react';
import { clsx } from 'clsx';
import type { FrameFocus, LanePattern } from '../../types';
import { PatternHeader, PatternInsight } from '../PatternChrome';
import { elementVisualClasses, isActiveProcess, messageTimePosition, processSpanProgress, selectableElementProps } from '../patternUtils';
import './LanePatternRenderer.css';

/**
 * 渲染多参与方时序消息,用于共识投票、跨链消息和网络传播场景。
 */
export function LanePatternRenderer({
  pattern,
  focus,
  selectedElementId,
  reducedMotion,
  onSelectElement,
}: {
  pattern: LanePattern;
  focus?: FrameFocus;
  selectedElementId?: string;
  reducedMotion: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const maxTime = Math.max(1, pattern.data.currentTime, ...pattern.data.messages.map((message) => message.endAt ?? message.at));
  const dropped = pattern.data.messages.filter((message) => message.status === 'dropped').length;
  const timeTicks = [0, Math.round(maxTime / 2), maxTime];
  return (
    <section className="sim-pattern sim-pattern--lane" aria-label={pattern.title}>
      <PatternHeader mode="lane" title={pattern.title} meta={`当前时间 ${pattern.data.currentTime} / 消息 ${pattern.data.messages.length}`} />
      <PatternInsight items={[['参与方', pattern.data.actors.length], ['丢弃', dropped]]} />
      <div className="sim-lane">
        <div className="sim-lane__axis" aria-hidden="true">
          <span />
          <div>
            {timeTicks.map((tick, index) => (
              <small key={`${tick}-${index}`} style={{ insetInlineStart: `${index === 0 ? 0 : index === 1 ? 50 : 100}%` }}>
                t{tick}
              </small>
            ))}
          </div>
        </div>
        {pattern.data.actors.map((actor) => (
          <div className="sim-lane__row" key={actor}>
            <span className="sim-lane__actor">{actor}</span>
            <div className="sim-lane__messages">
              {pattern.data.messages
                .filter((message) => message.from === actor || message.to === actor)
                .map((message) => {
                  const position = messageTimePosition(message, maxTime, reducedMotion);
                  return (
                    <span
                      className={clsx('sim-lane__message', `is-${message.status}`, elementVisualClasses(message, focus), selectedElementId === message.id && 'is-selected')}
                      key={`${actor}-${message.id}`}
                      style={{ insetInlineStart: `${position}%` }}
                      title={message.process?.label ?? message.detail}
                      {...selectableElementProps(message.id, onSelectElement, 'message')}
                    >
                      <b>{message.from === actor ? '出' : '入'}</b>
                      {message.label}
                      {message.process && isActiveProcess(message.process) && <i style={{ inlineSize: `${Math.round(processSpanProgress(message.process, reducedMotion) * 100)}%` }} aria-hidden="true" />}
                    </span>
                  );
                })}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
