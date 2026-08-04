// 竞赛队伍卡(竞赛详情页右栏)。
// 队伍编号来自本人战绩记录(GET /contest/my/contest-records 的 team_id)——
// 学生侧唯一能取到自己 team_id 的地方(对齐清单 §3.1)。
// 队长可锁定队伍;锁定后不能再加入新成员,故按危险操作对待:先确认再执行。

import { useCallback, useState } from 'react'
import { Copy, Lock, Users } from 'lucide-react'
import { TeamStatus, type ContestTeam } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  Modal,
  ModalBody,
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
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { teamStatusLabel, teamStatusTone } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

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
    <Card>
      <CardHeader title="我的队伍" description="队伍成员共享参赛记录与得分。" />
      <CardBody>
        <ResourceState
          resource={team}
          emptyIcon={Users}
          emptyTitle="暂无队伍信息"
          emptyDescription="报名成功后会显示你的队伍与成员。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {(data) => (
            <TeamDetail
              team={data}
              onChanged={() => {
                team.reload()
                onChanged()
              }}
            />
          )}
        </ResourceState>
      </CardBody>
    </Card>
  )
}

interface TeamDetailProps {
  team: ContestTeam
  onChanged: () => void
}

/**
 * TeamDetail 渲染队伍档案、邀请码与锁定动作。
 */
function TeamDetail({ team, onChanged }: TeamDetailProps) {
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [locking, setLocking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const [copied, setCopied] = useState(false)

  const locked = team.status === TeamStatus.LOCKED

  /** copyInviteCode 复制邀请码,复制失败给就近提示而不是静默。 */
  const copyInviteCode = useCallback(async () => {
    if (!team.invite_code) return
    try {
      await navigator.clipboard.writeText(team.invite_code)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch (error) {
      setActionError('复制没有成功,请手动选中邀请码后复制。')
      console.error('队伍邀请码复制失败', {
        operation: 'contest.team.copyInviteCode',
        reason: 'clipboard-write-failed',
        error,
      })
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
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="min-w-0 truncate text-base font-medium text-ink">{team.name}</span>
        <StatusIndicator tone={teamStatusTone(team.status)} label={teamStatusLabel(team.status)} />
      </div>

      <DescriptionList
        dense
        items={[
          { term: '成员人数', description: `${team.members.length} 人`, mono: true },
          { term: '组队时间', description: formatShortDateTime(team.created_at), mono: true },
        ]}
      />

      <div className="flex flex-col gap-2">
        {team.members.map((member, index) => (
          <div key={member.id} className="flex items-center justify-between gap-2">
            <span className="text-sm text-ink-sub">
              {member.is_leader ? '队长' : `队员 ${index + 1}`}
            </span>
            {member.is_leader ? <Badge tone="jade">队长</Badge> : null}
          </div>
        ))}
      </div>

      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

      {team.invite_code && !locked ? (
        <div className="flex flex-col gap-2 rounded-md border border-line bg-surface-sunken p-3">
          <span className="text-xs text-ink-sub">队伍邀请码(发给队友即可加入)</span>
          <div className="flex items-center justify-between gap-2">
            <span className="font-mono text-md tabular-nums text-ink">{team.invite_code}</span>
            <Button variant="ghost" size="sm" leftIcon={Copy} onClick={() => void copyInviteCode()}>
              {copied ? '已复制' : '复制'}
            </Button>
          </div>
        </div>
      ) : null}

      {locked ? (
        <Callout tone="info">队伍已锁定,不能再加入新成员。</Callout>
      ) : (
        <Button variant="outline" leftIcon={Lock} onClick={() => setConfirmOpen(true)}>
          锁定队伍
        </Button>
      )}

      <Modal open={confirmOpen} onOpenChange={setConfirmOpen}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认锁定队伍</ModalTitle>
            <ModalDescription>
              锁定后队伍不能再加入新成员,邀请码随之失效。开赛时系统也会自动锁定队伍。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <DescriptionList
              dense
              items={[
                { term: '队伍名称', description: team.name },
                { term: '当前成员', description: `${team.members.length} 人`, mono: true },
              ]}
            />
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              先不锁定
            </Button>
            <Button variant="danger" loading={locking} onClick={() => void lockTeam()}>
              确认锁定
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </div>
  )
}
