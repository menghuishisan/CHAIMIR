// 区块孵化器(实验工作台右栏)。
//
// 这是阶段 5 的创新①:把实验的检查点做成一条可见的「链」。
// 为什么这样做 —— 实验的判分点在数据上就是一串独立结果(通过/未通过 + 得分),
// 学生看到的却常是一个笼统的百分比,不知道自己卡在哪一环、下一环要做什么。
// 平台的进度语言本来就是链式区块(设计规范 §5.4),于是把每个检查点当成一个待铸的区块:
//   待孵化 = 还没判过;孵化中 = 判分正在跑;已铸块 = 判过且通过;未成块 = 判过但没通过。
// 铸块是不可逆的既成事实(通过就记分),这与区块的语义一致;未成块可以重判,也与之一致。
//
// 全部状态取自真实数据:阶段来自实例的 stages,区块来自实例的 checkpoints 与实验的检查点配置。
// 没有任何为了好看而造的中间态 —— 后端没给的状态这里就不显示。

import { useCallback, useMemo, useState } from 'react'
import { Blocks, CircleCheck, CircleX, Lock, Play, Unlock } from 'lucide-react'
import {
  EXPERIMENT_STAGE_STATUS,
  type CheckpointResult,
  type StageState,
  type StudentCheckpointConfig,
} from '@chaimir/api-client'
import { Badge, Button, ChainProgress, toast } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { experimentStageStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** BlockPhase 是一个检查点区块的四种形态。 */
type BlockPhase = 'pending' | 'incubating' | 'minted' | 'failed'

const PHASE_LABELS: Record<BlockPhase, string> = {
  pending: '待孵化',
  incubating: '孵化中',
  minted: '已铸块',
  failed: '未成块',
}

/** 已保存的代码引用:判分要拿这一刻的代码快照,不是让后端再去容器里抓。 */
export interface WorkspaceCodeRef {
  codeStorageKey: string
  codeHash: string
}

export interface BlockIncubatorProps {
  instanceId: string
  /** 实验配置里的检查点(学生投影:只有编号与分值,没有判题器与题目配置) */
  checkpoints: StudentCheckpointConfig[]
  /** 实例上的判分结果 */
  results: CheckpointResult[]
  /** 实例阶段状态 */
  stages: StageState[]
  /** 最近一次「保存工作区」的产物;没有保存过时判分按钮会说明原因 */
  codeRef?: WorkspaceCodeRef
  /** 判分完成后请调用方重新读取实例 */
  onJudged: () => void
  /** 激活一个已解锁阶段 */
  onActivateStage: (stage: number) => void
}

/**
 * BlockIncubator 渲染阶段链与检查点区块,并承载判分。
 */
export function BlockIncubator({
  instanceId,
  checkpoints,
  results,
  stages,
  codeRef,
  onJudged,
  onActivateStage,
}: BlockIncubatorProps) {
  const [judgingId, setJudgingId] = useState<string>()
  const [panelError, setPanelError] = useState<string>()

  const resultById = useMemo(
    () => new Map(results.map((result) => [result.id, result])),
    [results],
  )

  const blocks = useMemo(
    () =>
      checkpoints.map((checkpoint) => {
        const result = resultById.get(checkpoint.id)
        const phase: BlockPhase =
          judgingId === checkpoint.id
            ? 'incubating'
            : result === undefined
              ? 'pending'
              : result.passed
                ? 'minted'
                : 'failed'
        return { checkpoint, result, phase }
      }),
    [checkpoints, judgingId, resultById],
  )

  const minted = blocks.filter((block) => block.phase === 'minted').length
  const failedIndexes = blocks.reduce<number[]>((indexes, block, index) => {
    if (block.phase === 'failed') indexes.push(index)
    return indexes
  }, [])

  const earned = results.reduce((sum, result) => sum + result.score, 0)
  const total = checkpoints.reduce((sum, checkpoint) => sum + checkpoint.score, 0)

  /**
   * judge 触发一个检查点判分。
   * 代码引用来自最近一次保存工作区:判分对象必须是一个确定的快照,
   * 否则重判时拿到的代码可能已经变了,分数就不可复现。
   */
  const judge = useCallback(
    async (checkpointId: string) => {
      setJudgingId(checkpointId)
      setPanelError(undefined)
      try {
        const result = await api.experiment.judgeCheckpoint(instanceId, checkpointId, {
          code_storage_key: codeRef?.codeStorageKey,
          code_hash: codeRef?.codeHash,
        })
        toast.success(result.passed ? '判分通过,区块已铸成' : '判分完成,这次没有通过')
        onJudged()
      } catch (error) {
        setPanelError(userFacingErrorMessage(error, '判分没有跑完,请稍后重试。'))
      } finally {
        setJudgingId(undefined)
      }
    },
    [codeRef, instanceId, onJudged],
  )

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Blocks aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">区块孵化器</h2>
        </div>
        <p className="text-xs text-on-dark-sub">
          每通过一个检查点就铸成一个区块。铸成的区块不会回退,没成的可以改完再判一次。
        </p>
        <ChainProgress
          onDark
          size="md"
          label="已铸区块"
          total={blocks.length}
          done={minted}
          failedIndexes={failedIndexes}
        />
        <p className="font-mono text-xs tabular-nums text-on-dark-sub">
          链高 {minted}/{blocks.length} · 已得 {earned}/{total} 分
        </p>
      </section>

      {stages.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-on-dark-sub">实验阶段</h3>
          <ul className="flex flex-col gap-2">
            {stages.map((stage) => (
              <li
                key={stage.stage}
                className="flex flex-col gap-1 rounded-md border border-dark-line bg-dark-surface p-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="min-w-0 truncate text-sm text-on-dark">
                    第 {stage.stage} 阶段 · {stage.title}
                  </span>
                  <StageBadge status={stage.status} />
                </div>
                {stage.description ? (
                  <p className="line-clamp-2 text-xs text-on-dark-sub">{stage.description}</p>
                ) : null}
                {stage.status === EXPERIMENT_STAGE_STATUS.AVAILABLE ? (
                  <div>
                    <Button
                      variant="on-dark"
                      size="sm"
                      leftIcon={Unlock}
                      onClick={() => onActivateStage(stage.stage)}
                    >
                      进入这个阶段
                    </Button>
                  </div>
                ) : null}
                {stage.status === EXPERIMENT_STAGE_STATUS.LOCKED && stage.unlock_condition ? (
                  <p className="text-xs text-on-dark-faint">
                    {unlockHint(stage.unlock_condition.type, stage.unlock_condition.min_score)}
                  </p>
                ) : null}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-on-dark-sub">检查点区块</h3>
        {blocks.length === 0 ? (
          <p className="text-xs text-on-dark-sub">这个实验没有设自动判分点,完成后提交报告即可。</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {blocks.map((block, index) => (
              <li
                key={block.checkpoint.id}
                className="flex flex-col gap-2 rounded-md border border-dark-line bg-dark-surface p-2"
              >
                <div className="flex items-start justify-between gap-2">
                  <span className="min-w-0 text-sm text-on-dark">第 {index + 1} 块</span>
                  <PhaseBadge phase={block.phase} />
                </div>
                <p className="font-mono text-xs tabular-nums text-on-dark-sub">
                  {block.result
                    ? `得分 ${block.result.score}/${block.checkpoint.score}`
                    : `分值 ${block.checkpoint.score}`}
                </p>
                <div>
                  <Button
                    variant={block.phase === 'minted' ? 'on-dark' : 'seal'}
                    size="sm"
                    leftIcon={Play}
                    loading={block.phase === 'incubating'}
                    disabled={judgingId !== undefined}
                    onClick={() => void judge(block.checkpoint.id)}
                  >
                    {block.phase === 'minted' ? '重新判分' : '开始判分'}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {!codeRef && blocks.length > 0 ? (
        <p className="text-xs text-on-dark-sub">
          还没有保存过工作区。判分会按环境里当前的代码进行;先在代码面板点「保存工作区」,
          判的就是你确认过的那一份。
        </p>
      ) : null}

      {panelError ? <p className="text-xs text-on-dark-danger">{panelError}</p> : null}
    </div>
  )
}

/** StageBadge 渲染阶段状态。 */
function StageBadge({ status }: { status: StageState['status'] }) {
  if (status === EXPERIMENT_STAGE_STATUS.ACTIVE) {
    return <Badge tone="jade">{experimentStageStatusLabel(status)}</Badge>
  }
  if (status === EXPERIMENT_STAGE_STATUS.AVAILABLE) {
    return <Badge tone="info">{experimentStageStatusLabel(status)}</Badge>
  }
  return (
    <Badge tone="neutral">
      <span className="inline-flex items-center gap-1">
        <Lock aria-hidden="true" className="size-3" />
        {experimentStageStatusLabel(status)}
      </span>
    </Badge>
  )
}

/** PhaseBadge 渲染区块形态;通过与未通过各自配图标,不只靠颜色区分。 */
function PhaseBadge({ phase }: { phase: BlockPhase }) {
  if (phase === 'minted') {
    return (
      <Badge tone="success">
        <span className="inline-flex items-center gap-1">
          <CircleCheck aria-hidden="true" className="size-3" />
          {PHASE_LABELS.minted}
        </span>
      </Badge>
    )
  }
  if (phase === 'failed') {
    return (
      <Badge tone="danger">
        <span className="inline-flex items-center gap-1">
          <CircleX aria-hidden="true" className="size-3" />
          {PHASE_LABELS.failed}
        </span>
      </Badge>
    )
  }
  return <Badge tone={phase === 'incubating' ? 'info' : 'neutral'}>{PHASE_LABELS[phase]}</Badge>
}

/** unlockHint 用自然语言说明这个阶段怎么解锁,不显示内部检查点编号。 */
function unlockHint(type: 'checkpoint' | 'manual', minScore?: number): string {
  if (type === 'manual') return '这个阶段由老师手动放行。'
  return minScore !== undefined && minScore > 0
    ? `上一个检查点拿到 ${minScore} 分以上后解锁。`
    : '通过上一个检查点后解锁。'
}
