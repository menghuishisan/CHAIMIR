// SimRuntimeWorkbench 统一渲染浏览器 Worker 与服务端隔离容器的仿真工作台。
// 两种执行位置只负责状态推进和底栏控制,舞台、说明、交互区、消息流与分享入口只有一套。

import type { ReactNode } from 'react'
import {
  ChainProgress,
  TeachingFrameStage,
  TeachingFrameStream,
  WorkbenchShell,
  WorkbenchTopbar,
  frameHasStream,
  frameStreamEntries,
} from '@chaimir/ui'
import type {
  InteractionDescriptor,
  JsonObject,
  RuntimeFrame,
  SimPackageDescriptor,
} from '@chaimir/sim-sdk'
import { narrativeProgress } from '../playback'
import { SimWorkbenchAside } from './SimWorkbenchAside'
import { InteractionPanel } from './WorkbenchPanels'

interface SimRuntimeWorkbenchProps {
  title: string
  version: string
  descriptor: SimPackageDescriptor
  snapshot: RuntimeFrame
  onExit: () => void
  action?: ReactNode
  /** 场景名旁的小控件:本机推演的「换一组条件」与版本切换 */
  titleExtra?: ReactNode
  onSelectElement?: (elementId: string) => void
  onInteract?: (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => void
  footer: ReactNode
}

/** SimRuntimeWorkbench 装配仿真工作台的共享可视区域。 */
export function SimRuntimeWorkbench({
  title,
  version,
  descriptor,
  snapshot,
  onExit,
  action,
  titleExtra,
  onSelectElement,
  onInteract,
  footer,
}: SimRuntimeWorkbenchProps) {
  const frame = snapshot.view
  const hasStream = frameHasStream(frame)

  return (
    <WorkbenchShell
      workbench="sim"
      topbar={
        <WorkbenchTopbar
          onExit={onExit}
          exitLabel="退出推演"
          title={title}
          subtitle={version ? `${descriptor.meta.name} · ${version}` : descriptor.meta.name}
          titleExtra={titleExtra}
          progress={
            <ChainProgress
              onDark
              size="sm"
              label="教学步骤"
              total={descriptor.narrative.length}
              done={narrativeProgress(descriptor, snapshot)}
            />
          }
          cta={action}
        />
      }
      left={
        <SimWorkbenchAside
          frame={frame}
          checkpoints={descriptor.checkpoints}
          checkpointResults={snapshot.checkpointResults}
          codeTrace={descriptor.codeTrace}
          trace={snapshot.state._trace}
          selectedElementId={snapshot.state.selectedElementId}
          onSelectElement={onSelectElement}
          actions={onInteract ? (
            <InteractionPanel
              interactions={descriptor.interactions}
              availability={snapshot.interactionAvailability}
              selectedElementId={snapshot.state.selectedElementId}
              onInteract={onInteract}
            />
          ) : undefined}
        />
      }
      leftLabel={onInteract ? '说明与操作' : '说明'}
      stage={
        <TeachingFrameStage
          frame={frame}
          selectedElementId={snapshot.state.selectedElementId}
          onSelectElement={onSelectElement}
        />
      }
      right={
        hasStream ? (
          <TeachingFrameStream
            frame={frame}
            selectedElementId={snapshot.state.selectedElementId}
            onSelectElement={onSelectElement}
          />
        ) : undefined
      }
      rightLabel="消息流"
      rightCount={frameStreamEntries(frame).length}
      footer={footer}
    />
  )
}
