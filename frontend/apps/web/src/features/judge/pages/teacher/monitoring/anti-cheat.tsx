// TeacherAntiCheatPage 查询竞赛防作弊相似度线索。

import React, { useMemo, useState } from 'react'
import type { CheatRecord, CheatSuspect } from '@chaimir/api-client'
import { CheatAction, CheatType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea, useConfirm, ResourceState, FormField } from '@chaimir/ui'
import { Plus, RefreshCw, ShieldAlert } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../judge.module.css'
import { cheatActionLabel, cheatTypeLabel, formatDateTime, formatStudentReference, formatTeamReference, parseDelimitedList, sourceReferenceLabel } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherAntiCheatPage: React.FC = () => {
  const [contestId, setContestId] = useState('')
  const [problemId, setProblemId] = useState('')
  const [threshold, setThreshold] = useState('0.85')
  const [codeHash, setCodeHash] = useState('')
  const [excludeSourceRef, setExcludeSourceRef] = useState('')
  const [teamId, setTeamId] = useState('')
  const [cheatType, setCheatType] = useState(String(CheatType.SIMILARITY))
  const [cheatAction, setCheatAction] = useState(String(CheatAction.WARN))
  const [reviewNote, setReviewNote] = useState('')
  const [sourceRefs, setSourceRefs] = useState('')
  const [penaltyScore, setPenaltyScore] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const confirm = useConfirm()
  const resource = useAsyncResource(async () => {
    if (!contestId) return { suspects: [] as CheatSuspect[], records: [] as CheatRecord[] }
    const [suspects, records] = await Promise.all([
      problemId ? api.contest.listCheatSuspects(contestId, {
        problem_id: problemId,
        threshold: Number(threshold),
        code_hash: codeHash || undefined,
        exclude_source_ref: excludeSourceRef || undefined,
      }) : Promise.resolve([] as CheatSuspect[]),
      api.contest.listCheatRecords(contestId, { page: 1, size: 50 }),
    ])
    return { suspects, records: records.list }
  }, [codeHash, contestId, excludeSourceRef, problemId, threshold])

  /** createRecord 将人工复核结论写入后端防作弊记录。 */
  const createRecord = async () => {
    if (!contestId || !teamId.trim() || !reviewNote.trim()) return
    setError('')
    setMessage('')
    try {
      const action = Number(cheatAction) as CheatAction
      const confirmed = await confirm({
        title: '确认保存违规处理',
        description: `将对队伍 ${teamId.trim()} 执行“${cheatActionLabel(action)}”。记录保存后会进入审计日志${action === CheatAction.PENALTY ? '并调整排行榜得分' : action === CheatAction.DISQUALIFY ? '并取消参赛资格' : ''}。`,
        confirmLabel: '确认处理',
        confirmVariant: action === CheatAction.WARN ? 'primary' : 'danger',
      })
      if (!confirmed) return
      await api.contest.createCheatRecord(contestId, {
        team_id: teamId,
        type: Number(cheatType) as CheatType,
        action,
        evidence: {
          review_note: reviewNote.trim(),
          source_refs: parseDelimitedList(sourceRefs),
          ...(action === CheatAction.PENALTY ? { penalty_score: Number(penaltyScore) } : {}),
        },
      })
      setMessage('处理记录已保存。')
      setReviewNote('')
      setSourceRefs('')
      setPenaltyScore('')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '处理记录保存失败，请检查输入后重试。'))
    }
  }

  const columns = useMemo<TableColumn<CheatSuspect>[]>(() => [
    { key: 'submitter', title: '提交人', render: (row) => formatStudentReference(row.submitter_id), priority: 'primary' },
    { key: 'source', title: '提交记录', render: (row) => sourceReferenceLabel(row.source_ref) },
    { key: 'hash', title: '代码指纹', render: (row) => row.code_hash ? `${row.code_hash.slice(0, 8)}…${row.code_hash.slice(-6)}` : '未提供' },
    { key: 'score', title: '相似度', render: (row) => <span className={styles.status}>{(row.score * 100).toFixed(1)}%</span> },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><ShieldAlert size={28} />代码相似度告警</h1>
          <p className={styles.subtitle}>按竞赛和题目查询提交内容的相似度线索。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.panel}>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="竞赛编号"><Input fullWidth value={contestId} onChange={(event) => setContestId(event.target.value)} /></FormField>
          <FormField className={styles.field} label="题目编号"><Input fullWidth value={problemId} onChange={(event) => setProblemId(event.target.value)} /></FormField>
          <FormField className={styles.field} label="阈值"><Input fullWidth value={threshold} onChange={(event) => setThreshold(event.target.value)} /></FormField>
          <FormField className={styles.field} label="代码指纹"><Input fullWidth value={codeHash} onChange={(event) => setCodeHash(event.target.value)} /></FormField>
          <FormField className={styles.field} label="排除的提交记录"><Input fullWidth value={excludeSourceRef} onChange={(event) => setExcludeSourceRef(event.target.value)} /></FormField>
        </div>
      </section>
      <section className={styles.panel}>
        <h2 className={styles.sectionTitle}>记录处理结论</h2>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="队伍编号"><Input fullWidth value={teamId} onChange={(event) => setTeamId(event.target.value)} /></FormField>
          <FormField className={styles.field} label="线索类型"><Select fullWidth value={cheatType} onChange={setCheatType} options={[{ value: '1', label: '代码相似' }, { value: '2', label: '行为异常' }, { value: '3', label: '环境异常' }]} /></FormField>
          <FormField className={styles.field} label="处理方式"><Select fullWidth value={cheatAction} onChange={setCheatAction} options={[{ value: '1', label: '警告' }, { value: '2', label: '扣分' }, { value: '3', label: '取消资格' }]} /></FormField>
        </div>
        <FormField className={styles.field} label="复核说明"><Textarea fullWidth rows={4} value={reviewNote} placeholder="说明复核过程、关键事实和处理理由" onChange={(event) => setReviewNote(event.target.value)} /></FormField>
        <FormField className={styles.field} label="相关提交来源"><Textarea fullWidth rows={2} value={sourceRefs} placeholder="每行填写一个相关提交来源，可不填" onChange={(event) => setSourceRefs(event.target.value)} /></FormField>
        {Number(cheatAction) === CheatAction.PENALTY && <FormField className={styles.field} label="扣除分数"><Input fullWidth type="number" min={0.01} step={0.01} value={penaltyScore} onChange={(event) => setPenaltyScore(event.target.value)} /></FormField>}
        <Button icon={<Plus size={15} />} loading={pendingAction === 'record'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('record', createRecord)}>保存处理记录</Button>
      </section>
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在查询相似度线索" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={resource.data?.suspects || []} rowKey={(row) => `${row.source_ref}-${row.submitter_id}`} emptyTitle="暂无线索" emptyDescription="请输入竞赛和题目编号后查询。" ariaLabel="防作弊相似度线索列表" />
        </div>
      )}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table<CheatRecord>
            rows={resource.data?.records || []}
            rowKey="id"
            ariaLabel="防作弊处理记录"
            emptyTitle="暂无处理记录"
            emptyDescription="人工复核后可在这里保存处理结论。"
            columns={[
              { key: 'team', title: '队伍', render: (row) => formatTeamReference(row.team_id), priority: 'primary' },
              { key: 'type', title: '线索类型', render: (row) => cheatTypeLabel(row.type) },
              { key: 'evidence', title: '复核说明', render: (row) => row.evidence.review_note },
              { key: 'action', title: '处理方式', render: (row) => row.action === CheatAction.PENALTY ? `${cheatActionLabel(row.action)} ${row.evidence.penalty_score || 0} 分` : cheatActionLabel(row.action) },
              { key: 'time', title: '记录时间', render: (row) => formatDateTime(row.created_at) },
            ]}
          />
        </div>
      )}
    </div>
  )
}

export default TeacherAntiCheatPage
