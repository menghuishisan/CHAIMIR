// ChainPatternRenderer 渲染主链、分叉和区块状态。

import React from 'react';
import { clsx } from 'clsx';
import type { ChainPattern, FrameFocus } from '../../types';
import { PatternHeader, PatternInsight } from '../PatternChrome';
import { elementVisualClasses, selectableElementProps } from '../patternUtils';
import './ChainPatternRenderer.css';

/**
 * 渲染区块主链与分叉序列,用于出块、最长链、双花和自私挖矿场景。
 */
export function ChainPatternRenderer({
  pattern,
  focus,
  selectedElementId,
  onSelectElement,
}: {
  pattern: ChainPattern;
  focus?: FrameFocus;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const canonical = pattern.data.blocks;
  const forkBlocks = pattern.data.forks.flat();
  const attackerBlocks = forkBlocks.concat(canonical).filter((block) => block.status === 'attacker').length;
  return (
    <section className="sim-pattern sim-pattern--chain" aria-label={pattern.title}>
      <PatternHeader mode="chain" title={pattern.title} meta={`${canonical.length} 个主链块 / ${forkBlocks.length} 个分叉块`} />
      <PatternInsight items={[['规范链尖', pattern.data.canonicalTip ?? '等待'], ['攻击块', attackerBlocks]]} />
      <div className="sim-chain-board" role="list" aria-label="主链与分叉">
        <div className="sim-chain-row">
          <span className="sim-chain-row__label">主链</span>
          <div className="sim-chain">
            {canonical.map((block, index) => (
              <React.Fragment key={block.id}>
                {index > 0 && <span className="sim-chain__link" aria-hidden="true" />}
                <ChainBlockCard block={block} focus={focus} selectedElementId={selectedElementId} onSelectElement={onSelectElement} canonicalTip={pattern.data.canonicalTip} />
              </React.Fragment>
            ))}
          </div>
        </div>
        {pattern.data.forks.map((fork, forkIndex) => (
          <div className="sim-chain-row sim-chain-row--fork" key={`fork-${forkIndex}`}>
            <span className="sim-chain-row__label">分叉 {forkIndex + 1}</span>
            <div className="sim-chain">
              {fork.map((block, index) => (
                <React.Fragment key={`${forkIndex}-${block.id}`}>
                  {index > 0 && <span className="sim-chain__link is-fork" aria-hidden="true" />}
                  <ChainBlockCard block={block} focus={focus} selectedElementId={selectedElementId} onSelectElement={onSelectElement} canonicalTip={pattern.data.canonicalTip} />
                </React.Fragment>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/**
 * 渲染单个区块卡片,用于主链和分叉复用。
 */
function ChainBlockCard({
  block,
  focus,
  selectedElementId,
  onSelectElement,
  canonicalTip,
}: {
  block: ChainPattern['data']['blocks'][number];
  focus?: FrameFocus;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
  canonicalTip?: string;
}): React.ReactElement {
  return (
    <article
      className={clsx('sim-chain__block', `is-${block.status}`, elementVisualClasses(block, focus), selectedElementId === block.id && 'is-selected', canonicalTip === block.id && 'is-tip')}
      role="listitem"
      {...selectableElementProps(block.id, onSelectElement, 'block')}
    >
      <span className="sim-chain__height">#{block.height}</span>
      <span className="sim-chain__label">{block.label}</span>
      <code className="sim-chain__hash">{block.hash.slice(0, 10)}</code>
      <small>父 {block.parentHash.slice(0, 8)}</small>
    </article>
  );
}
