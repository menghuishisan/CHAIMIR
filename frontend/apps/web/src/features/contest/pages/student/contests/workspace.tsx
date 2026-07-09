// 学生竞赛工作台页：调用后端题目、环境、提交、对抗参战物和排行榜接口。

import React, { useMemo, useState } from 'react'
import type { ContestProblem, EnvSummary } from '@chaimir/api-client'
import { BattleRole } from '@chaimir/api-client'
import { Button, Input, Select, Table, Textarea } from '@chaimir/ui'
import { Play, RefreshCw, Send, Swords } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { battleRoleOptions, parseJsonObject } from '../../../../../utils/index'

const StudentContestWorkspacePage: React.FC = () => {
  const { id } = useParams()
  const [problemId, setProblemId] = useState('')
  const [runtimeCode, setRuntimeCode] = useState('')
  const [runtimeVersion, setRuntimeVersion] = useState('')
  const [toolCodes, setToolCodes] = useState('')
  const [contentRef, setContentRef] = useState('{}')
  const [codeStorageKey, setCodeStorageKey] = useState('')
  const [codeHash, setCodeHash] = useState('')
  const [sandboxRef, setSandboxRef] = useState('')
  const [artifactRef, setArtifactRef] = useState('')
  const [battleRole, setBattleRole] = useState(String(BattleRole.ATTACK))
  const [env, setEnv] = useState<EnvSummary | null>(null)
  const [message, setMessage] = useState('')

  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法进入赛场。')
      const [problems, ladder, entries] = await Promise.all([
        api.contest.getProblems(id),
        api.contest.getLadder(id, { page: 1, size: 10 }),
        api.contest.listBattleEntries(id).catch(() => []),
      ])
      return { problems, ladder: ladder.list, entries }
    },
    [id]
  )

  const selectedProblem = useMemo(
    () => resource.data?.problems.find((item) => item.id === problemId) ?? resource.data?.problems[0],
    [problemId, resource.data?.problems]
  )

  const createEnv = async () => {
    if (!id || !selectedProblem || !runtimeCode.trim() || !runtimeVersion.trim()) return
    setMessage('')
    try {
      const result = await api.contest.createEnv(id, selectedProblem.id, {
        runtime_code: runtimeCode.trim(),
        runtime_image_version: runtimeVersion.trim(),
        tool_codes: toolCodes.split(',').map((item) => item.trim()).filter(Boolean),
      })
      setEnv(result)
      setSandboxRef(result.sandbox_id)
      setMessage('竞赛环境已创建。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法创建竞赛环境。')
    }
  }

  const submitSolve = async () => {
    if (!id || !selectedProblem) return
    setMessage('')
    try {
      await api.contest.submitSolve(id, selectedProblem.id, {
        content_ref: parseJsonObject(contentRef),
        code_storage_key: codeStorageKey.trim() || undefined,
        code_hash: codeHash.trim() || undefined,
        sandbox_ref: sandboxRef.trim() || undefined,
      })
      setMessage('提交已发送，判分完成后会更新成绩。')
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法提交答案，请检查内容引用格式。')
    }
  }

  const submitBattleEntry = async () => {
    if (!id || !selectedProblem || !artifactRef.trim() || !codeHash.trim()) return
    setMessage('')
    try {
      await api.contest.submitBattleEntry(id, {
        problem_id: Number(selectedProblem.id),
        role: Number(battleRole) as BattleRole,
        artifact_ref: artifactRef.trim(),
        code_hash: codeHash.trim(),
      })
      setMessage('参战物已提交。')
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法提交参战物。')
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在进入赛场" description="系统正在同步题目、榜单和参战记录。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.workspace}>
      <aside className={styles.workspacePanel}>
        <h1 className={styles.workspaceTitle}>竞赛赛场</h1>
        <p className={styles.workspaceMeta}>选择题目后创建环境或提交答案。</p>
        {message && <p className={styles.darkMessage} role="status">{message}</p>}

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>题目</h2>
          <Select
            value={selectedProblem?.id ?? ''}
            options={(resource.data?.problems ?? []).map((item) => ({ label: `${item.seq}. ${item.item_code}`, value: item.id }))}
            onChange={setProblemId}
          />
          {selectedProblem && <p className={styles.workspaceMeta}>分值 {selectedProblem.score}，版本 {selectedProblem.item_version}</p>}
        </div>

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>创建环境</h2>
          <Input className={styles.darkInput} placeholder="运行时编号" value={runtimeCode} onChange={(event) => setRuntimeCode(event.target.value)} fullWidth />
          <Input className={styles.darkInput} placeholder="镜像版本" value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} fullWidth />
          <Input className={styles.darkInput} placeholder="工具编号，多个用逗号分隔" value={toolCodes} onChange={(event) => setToolCodes(event.target.value)} fullWidth />
          <Button size="sm" icon={<Play size={15} />} onClick={createEnv}>创建环境</Button>
          {env && <p className={styles.workspaceMeta}>沙箱 {env.sandbox_id}，状态 {env.status}</p>}
        </div>

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>提交答案</h2>
          <Textarea className={styles.darkInput} rows={4} value={contentRef} onChange={(event) => setContentRef(event.target.value)} fullWidth />
          <Input className={styles.darkInput} placeholder="代码存储引用" value={codeStorageKey} onChange={(event) => setCodeStorageKey(event.target.value)} fullWidth />
          <Input className={styles.darkInput} placeholder="代码哈希" value={codeHash} onChange={(event) => setCodeHash(event.target.value)} fullWidth />
          <Input className={styles.darkInput} placeholder="沙箱引用" value={sandboxRef} onChange={(event) => setSandboxRef(event.target.value)} fullWidth />
          <Button size="sm" icon={<Send size={15} />} onClick={submitSolve}>提交判分</Button>
        </div>

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>对抗赛参战物</h2>
          <Select
            value={battleRole}
            options={battleRoleOptions}
            onChange={setBattleRole}
          />
          <Input className={styles.darkInput} placeholder="参战物引用" value={artifactRef} onChange={(event) => setArtifactRef(event.target.value)} fullWidth />
          <Button size="sm" icon={<Swords size={15} />} onClick={submitBattleEntry}>提交参战物</Button>
        </div>
      </aside>

      <main className={styles.stage}>
        <div className={styles.header}>
          <h2 className={styles.workspaceTitle}>赛场数据</h2>
          <Button variant="on-dark" size="sm" icon={<RefreshCw size={15} />} onClick={resource.reload}>刷新</Button>
        </div>
        <div className={styles.split}>
          <section className={styles.darkCard}>
            <h3 className={styles.workspaceTitle}>题面</h3>
            <Table<ContestProblem>
              rows={resource.data?.problems ?? []}
              rowKey="id"
              ariaLabel="竞赛题目"
              columns={[
                { key: 'seq', title: '序号', dataIndex: 'seq' },
                { key: 'item', title: '题目', dataIndex: 'item_code', priority: 'primary' },
                { key: 'score', title: '分值', dataIndex: 'score' },
              ]}
            />
          </section>
          <aside className={styles.darkCard}>
            <h3 className={styles.workspaceTitle}>排行榜</h3>
            <ul className={styles.list}>
              {(resource.data?.ladder ?? []).map((rank) => (
                <li className={styles.listItem} key={rank.team_id}>第 {rank.rank} 名，{rank.score} 分</li>
              ))}
            </ul>
          </aside>
        </div>
      </main>
    </div>
  )
}

export default StudentContestWorkspacePage
