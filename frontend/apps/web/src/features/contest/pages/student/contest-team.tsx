// 竞赛队伍卡(竞赛详情右栏「我的队伍」)。设计 §7.2:队名、成员、邀请码复制、锁队状态。
// 队伍编号来自本人战绩记录(GET /contest/my/contest-records 的 team_id)——
// 学生侧唯一能取到自己 team_id 的地方(对齐清单 §3.1)。
// 只有队长能锁队(后端同口径);锁定后不能再加入新成员,故先确认再执行。
// 成员接口只回账号编号,故按「队长 / 队员」与「我」呈现,内部编号不进界面。

import { useCallback, useState } from 'react'
import { Copy, Lock } from 'lucide-react'
import { TeamStatus, type ContestTeam } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Modal,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource } from '../../../../hooks'
import { copyText } from '../../../../utils/clipboard'
import { teamStatusLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { teamStatusTone } from '../../statusPresentation'

export interface ContestTeamCardProps {
  teamId: string
  onChanged: () => void
}

/**
 * ContestTeamCard 展示我的队伍与成员,并提供邀请码分享与锁定。
 */
export function ContestTeamCard({ teamId, onChanged }: ContestTeamCardProps) {
  const team = useAsyncResource(() => api.contest.getTeam(teamId), [teamId], () => false)

  return (
    <section aria-labelledby="contest-team-title" className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="contest-team-title" className="text-sm font-medium text-ink-sub">
        我的队伍
      </h2>
      {team.status === 'loading' ? (
        <div className="mt-3 flex flex-col gap-2" aria-busy="true">
          <Skeleton variant="title" width="half" />
          <Skeleton variant="line" lines={2} />
        </div>
      ) : team.status === 'error' || team.data === null ? (
        <ResourceError
          message={team.error?.message ?? '队伍信息暂时读取不到'}
          traceId={team.error?.traceId}
          onRetry={team.reload}
        />
      ) : (
        <TeamDetail
          team={team.data}
          onChanged={() => {
            team.reload()
            onChanged()
          }}
        />
      )}
    </section>
  )
}

/** TeamDetail 队名与状态、成员、邀请码、锁队 */
function TeamDetail({ team, onChanged }: { team: ContestTeam; onChanged: () => void }) {
  const { me } = useSession()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [locking, setLocking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const [copied, setCopied] = useState(false)

  const locked = team.status === TeamStatus.LOCKED
  const iAmLeader = team.members.some((member) => member.is_leader && member.account_id === me.account.id)
  const members = [...team.members].sort((a, b) => Number(b.is_leader) - Number(a.is_leader))

  /** copyInviteCode 复制邀请码;失败提示与结构化日志由全站剪贴板入口统一处理。 */
  const copyInviteCode = useCallback(async () => {
    if (!team.invite_code) return
    setActionError(undefined)
    const ok = await copyText(team.invite_code, { what: '队伍邀请码', operation: 'contest.team.copyInviteCode' })
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    }
  }, [team.invite_code])

  /** lockTeam 锁定队伍:锁定后不能再加入新成员,故先确认。 */
  const lockTeam = useCallback(async () => {
    setLocking(true)
    setActionError(undefined)
    try {
      await api.contest.lockTeam(team.id)
      setConfirmOpen(false)
      toast.success('队伍已锁定')
      onChanged()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '锁定队伍没有成功,请稍后重试。'))
    } finally {
      setLocking(false)
    }
  }, [onChanged, team.id])

  return (
    <div className="mt-2 flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="min-w-0 truncate text-lg font-semibold text-ink">{team.name}</span>
        <StatusIndicator tone={teamStatusTone(team.status)} label={teamStatusLabel(team.status)} />
      </div>

      <ul className="flex flex-col gap-2">
        {members.map((member, index) => (
          <li key={member.id} className="flex items-center gap-2 text-sm text-ink">
            {member.is_leader ? '队长' : `队员 ${index}`}
            {member.account_id === me.account.id ? <Badge tone="jade">我</Badge> : null}
          </li>
        ))}
      </ul>

      {team.invite_code && !locked ? (
        <div className="rounded-md bg-surface-hover p-3">
          <p className="text-xs text-ink-sub">队伍邀请码,发给队友即可加入</p>
          <div className="mt-1 flex items-center justify-between gap-2">
            <span className="font-mono text-md tabular-nums text-ink">{team.invite_code}</span>
            <Button variant="ghost" size="sm" leftIcon={Copy} onClick={() => void copyInviteCode()}>
              {copied ? '已复制' : '复制'}
            </Button>
          </div>
        </div>
      ) : null}

      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

      {locked ? (
        <p className="flex items-center gap-1.5 text-sm text-ink-sub">
          <Lock aria-hidden="true" className="size-4" />
          队伍已锁定,不能再加入新成员。
        </p>
      ) : iAmLeader ? (
        <Button variant="outline" leftIcon={Lock} onClick={() => setConfirmOpen(true)}>
          锁定队伍
        </Button>
      ) : (
        <p className="text-sm text-ink-sub">队长可以提前锁定队伍;开赛时系统也会自动锁定。</p>
      )}

      <Modal open={confirmOpen} onOpenChange={setConfirmOpen}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>锁定「{team.name}」?</ModalTitle>
            <ModalDescription>
              现在队里有 {team.members.length} 人。锁定后不能再加入新成员,邀请码随之失效;开赛时系统也会自动锁定。
            </ModalDescription>
          </ModalHeader>
          <ModalFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              先不锁定
            </Button>
            <Button variant="danger" loading={locking} onClick={() => void lockTeam()}>
              锁定队伍
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  )
}
