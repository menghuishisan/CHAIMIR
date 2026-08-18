// 仿真播放控制条:速度档 + 播放/暂停/单步/回退/重播。
//
// 仿真工作台与公开回放页共用它 —— 两页的播放语义一致(M4 需求 C2/C3:单步推进/回退、变速/暂停),
// 差别只在"一步是什么"与是否提供换随机条件,故按 props 门控而不是各写一套控制区。

import { Pause, Play, RotateCcw, StepBack, StepForward } from 'lucide-react'
import { Button, SegmentedControl } from '@chaimir/ui'
import { SIM_SPEED_OPTIONS } from '../playback'

export interface SimPlaybackControlsProps {
  playing: boolean
  /** 播放/暂停切换 */
  onToggle: () => void
  /** 走一步(手动) */
  onStep: () => void
  /** 回退一步 */
  onStepBack: () => void
  /** 回到初始状态 */
  onRestart: () => void
  multiplier: number
  onMultiplierChange: (multiplier: number) => void
  /** 出错后禁用全部控制:此时画面已停在最后一个可解释的状态 */
  disabled: boolean
  /** 已到末尾(公开回放复现完成):不能再往前,但仍可回退与重播 */
  atEnd?: boolean
  /** 尚在初始状态:回退与重播无意义 */
  atStart: boolean
  /** 播放按钮文案(工作台是「自动推进」,回放页是「播放」) */
  playLabel: string
}

/**
 * SimPlaybackControls 渲染播放控制区。
 * 速度档用分段控件而不是滑块:挡位少且离散,选起来比拖动准。
 */
export function SimPlaybackControls({
  playing,
  onToggle,
  onStep,
  onStepBack,
  onRestart,
  multiplier,
  onMultiplierChange,
  disabled,
  atEnd = false,
  atStart,
  playLabel,
}: SimPlaybackControlsProps) {
  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2">
      <SegmentedControl
        onDark
        size="sm"
        aria-label="推演速度"
        value={String(multiplier)}
        onValueChange={(value) => onMultiplierChange(Number(value))}
        options={SIM_SPEED_OPTIONS.map((option) => ({
          value: String(option.multiplier),
          label: option.label,
        }))}
      />
      <Button
        variant="on-dark"
        size="sm"
        leftIcon={RotateCcw}
        disabled={disabled || atStart}
        onClick={onRestart}
      >
        从头再来
      </Button>
      <Button
        variant="on-dark"
        size="sm"
        leftIcon={StepBack}
        disabled={disabled || atStart}
        onClick={onStepBack}
      >
        上一步
      </Button>
      <Button
        variant="on-dark"
        size="sm"
        leftIcon={playing ? Pause : Play}
        disabled={disabled || atEnd}
        onClick={onToggle}
      >
        {playing ? '暂停' : playLabel}
      </Button>
      <Button
        variant="on-dark"
        size="sm"
        leftIcon={StepForward}
        disabled={disabled || atEnd}
        onClick={onStep}
      >
        下一步
      </Button>
    </div>
  )
}
