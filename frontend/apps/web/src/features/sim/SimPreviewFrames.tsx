// 样例教学帧预览:上架审核与作者自查共用。
//
// 为什么必须有它:自动校验只能回答「能不能跑、是否确定性」,回答不了「这个算法实现对不对」——
// 只看两个 passed 徽章的审核等于没审(见 docs/04-仿真可视化引擎/06-业务流程与状态机.md §4)。
// 帧由隔离容器在上架前渲出、经后端按 TeachingFrame 协议校验后落进审核报告,这里只负责摊开给人看。
//
// 帧用工作台同一套封闭模式渲染器绘制,故套一层墨色面板 —— 那些组件的配色是为沉浸态设计的,
// 放在宣纸光面上会失去对比;这层面板同时让审核人看到的画面与学生看到的一致。

import { useState } from 'react'
import { ChevronLeft, ChevronRight, MonitorPlay } from 'lucide-react'
import type { SimTeachingSnapshot } from '@chaimir/api-client'
import type { TeachingFrame } from '@chaimir/sim-sdk'
import { Button, TeachingFrameStage } from '@chaimir/ui'

export interface SimPreviewFramesProps {
  frames?: SimTeachingSnapshot[]
}

/**
 * SimPreviewFrames 逐帧展示隔离预览渲出的教学画面。
 * 没有样例帧时整段不出现:预览尚未跑完或包跑不起来时,结论行已经说明了原因。
 */
export function SimPreviewFrames({ frames }: SimPreviewFramesProps) {
  const [index, setIndex] = useState(0)
  if (!frames || frames.length === 0) return null

  const current = Math.min(index, frames.length - 1)
  const frame = frames[current]
  // 帧在写入审核报告前已由后端按 TeachingFrame 协议校验(见 backend validation_frame.go),
  // 这里不重复一遍同样的规则 —— 两份实现必然漂移。
  const view = frame.view as unknown as TeachingFrame

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
      <div className="overflow-x-auto rounded-md border border-dark-line bg-dark-bg">
        <p className="border-b border-dark-line px-4 py-2 text-xs text-on-dark-sub">
          {view.summary}
        </p>
        <TeachingFrameStage frame={view} />
      </div>
    </section>
  )
}
