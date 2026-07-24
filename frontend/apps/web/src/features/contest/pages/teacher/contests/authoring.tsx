// 教师竞赛题目配置页：读取和写入后端竞赛题目。

import React, { useState } from 'react'
import type { ContestProblem } from '@chaimir/api-client'
import { BattleRule, ContestMode } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Switch, Table, ResourceState } from '@chaimir/ui'
import { ListPlus } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { battleRuleLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherContestAuthoringPage: React.FC = () => {
  const { id } = useParams()
  const [itemCode, setItemCode] = useState('')
  const [itemVersion, setItemVersion] = useState('')
  const [score, setScore] = useState(100)
  const [seq, setSeq] = useState(1)
  const [dynamicEnabled, setDynamicEnabled] = useState(false)
  const [minScore, setMinScore] = useState(50)
  const [decayPerSolve, setDecayPerSolve] = useState(1)
  const [battleRule, setBattleRule] = useState(String(BattleRule.ATTACK_DEFENSE))
  const [runtimeCode, setRuntimeCode] = useState('')
  const [runtimeVersion, setRuntimeVersion] = useState('')
  const [toolCodes, setToolCodes] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法读取题目。')
      const [contest, problems] = await Promise.all([api.contest.getContest(id), api.contest.getProblems(id)])
      return { contest, problems }
    },
    [id]
  )

  const addProblem = async () => {
    if (!id || !itemCode.trim() || !itemVersion.trim()) return
    setMessage('')
    setError('')
    try {
      const isBattle = resource.data?.contest.mode === ContestMode.BATTLE
      await api.contest.addProblem(id, {
        item_code: itemCode.trim(),
        item_version: itemVersion.trim(),
        score,
        seq,
        dynamic_score: dynamicEnabled ? { min_score: minScore, decay_per_solve: decayPerSolve } : undefined,
        battle_config: isBattle ? {
          runtime_code: runtimeCode.trim(),
          runtime_image_version: runtimeVersion.trim(),
          tool_codes: toolCodes.split(/[,，]/).map((item) => item.trim()).filter(Boolean),
        } : undefined,
        battle_rule: isBattle ? Number(battleRule) as BattleRule : undefined,
      })
      setMessage('题目已保存。')
      resource.reload()
    } catch (error) {
      setError(userFacingErrorMessage(error, '暂时无法保存题目，请检查必填信息。'))
    }
  }

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取竞赛题目" description="系统正在同步已配置的题目。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
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
      {message && <Callout variant="success" title="题目已保存">{message}</Callout>}
      {error && <Callout variant="danger" title="题目未保存">{error}</Callout>}

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>已配置题目</h2>
          <Table<ContestProblem>
            rows={resource.data?.problems ?? []}
            rowKey="id"
            ariaLabel="竞赛题目"
            emptyTitle="暂无题目"
            emptyDescription="添加题目后会显示在这里。"
            columns={[
              { key: 'seq', title: '序号', dataIndex: 'seq' },
              { key: 'title', title: '题目', render: (row) => row.title || row.item_code, priority: 'primary' },
              { key: 'item', title: '内容编号', dataIndex: 'item_code' },
              { key: 'version', title: '版本', dataIndex: 'item_version' },
              { key: 'score', title: '分值', dataIndex: 'score' },
              { key: 'dynamic', title: '动态分', render: (row) => row.dynamic_score ? `每解出一队减少 ${row.dynamic_score.decay_per_solve} 分，最低 ${row.dynamic_score.min_score} 分` : '关闭' },
              { key: 'battle', title: '对局规则', render: (row) => row.battle_rule ? battleRuleLabel(row.battle_rule) : '不适用' },
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
          <Switch checked={dynamicEnabled} label="按解出队伍数逐步降低分值" onChange={(event) => setDynamicEnabled(event.target.checked)} />
          {dynamicEnabled && <div className={styles.grid}>
            <div className={styles.field}><label className={styles.label} htmlFor="min-score">最低分值</label><Input id="min-score" type="number" min={1} max={score} value={minScore} onChange={(event) => setMinScore(Number(event.target.value))} fullWidth /></div>
            <div className={styles.field}><label className={styles.label} htmlFor="decay-score">每队衰减</label><Input id="decay-score" type="number" min={1} value={decayPerSolve} onChange={(event) => setDecayPerSolve(Number(event.target.value))} fullWidth /></div>
          </div>}
          {resource.data?.contest.mode === ContestMode.BATTLE && <>
            <div className={styles.field}><label className={styles.label} htmlFor="battle-rule">对局规则</label><Select id="battle-rule" value={battleRule} options={[{ value: String(BattleRule.ATTACK_DEFENSE), label: '攻防对局' }, { value: String(BattleRule.GAME), label: '策略博弈' }]} onChange={setBattleRule} /></div>
            <div className={styles.field}><label className={styles.label} htmlFor="runtime-code">运行环境</label><Input id="runtime-code" value={runtimeCode} onChange={(event) => setRuntimeCode(event.target.value)} placeholder="填写平台已登记的运行环境" fullWidth /></div>
            <div className={styles.field}><label className={styles.label} htmlFor="runtime-version">环境版本</label><Input id="runtime-version" value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} placeholder="填写固定的镜像版本" fullWidth /></div>
            <div className={styles.field}><label className={styles.label} htmlFor="tool-codes">对局工具</label><Input id="tool-codes" value={toolCodes} onChange={(event) => setToolCodes(event.target.value)} placeholder="多个工具用逗号分隔" fullWidth /></div>
          </>}
          <Button icon={<ListPlus size={16} />} loading={pendingAction === 'problem'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('problem', addProblem)}>保存题目</Button>
        </aside>
      </div>
    </div>
  )
}

export default TeacherContestAuthoringPage
