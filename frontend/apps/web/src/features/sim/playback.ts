// 仿真运行时页面的共享播放控制与进度派生。
//
// 仿真工作台(带/不带会话)与公开回放页做的是同一件事:按固定节奏把 Worker 往前推,
// 允许单步、暂停与变速(M4 需求 C2/C3)。两页的差别只有"一步是什么":工作台推进一个
// 推演时刻,回放页按记录走下一条动作。故把节奏、速度档与调度收敛到这里,页面只提供"走一步"。
//
// 为什么调度在页面层而不在 SimWorkerClient 里:每一步必须等上一步的快照回到主线程后再排下一步,
// 慢帧才不会让命令堆积;client 内的固定间隔定时器做不到这件事(它不知道快照何时到达),
// 那套定时器已随本文件一并删除,不保留两套播放实现。

import { useCallback, useEffect, useState } from 'react'
import type { RuntimeSnapshot, SimPackageDescriptor } from '@chaimir/sim-sdk'
import { appConfig } from '../../app/config'

/** 播放速度档:倍数越大节奏越快。1 倍是部署配置给出的教学基准节奏。 */
export interface SimSpeedOption {
  label: string
  multiplier: number
}

/** 速度档位:讲解时放慢、复看时加速;不做连续调速滑块(挡位少一点更好选)。 */
export const SIM_SPEED_OPTIONS: readonly SimSpeedOption[] = [
  { label: '0.5 倍', multiplier: 0.5 },
  { label: '1 倍', multiplier: 1 },
  { label: '2 倍', multiplier: 2 },
  { label: '4 倍', multiplier: 4 },
] as const

/** 默认速度:教学基准节奏。 */
const DEFAULT_MULTIPLIER = 1

export interface SimPlaybackOptions {
  /** 走一步:工作台推进一个推演时刻,回放页复现下一条记录动作 */
  advance: () => void
  /** 是否还能继续自动播放(出错或已到末尾时为 false) */
  canAdvance: boolean
  /** 上一步已落地的信号:它变化即说明快照已回到主线程,可以排下一步 */
  cue: unknown
  /** 初始是否自动播放(减弱动效偏好下由调用方传 false) */
  autoPlay?: boolean
}

export interface SimPlaybackState {
  playing: boolean
  /** 切换播放/暂停 */
  toggle: () => void
  /** 显式停止自动播放(单步、回退、重播等手动操作后调用) */
  stop: () => void
  multiplier: number
  setMultiplier: (multiplier: number) => void
}

/**
 * useSimPlayback 管理自动播放节奏与速度档。
 * 每次 cue 变化后按当前速度排下一步,组件卸载或暂停时清掉待执行的调度。
 */
export function useSimPlayback({ advance, canAdvance, cue, autoPlay = false }: SimPlaybackOptions): SimPlaybackState {
  const [playing, setPlaying] = useState(autoPlay)
  const [multiplier, setMultiplier] = useState(DEFAULT_MULTIPLIER)

  useEffect(() => {
    if (!playing || !canAdvance) return
    // cue 不在 effect 体内使用,但必须进依赖:它是「上一步已落地」的信号,变化才排下一步。
    // 这样调度天然自适应慢帧 —— 快照没回来就不会排出下一条命令。
    const timer = setTimeout(advance, Math.round(appConfig.simStepIntervalMs / multiplier))
    return () => clearTimeout(timer)
  }, [advance, canAdvance, cue, multiplier, playing])

  const toggle = useCallback(() => setPlaying((current) => !current), [])
  const stop = useCallback(() => setPlaying(false), [])

  return { playing, toggle, stop, multiplier, setMultiplier }
}

/**
 * narrativeProgress 把当前叙事步骤换成链式进度的完成数:讲到第几步,前面几步就都讲过了。
 * 叙事步骤由状态断言命中(浏览器 Worker 与隔离容器同一套引擎求值),
 * 尚未命中任何一步时进度为 0。两个执行位置的快照都只用到 currentStep,故按这一个字段收窄参数。
 */
export function narrativeProgress(
  descriptor: SimPackageDescriptor,
  snapshot: Pick<RuntimeSnapshot, 'currentStep'>,
): number {
  const currentId = snapshot.currentStep?.id
  return descriptor.narrative.findIndex((step) => step.id === currentId) + 1
}
