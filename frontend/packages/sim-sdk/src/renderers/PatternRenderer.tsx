// PatternRenderer 仅按封闭模式分派平台维护的可视化渲染器。

import React from 'react';
import type { FrameFocus, PatternBinding } from '../types';
import { ChainPatternRenderer } from './patterns/ChainPatternRenderer';
import { ChartPatternRenderer } from './patterns/ChartPatternRenderer';
import { GraphPatternRenderer } from './patterns/GraphPatternRenderer';
import { LanePatternRenderer } from './patterns/LanePatternRenderer';
import { MatrixPatternRenderer } from './patterns/MatrixPatternRenderer';
import { PipelinePatternRenderer } from './patterns/PipelinePatternRenderer';
import { TreePatternRenderer } from './patterns/TreePatternRenderer';
import './PatternRenderer.css';

export interface PatternRendererProps {
  pattern: PatternBinding;
  focus?: FrameFocus;
  selectedElementId?: string;
  reducedMotion?: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}

/** PatternRenderer 把封闭模式交给对应的唯一渲染器。 */
export function PatternRenderer({ pattern, focus, selectedElementId, reducedMotion = false, onSelectElement }: PatternRendererProps): React.ReactElement {
  const props = { focus, selectedElementId, onSelectElement };
  switch (pattern.mode) {
    case 'graph': return <GraphPatternRenderer pattern={pattern} reducedMotion={reducedMotion} {...props} />;
    case 'chain': return <ChainPatternRenderer pattern={pattern} {...props} />;
    case 'tree': return <TreePatternRenderer pattern={pattern} {...props} />;
    case 'matrix': return <MatrixPatternRenderer pattern={pattern} {...props} />;
    case 'pipeline': return <PipelinePatternRenderer pattern={pattern} reducedMotion={reducedMotion} {...props} />;
    case 'lane': return <LanePatternRenderer pattern={pattern} reducedMotion={reducedMotion} {...props} />;
    case 'chart': return <ChartPatternRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
  }
}
