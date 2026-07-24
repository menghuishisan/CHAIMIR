// 学生竞赛报名页：调用后端报名、加入队伍和锁定队伍接口。

import React, { useState } from 'react'
import type { ContestTeam } from '@chaimir/api-client'
import { Button, Input } from '@chaimir/ui'
import { Lock, UserPlus, Users } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../contest.module.css'

const StudentContestApplyPage: React.FC = () => {
  const { id } = useParams()
  const [teamName, setTeamName] = useState('')
  const [teamId, setTeamId] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [team, setTeam] = useState<ContestTeam | null>(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [pendingAction, setPendingAction] = useState('')

  /** runAction 串行执行队伍操作，避免重复提交并统一错误反馈。 */
  const runAction = async (key: string, action: () => Promise<ContestTeam>, success: string) => {
    if (pendingAction) return
    setPendingAction(key)
    setMessage('')
    setError('')
    try {
      setTeam(await action())
      setMessage(success)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '队伍操作未完成，请稍后重试。'))
    } finally {
      setPendingAction('')
    }
  }

  const signup = async () => {
    if (!id || !teamName.trim()) return
    await runAction('signup', async () => {
      const result = await api.contest.signup(id, { team_name: teamName.trim() })
      setTeamId(result.id)
      return api.contest.getTeam(result.id)
    }, '报名已提交。')
  }

  const join = async () => {
    if (!teamId.trim() || !inviteCode.trim()) return
    await runAction('join', async () => {
      const result = await api.contest.joinTeam(teamId.trim(), { invite_code: inviteCode.trim() })
      return api.contest.getTeam(result.id)
    }, '已加入队伍。')
  }

  const lock = async () => {
    if (!teamId.trim()) return
    await runAction('lock', () => api.contest.lockTeam(teamId.trim()), '队伍名单已锁定。')
  }

  /** refreshTeam 按队伍编号读取服务端最新成员和锁定状态。 */
  const refreshTeam = async () => {
    if (!teamId.trim()) return
    await runAction('refresh', () => api.contest.getTeam(teamId.trim()), '队伍信息已更新。')
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>学生端 / 竞赛中心 / 竞赛报名</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <UserPlus className={styles.titleIcon} size={28} />
          竞赛报名
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}
      {error && <p className={styles.message} role="alert">{error}</p>}

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>创建或更新报名队伍</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="team-name">队伍名称</label>
            <Input id="team-name" value={teamName} onChange={(event) => setTeamName(event.target.value)} fullWidth />
          </div>
          <Button icon={<Users size={16} />} loading={pendingAction === 'signup'} disabled={Boolean(pendingAction) || !teamName.trim()} onClick={signup}>提交报名</Button>
        </section>
        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>加入已有队伍</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="team-id">队伍编号</label>
            <Input id="team-id" value={teamId} onChange={(event) => setTeamId(event.target.value)} fullWidth />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="invite-code">邀请码</label>
            <Input id="invite-code" value={inviteCode} onChange={(event) => setInviteCode(event.target.value)} fullWidth />
          </div>
          <div className={styles.actions}>
            <Button variant="outline" loading={pendingAction === 'join'} disabled={Boolean(pendingAction) || !teamId.trim() || !inviteCode.trim()} onClick={join}>加入队伍</Button>
            <Button variant="outline" icon={<Lock size={16} />} loading={pendingAction === 'lock'} disabled={Boolean(pendingAction) || !teamId.trim()} onClick={lock}>锁定名单</Button>
            <Button variant="ghost" loading={pendingAction === 'refresh'} disabled={Boolean(pendingAction) || !teamId.trim()} onClick={() => void refreshTeam()}>刷新队伍</Button>
          </div>
        </aside>
      </div>

      {team && (
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>当前队伍</h2>
          <p className={styles.muted}>队伍 {team.name}，编号 {team.id}{team.invite_code ? `，邀请码 ${team.invite_code}` : ''}</p>
          <p className={styles.muted}>成员 {team.members.length} 人</p>
        </section>
      )}
    </div>
  )
}

export default StudentContestApplyPage
