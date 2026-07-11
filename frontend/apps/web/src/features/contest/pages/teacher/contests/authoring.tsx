// 教师竞赛题目配置页：读取和写入后端竞赛题目。

import React, { useState } from 'react'
import type { ContestProblem } from '@chaimir/api-client'
import { Button, Input, Table, Textarea } from '@chaimir/ui'
import { ListPlus } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { parseJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherContestAuthoringPage: React.FC = () => {
  const { id } = useParams()
  const [itemCode, setItemCode] = useState('')
  const [itemVersion, setItemVersion] = useState('')
  const [score, setScore] = useState(100)
  const [seq, setSeq] = useState(1)
  const [dynamicScore, setDynamicScore] = useState('{}')
  const [battleConfig, setBattleConfig] = useState('{}')
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法读取题目。')
      return api.contest.getProblems(id)
    },
    [id]
  )

  const addProblem = async () => {
    if (!id || !itemCode.trim() || !itemVersion.trim()) return
    setMessage('')
    try {
      await api.contest.addProblem(id, {
        item_code: itemCode.trim(),
        item_version: itemVersion.trim(),
        score,
        seq,
        dynamic_score: parseJsonObject(dynamicScore),
        battle_config: parseJsonObject(battleConfig),
      })
      setMessage('题目已保存。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法保存题目，请检查配置格式。'))
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取竞赛题目" description="系统正在同步已配置的题目。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 竞赛管理 / 题目配置</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <ListPlus className={styles.titleIcon} size={28} />
          竞赛题目
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>已配置题目</h2>
          <Table<ContestProblem>
            rows={resource.data ?? []}
            rowKey="id"
            ariaLabel="竞赛题目"
            emptyTitle="暂无题目"
            emptyDescription="添加题目后会显示在这里。"
            columns={[
              { key: 'seq', title: '序号', dataIndex: 'seq' },
              { key: 'item', title: '题目编号', dataIndex: 'item_code', priority: 'primary' },
              { key: 'version', title: '版本', dataIndex: 'item_version' },
              { key: 'score', title: '分值', dataIndex: 'score' },
            ]}
          />
        </section>
        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>添加或更新题目</h2>
          <div className={styles.field}><label className={styles.label} htmlFor="item-code">题目编号</label><Input id="item-code" value={itemCode} onChange={(event) => setItemCode(event.target.value)} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="item-version">题目版本</label><Input id="item-version" value={itemVersion} onChange={(event) => setItemVersion(event.target.value)} fullWidth /></div>
          <div className={styles.grid}>
            <div className={styles.field}><label className={styles.label} htmlFor="score">分值</label><Input id="score" type="number" value={score} onChange={(event) => setScore(Number(event.target.value))} fullWidth /></div>
            <div className={styles.field}><label className={styles.label} htmlFor="seq">排序</label><Input id="seq" type="number" value={seq} onChange={(event) => setSeq(Number(event.target.value))} fullWidth /></div>
          </div>
          <div className={styles.field}><label className={styles.label} htmlFor="dynamic-score">动态分规则</label><Textarea id="dynamic-score" className={styles.jsonEditor} value={dynamicScore} onChange={(event) => setDynamicScore(event.target.value)} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="battle-config">对抗配置</label><Textarea id="battle-config" className={styles.jsonEditor} value={battleConfig} onChange={(event) => setBattleConfig(event.target.value)} fullWidth /></div>
          <Button icon={<ListPlus size={16} />} onClick={addProblem}>保存题目</Button>
        </aside>
      </div>
    </div>
  )
}

export default TeacherContestAuthoringPage
