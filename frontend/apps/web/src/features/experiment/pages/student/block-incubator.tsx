import { Blocks, CircleCheck, CircleX, Lock, Play, Unlock } from 'lucide-react'
import {
  EXPERIMENT_STAGE_STATUS,
  ExperimentInstanceStatus,
  JudgeTaskStatusCode,
  type CheckpointResult,
  type StageState,
  type StudentCheckpointConfig,
} from '@chaimir/api-client'
import { Badge, Button, ChainProgress } from '@chaimir/ui'
import { experimentStageStatusLabel } from '../../../../utils/labels/experiment'

/** 顶栏和检查点面板共用工作台的判分动作。 */
export interface CheckpointJudge {
  judgingId: string | undefined
  judge: (checkpointId: string) => Promise<void>
}

type BlockPhase = 'pending' | 'incubating' | 'minted' | 'failed' | 'error' | 'cancelled'

const PHASE_LABELS: Record<BlockPhase, string> = {
  pending: '待孵化',
  incubating: '孵化中',
  minted: '已铸块',
  failed: '未成块',
  error: '判分未完成',
  cancelled: '判分已取消',
}

export interface BlockIncubatorProps {
  checkpoints: StudentCheckpointConfig[]
  results: CheckpointResult[]
  stages: StageState[]
  instanceStatus: ExperimentInstanceStatus
  busy: boolean
  /** 只有实例处于 running/paused 时才允许触发检查点判分。 */
  canJudge: boolean
  /** 判分状态与动作:由工作台持有,顶栏的「判分」与这里的按钮共用同一份 */
  judging: CheckpointJudge
  /** 激活一个已解锁阶段 */
  onActivateStage: (stage: number) => void
}

export function BlockIncubator({
  checkpoints,
  results,
  stages,
  instanceStatus,
  busy,
  canJudge,
  judging,
  onActivateStage,
}: BlockIncubatorProps) {
  const { judgingId, judge } = judging

  const resultById = new Map(results.map((result) => [result.id, result]))
  const blocks = checkpoints.map((checkpoint) => {
    const result = resultById.get(checkpoint.id)
    const phase = judgingId === checkpoint.id ? 'incubating' : resultPhase(result)
    return { checkpoint, result, phase }
  })

  const minted = blocks.filter((block) => block.phase === 'minted').length
  const failedIndexes = blocks.reduce<number[]>((indexes, block, index) => {
    if (block.phase === 'failed') indexes.push(index)
    return indexes
  }, [])

  const earned = results.reduce(
    (sum, result) => sum + (result.judge_status === JudgeTaskStatusCode.DONE ? result.score : 0),
    0,
  )
  const total = checkpoints.reduce((sum, checkpoint) => sum + checkpoint.score, 0)

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Blocks aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">区块孵化器</h2>
        </div>
        <p className="text-xs text-on-dark-sub">
          每个区块对应一个检查点,重新判分后以最新结果为准。
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
                    {instanceStatus === ExperimentInstanceStatus.PAUSED ? (
                      <p className="text-xs text-on-dark-faint">请先恢复实验，再进入这个阶段。</p>
                    ) : (
                      <Button
                        variant="on-dark"
                        size="sm"
                        leftIcon={Unlock}
                        disabled={busy || instanceStatus !== ExperimentInstanceStatus.RUNNING}
                        onClick={() => onActivateStage(stage.stage)}
                      >
                        进入这个阶段
                      </Button>
                    )}
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
                  {block.result && block.result.judge_status === JudgeTaskStatusCode.DONE
                    ? `得分 ${block.result.score}/${block.checkpoint.score}`
                    : block.phase === 'incubating'
                      ? '判分正在处理,请稍后刷新结果'
                      : block.phase === 'error' || block.phase === 'cancelled'
                        ? '请重新判分后查看结果'
                        : `分值 ${block.checkpoint.score}`}
                </p>
                <div>
                  <Button
                    variant={block.phase === 'minted' ? 'on-dark' : 'seal'}
                    size="sm"
                    leftIcon={Play}
                    loading={block.phase === 'incubating'}
                    disabled={!canJudge || judgingId !== undefined}
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

      {!canJudge && blocks.length > 0 ? (
        <p className="text-xs text-on-dark-sub">实验状态或环境状态暂不支持判分，请刷新状态后重试。</p>
      ) : null}

    </div>
  )
}

function resultPhase(result?: CheckpointResult): BlockPhase {
  if (!result) return 'pending'
  if (result.judge_status === JudgeTaskStatusCode.QUEUED || result.judge_status === JudgeTaskStatusCode.JUDGING) return 'incubating'
  if (result.judge_status === JudgeTaskStatusCode.FAILED) return 'error'
  if (result.judge_status === JudgeTaskStatusCode.CANCELLED) return 'cancelled'
  return result.passed ? 'minted' : 'failed'
}

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

function unlockHint(type: 'checkpoint' | 'manual', minScore?: number): string {
  if (type === 'manual') return '这个阶段由老师手动放行。'
  return minScore !== undefined && minScore > 0
    ? `上一个检查点拿到 ${minScore} 分以上后解锁。`
    : '通过上一个检查点后解锁。'
}
