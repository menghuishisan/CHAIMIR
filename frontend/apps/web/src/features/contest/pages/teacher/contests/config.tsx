// 教师竞赛配置页：把竞赛规则和赛程保存到后端竞赛定义。

import React, { useEffect, useMemo, useState } from 'react'
import type { Contest, ContestRequest } from '@chaimir/api-client'
import { ContestMode, MatchMode, TeamMode } from '@chaimir/api-client'
import { Button, Input, Select, Textarea } from '@chaimir/ui'
import { Save, Settings } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import { defaultContestRequest } from '../../../config/contest'
import styles from '../../contest.module.css'
import { contestModeOptions, formatDateTimeLocalInput, matchModeOptions, parseDateTimeLocalInput, parseJsonObject, teamModeOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherContestConfigPage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const [form, setForm] = useState<ContestRequest>(defaultContestRequest)
  const [rulesText, setRulesText] = useState('{}')
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(
    async () => {
      if (!id) return null
      const response = await api.contest.getContests({ page: 1, size: 100 })
      return response.list.find((item) => item.id === id) ?? null
    },
    [id],
    () => false
  )

  useEffect(() => {
    if (!resource.data) return
    const request = toRequest(resource.data)
    setForm(request)
    setRulesText(JSON.stringify(request.rules, null, 2))
  }, [resource.data])

  const rules = useMemo(() => {
    try {
      return { ok: true, value: parseJsonObject(rulesText) }
    } catch {
      return { ok: false, value: {} }
    }
  }, [rulesText])

  const save = async () => {
    if (!rules.ok) {
      setMessage('竞赛规则不是有效的结构，请检查后再保存。')
      return
    }
    setMessage('')
    try {
      const payload = { ...form, rules: rules.value }
      const saved = id ? await api.contest.updateContest(id, payload) : await api.contest.createContest(payload)
      setMessage('竞赛配置已保存。')
      if (!id) navigate(`/teacher/contests/${saved.id}/config`, { replace: true })
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法保存竞赛配置。'))
    }
  }

  if (id && resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛配置" description="系统正在同步已保存的赛程和规则。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  if (id && resource.status === 'empty') {
    return <EmptyState title="未找到竞赛" description="该竞赛可能已删除或你没有访问权限。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 竞赛管理 / 竞赛配置</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Settings className={styles.titleIcon} size={28} />
          竞赛配置
        </h1>
        <Button icon={<Save size={16} />} onClick={save}>保存配置</Button>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <section className={`${styles.panel} ${styles.section}`}>
        <div className={styles.grid}>
          <div className={styles.field}><label className={styles.label} htmlFor="name">竞赛名称</label><Input id="name" value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} fullWidth /></div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="mode">竞赛模式</label>
            <Select id="mode" value={String(form.mode)} options={contestModeOptions} onChange={(value) => setForm((current) => ({ ...current, mode: Number(value) as ContestMode }))} />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="team-mode">组队模式</label>
            <Select id="team-mode" value={String(form.team_mode)} options={teamModeOptions} onChange={(value) => setForm((current) => ({ ...current, team_mode: Number(value) as TeamMode }))} />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="match-mode">对局模式</label>
            <Select id="match-mode" value={String(form.match_mode ?? MatchMode.ROUND_ROBIN)} options={matchModeOptions} onChange={(value) => setForm((current) => ({ ...current, match_mode: Number(value) as MatchMode }))} />
          </div>
          <div className={styles.field}><label className={styles.label} htmlFor="signup-start">报名开始</label><Input id="signup-start" type="datetime-local" value={formatDateTimeLocalInput(form.signup_start)} onChange={(event) => setForm((current) => ({ ...current, signup_start: parseDateTimeLocalInput(event.target.value) }))} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="signup-end">报名结束</label><Input id="signup-end" type="datetime-local" value={formatDateTimeLocalInput(form.signup_end)} onChange={(event) => setForm((current) => ({ ...current, signup_end: parseDateTimeLocalInput(event.target.value) }))} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="start-at">比赛开始</label><Input id="start-at" type="datetime-local" value={formatDateTimeLocalInput(form.start_at)} onChange={(event) => setForm((current) => ({ ...current, start_at: parseDateTimeLocalInput(event.target.value) }))} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="end-at">比赛结束</label><Input id="end-at" type="datetime-local" value={formatDateTimeLocalInput(form.end_at)} onChange={(event) => setForm((current) => ({ ...current, end_at: parseDateTimeLocalInput(event.target.value) }))} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="freeze">封榜时长</label><Input id="freeze" type="number" value={form.freeze_minutes} onChange={(event) => setForm((current) => ({ ...current, freeze_minutes: Number(event.target.value) }))} fullWidth /></div>
        </div>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="rules">竞赛规则</label>
          <Textarea id="rules" className={styles.jsonEditor} value={rulesText} onChange={(event) => setRulesText(event.target.value)} fullWidth />
        </div>
      </section>
    </div>
  )
}

function toRequest(contest: Contest): ContestRequest {
  return {
    name: contest.name,
    mode: contest.mode,
    match_mode: contest.match_mode,
    team_mode: contest.team_mode,
    signup_start: contest.signup_start,
    signup_end: contest.signup_end,
    start_at: contest.start_at,
    end_at: contest.end_at,
    freeze_minutes: contest.freeze_minutes,
    rules: contest.rules,
  }
}

export default TeacherContestConfigPage
