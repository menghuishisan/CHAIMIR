// TeacherAntiCheatPage 查询竞赛防作弊相似度线索。

import React, { useMemo, useState } from 'react'
import type { CheatSuspect } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Input, Table } from '@chaimir/ui'
import { RefreshCw, ShieldAlert } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../judge.module.css'

const TeacherAntiCheatPage: React.FC = () => {
  const [contestId, setContestId] = useState('')
  const [problemId, setProblemId] = useState('')
  const [threshold, setThreshold] = useState('0.85')
  const [codeHash, setCodeHash] = useState('')
  const [excludeSourceRef, setExcludeSourceRef] = useState('')
  const resource = useAsyncResource(() => {
    if (!contestId || !problemId) {
      return Promise.resolve([] as CheatSuspect[])
    }
    return api.contest.listCheatSuspects(contestId, {
      problem_id: Number(problemId),
      threshold: Number(threshold),
      code_hash: codeHash || undefined,
      exclude_source_ref: excludeSourceRef || undefined,
    })
  }, [codeHash, contestId, excludeSourceRef, problemId, threshold])

  const columns = useMemo<TableColumn<CheatSuspect>[]>(() => [
    { key: 'submitter', title: '提交人', dataIndex: 'submitter_id', priority: 'primary' },
    { key: 'source', title: '来源引用', dataIndex: 'source_ref' },
    { key: 'hash', title: '代码哈希', render: (row) => row.code_hash || '未提供' },
    { key: 'score', title: '相似度', render: (row) => <span className={styles.status}>{(row.score * 100).toFixed(1)}%</span> },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><ShieldAlert size={28} />代码相似度告警</h1>
          <p className={styles.subtitle}>按竞赛和题目查询后端识别的相似度线索。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      <section className={styles.panel}>
        <div className={styles.formGrid}>
          <label className={styles.field}>竞赛编号<Input fullWidth value={contestId} onChange={(event) => setContestId(event.target.value)} /></label>
          <label className={styles.field}>题目编号<Input fullWidth value={problemId} onChange={(event) => setProblemId(event.target.value)} /></label>
          <label className={styles.field}>阈值<Input fullWidth value={threshold} onChange={(event) => setThreshold(event.target.value)} /></label>
          <label className={styles.field}>代码哈希<Input fullWidth value={codeHash} onChange={(event) => setCodeHash(event.target.value)} /></label>
          <label className={styles.field}>排除来源<Input fullWidth value={excludeSourceRef} onChange={(event) => setExcludeSourceRef(event.target.value)} /></label>
        </div>
      </section>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在查询相似度线索" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={resource.data || []} rowKey={(row) => `${row.source_ref}-${row.submitter_id}`} emptyTitle="暂无线索" emptyDescription="请输入竞赛和题目编号后查询。" ariaLabel="防作弊相似度线索列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherAntiCheatPage
