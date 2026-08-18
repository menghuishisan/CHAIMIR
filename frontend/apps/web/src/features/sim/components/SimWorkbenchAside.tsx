// 仿真工作台左辅助区:手风琴阅读区 + 常驻操作区(规范 §7.2 B)。
//
// 三处装配共用它 —— 本机推演、服务端隔离推演、公开回放。三者的差别只在「有没有操作」
// 和「元素能不能点」,阅读区完全一样,故不为它们各写一份左栏。
//
// 段的取舍按场景实际声明:没有检查点就没有「观察结论」段,没有代码追踪就没有「执行追踪」段,
// 不留空标题。段标题常驻可见(等于一份目录),展开的那段占满剩余高度并自行滚动;
// 「可用操作」钉在底部不参与折叠。

import type { ReactNode } from 'react'
import {
  TeachingFrameAside,
  TeachingFrameBrief,
  WorkbenchAccordion,
  frameHasAside,
  type WorkbenchAccordionSection,
} from '@chaimir/ui'
import type {
  CheckpointDescriptor,
  CheckpointResult,
  CodeTraceDef,
  TeachingFrame,
  TraceInfo,
} from '@chaimir/sim-sdk'
import { CheckpointPanel, CodeTraceSection, achievedCount } from './WorkbenchPanels'

export interface SimWorkbenchAsideProps {
  frame: TeachingFrame
  checkpoints: CheckpointDescriptor[]
  checkpointResults: Record<string, CheckpointResult>
  codeTrace?: CodeTraceDef
  trace?: TraceInfo
  selectedElementId?: string
  /** 元素选择回调;只读回放不传 */
  onSelectElement?: (elementId: string) => void
  /** 常驻操作区;只读回放不传 */
  actions?: ReactNode
}

/**
 * SimWorkbenchAside 组装左辅助区:阶段说明 / 观察结论 / 执行追踪 / 证据与指标 + 常驻操作。
 */
export function SimWorkbenchAside({
  frame,
  checkpoints,
  checkpointResults,
  codeTrace,
  trace,
  selectedElementId,
  onSelectElement,
  actions,
}: SimWorkbenchAsideProps) {
  const sections: WorkbenchAccordionSection[] = [
    {
      id: 'sim-phase',
      title: '阶段说明',
      content: <TeachingFrameBrief frame={frame} />,
    },
  ]

  if (checkpoints.length > 0) {
    sections.push({
      id: 'sim-checkpoints',
      title: '观察结论',
      hint: `${achievedCount(checkpoints, checkpointResults)}/${checkpoints.length} 已观察到`,
      content: <CheckpointPanel checkpoints={checkpoints} results={checkpointResults} />,
    })
  }

  if (codeTrace) {
    sections.push({
      id: 'sim-trace',
      title: '执行追踪',
      content: <CodeTraceSection codeTrace={codeTrace} trace={trace} />,
    })
  }

  if (frameHasAside(frame)) {
    sections.push({
      id: 'sim-evidence',
      title: '证据与指标',
      content: (
        <TeachingFrameAside
          frame={frame}
          selectedElementId={selectedElementId}
          onSelectElement={onSelectElement}
        />
      ),
    })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <WorkbenchAccordion sections={sections} />
      {actions}
    </div>
  )
}
