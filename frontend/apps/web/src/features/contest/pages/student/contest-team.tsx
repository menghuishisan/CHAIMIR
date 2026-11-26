// 学生竞赛队伍：成员、邀请码分享与队长锁队。

import { useState } from 'react'
import { Copy, Link as LinkIcon, Lock } from 'lucide-react'
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

/** ContestTeamCard 读取队伍并在操作后刷新成员与状态。 */
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

/** TeamDetail 展示成员、分享入队入口并处理队长锁队。 */
function TeamDetail({ team, onChanged }: { team: ContestTeam; onChanged: () => void }) {
  const { me } = useSession()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [locking, setLocking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const locked = team.status === TeamStatus.LOCKED
  const iAmLeader = team.members.some((member) => member.is_leader && member.account_id === me.account.id)
  const members = [...team.members].sort((a, b) => Number(b.is_leader) - Number(a.is_leader))

  /** copyInviteCode 复用共享剪贴板入口的提示与错误处理。 */
  async function copyInviteCode() {
    if (!team.invite_code) return
    setActionError(undefined)
    const ok = await copyText(team.invite_code, { what: '队伍邀请码', operation: 'contest.team.copyInviteCode' })
    if (ok) toast.success('邀请码已复制')
  }

  /** copyJoinLink 分享同源入队地址，邀请码始终单独传递。 */
  async function copyJoinLink() {
    const link = new URL(`/student/contests/${team.contest_id}/join`, window.location.origin).href
    const ok = await copyText(link, { what: '入队链接', operation: 'contest.team.copyJoinLink' })
    if (ok) toast.success('入队链接已复制')
  }

  /** lockTeam 锁定队伍:锁定后不能再加入新成员,故先确认。 */
  async function lockTeam() {
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
  }

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
          <p className="text-xs text-ink-sub">把入队链接和邀请码分别发给队友，登录后即可加入。</p>
          <div className="mt-1 flex items-center justify-between gap-2">
            <span className="font-mono text-md tabular-nums text-ink">{team.invite_code}</span>
            <Button variant="ghost" size="sm" leftIcon={Copy} onClick={copyInviteCode}>
              复制邀请码
            </Button>
          </div>
          <Button variant="ghost" size="sm" leftIcon={LinkIcon} onClick={copyJoinLink}>
            复制入队链接
          </Button>
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
