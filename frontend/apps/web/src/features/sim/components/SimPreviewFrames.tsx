// 作者自查与平台审核共用学生端渲染器，样例画面用于人工核对算法与教学内容。

import { useState } from 'react'
import { ChevronLeft, ChevronRight, MonitorPlay } from 'lucide-react'
import type { SimTeachingSnapshot } from '@chaimir/api-client'
import { isTeachingFrame } from '@chaimir/sim-sdk'
import { Button, Callout, TeachingFrameStage, TeachingFrameStream, frameHasStream } from '@chaimir/ui'

export interface SimPreviewFramesProps {
  frames?: SimTeachingSnapshot[]
}

export function SimPreviewFrames({ frames }: SimPreviewFramesProps) {
  const [index, setIndex] = useState(0)
  if (!frames || frames.length === 0) return null

  const current = Math.min(index, frames.length - 1)
  const frame = frames[current]
  const view = isTeachingFrame(frame.view) ? frame.view : undefined

  return (
    <section className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-sm text-ink-sub">
          <MonitorPlay aria-hidden="true" className="size-4" />
          学生会看到的画面
        </span>
        <span className="flex items-center gap-1">
          <span className="font-mono text-xs tabular-nums text-ink-sub">
            第 {current + 1} / {frames.length} 帧 · 推演时刻 {frame.tick}
          </span>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={ChevronLeft}
            disabled={current === 0}
            onClick={() => setIndex(current - 1)}
          >
            上一帧
          </Button>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={ChevronRight}
            disabled={current >= frames.length - 1}
            onClick={() => setIndex(current + 1)}
          >
            下一帧
          </Button>
        </span>
      </div>
      {view ? <div className="overflow-x-auto rounded-md border border-dark-line bg-dark-bg">
        <p className="border-b border-dark-line px-4 py-2 text-xs text-on-dark-sub">
          {view.summary}
        </p>
        {frameHasStream(view) ? (
          <div className="grid gap-2 md:grid-cols-3">
            <div className="h-96 md:col-span-2">
              <TeachingFrameStage frame={view} />
            </div>
            <div className="h-96 border-t border-dark-line md:border-l md:border-t-0">
              <TeachingFrameStream frame={view} />
            </div>
          </div>
        ) : (
          <div className="h-96">
            <TeachingFrameStage frame={view} />
          </div>
        )}
      </div> : <Callout tone="danger">这帧预览画面不完整，请查看其他帧或联系管理员。</Callout>}
    </section>
  )
}
