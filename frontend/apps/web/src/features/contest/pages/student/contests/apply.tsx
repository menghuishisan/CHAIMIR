// 学生竞赛报名页：调用后端报名、加入队伍和锁定队伍接口。

import React, { useState } from 'react'
import type { ContestTeam } from '@chaimir/api-client'
import { Button, Input } from '@chaimir/ui'
import { Lock, UserPlus, Users } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import styles from '../../contest.module.css'

const StudentContestApplyPage: React.FC = () => {
  const { id } = useParams()
  const [teamName, setTeamName] = useState('')
  const [teamId, setTeamId] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [team, setTeam] = useState<ContestTeam | null>(null)
  const [message, setMessage] = useState('')

  const signup = async () => {
    if (!id || !teamName.trim()) return
    setMessage('')
    try {
      const result = await api.contest.signup(id, { team_name: teamName.trim() })
      setTeam(result)
      setTeamId(result.id)
      setMessage('报名已提交。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法提交报名。')
    }
  }

  const join = async () => {
    if (!teamId.trim() || !inviteCode.trim()) return
    setMessage('')
    try {
      const result = await api.contest.joinTeam(teamId.trim(), { invite_code: inviteCode.trim() })
      setTeam(result)
      setMessage('已加入队伍。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法加入队伍。')
    }
  }

  const lock = async () => {
    if (!teamId.trim()) return
    setMessage('')
    try {
      const result = await api.contest.lockTeam(teamId.trim())
      setTeam(result)
      setMessage('队伍名单已锁定。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法锁定队伍。')
    }
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

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>创建或更新报名队伍</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="team-name">队伍名称</label>
            <Input id="team-name" value={teamName} onChange={(event) => setTeamName(event.target.value)} fullWidth />
          </div>
          <Button icon={<Users size={16} />} onClick={signup}>提交报名</Button>
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
            <Button variant="outline" onClick={join}>加入队伍</Button>
            <Button variant="outline" icon={<Lock size={16} />} onClick={lock}>锁定名单</Button>
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
