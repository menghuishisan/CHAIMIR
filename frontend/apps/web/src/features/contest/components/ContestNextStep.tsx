// 赛事列表与详情共用的赛程操作及确认弹窗。

import { useState } from 'react'
import { Archive, Lock, Play, Send, Snowflake, type LucideIcon } from 'lucide-react'
import type { Contest } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  cn,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { contestNextStep, type LifecycleAction } from '../contestLifecycle'

const ACTION_COPY: Record<
  LifecycleAction,
  { label: string; icon: LucideIcon; title: string; description: string; confirm: string; seal: boolean }
> = {
  publish: {
    label: '发布',
    icon: Send,
    title: '发布这场赛事?',
    description: '发布后学生就能看到赛事,并在报名窗口内报名。发布需要至少一道赛题,发布后赛题不能再修改。',
    confirm: '确认发布',
    seal: true,
  },
  start: {
    label: '开赛',
    icon: Play,
    title: '现在开始比赛?',
    description: '开赛后报名关闭、队伍自动锁定,学生可以进入答题。开赛不能撤销。',
    confirm: '确认开赛',
    seal: false,
  },
  freeze: {
    label: '封榜',
    icon: Snowflake,
    title: '现在封榜?',
    description: '封榜后榜单对学生停止更新,学生仍可以继续提交;比赛结束后名次一次性揭晓。',
    confirm: '确认封榜',
    seal: false,
  },
  end: {
    label: '结束比赛',
    icon: Lock,
    title: '现在结束比赛?',
    description: '结束后学生不能再提交,榜单恢复显示最终名次。结束不能撤销。',
    confirm: '确认结束',
    seal: false,
  },
  archive: {
    label: '归档',
    icon: Archive,
    title: '归档这场赛事?',
    description: '归档会生成最终榜单快照并回收比赛环境,之后赛事只读。归档不能撤销。',
    confirm: '确认归档',
    seal: true,
  },
}

/** runLifecycleAction 执行赛事状态流转。 */
async function runLifecycleAction(action: LifecycleAction, contestId: string): Promise<void> {
  if (action === 'publish') await api.contest.publishContest(contestId)
  else if (action === 'start') await api.contest.startContest(contestId)
  else if (action === 'freeze') await api.contest.freezeContest(contestId)
  else if (action === 'end') await api.contest.endContest(contestId)
  else await api.contest.archiveContest(contestId)
}

export interface ContestNextStepButtonProps {
  contest: Contest
  /** 行内用 sm 的次要按钮;页头用 md 的主按钮(发布、归档为朱砂) */
  placement: 'row' | 'header'
  /** 流转成功后(弹窗已关闭)通知调用方重读 */
  onDone: () => void
  /** 由页面补充的发布阻塞原因，如未添加赛题。 */
  publishBlocker?: string
}

/** ContestNextStepButton 按赛程与发布条件提供下一步操作。 */
export function ContestNextStepButton({ contest, placement, onDone, publishBlocker }: ContestNextStepButtonProps) {
  const [open, setOpen] = useState(false)
  const [working, setWorking] = useState(false)
  const [error, setError] = useState<string>()
  const timed = contestNextStep(contest)
  const step =
    timed && timed.action === 'publish' && timed.available && publishBlocker
      ? { action: timed.action, available: false, reason: publishBlocker }
      : timed

  const action = step?.action

  async function confirm() {
    if (!action) return
    setWorking(true)
    setError(undefined)
    try {
      await runLifecycleAction(action, contest.id)
      toast.success(action === 'archive' ? '赛事已归档,最终榜单已生成' : `已${ACTION_COPY[action].label}`)
      setOpen(false)
      onDone()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

  if (!step) return null
  const copy = ACTION_COPY[step.action]
  const button = (
    <Button
      variant={placement === 'header' ? (copy.seal ? 'seal' : 'primary') : 'secondary'}
      size={placement === 'header' ? 'md' : 'sm'}
      leftIcon={copy.icon}
      disabled={!step.available}
      onClick={() => {
        setError(undefined)
        setOpen(true)
      }}
    >
      {copy.label}
    </Button>
  )

  return (
    <>
      {step.available ? (
        button
      ) : (
        <span
          className={cn(
            'inline-flex gap-1.5',
            placement === 'row' ? 'flex-col items-end text-right' : 'flex-row-reverse flex-wrap items-center justify-end',
          )}
        >
          {button}
          <span className={cn('text-ink-sub', placement === 'row' ? 'max-w-48 text-xs' : 'text-sm')}>{step.reason}</span>
        </span>
      )}
      <Modal open={open} onOpenChange={(next) => !working && setOpen(next)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>{copy.title}</ModalTitle>
            <ModalDescription>{copy.description}</ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-3">
            <p className="text-base font-medium text-ink">{contest.name}</p>
            {error ? <Callout tone="danger">{error}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" disabled={working} onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button variant={copy.seal ? 'seal' : 'primary'} loading={working} onClick={() => void confirm()}>
              {copy.confirm}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </>
  )
}
