// 学生竞赛工作台页：复用共享 Sandbox IDE，并接入环境、提交、对抗和排行榜能力。

import React, { useMemo, useState } from 'react'
import type { ContestProblem, ContestSubmission } from '@chaimir/api-client'
import { BattleRole } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Play, Send, Swords } from 'lucide-react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import { battleRoleOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { SandboxIdeWorkspace } from '../../../../sandbox/components/SandboxIdeWorkspace'
import styles from '../../contest.module.css'

/** StudentContestWorkspacePage 让竞赛业务复用真实沙箱 IDE，不维护第二套工作台结构。 */
const StudentContestWorkspacePage: React.FC = () => {
  const { id } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [problemId, setProblemId] = useState(searchParams.get('problemId') || '')
  const [runtimeCode, setRuntimeCode] = useState('')
  const [runtimeVersion, setRuntimeVersion] = useState('')
  const [toolCodes, setToolCodes] = useState('')
  const [answer, setAnswer] = useState('')
  const [codeStorageKey, setCodeStorageKey] = useState('')
  const [codeHash, setCodeHash] = useState('')
  const [battleRole, setBattleRole] = useState(String(BattleRole.ATTACK))
  const [submission, setSubmission] = useState<ContestSubmission>()
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const sandboxId = searchParams.get('sandboxId') || ''

  const resource = useAsyncResource(
    async () => {
      if (!id) throw new Error('缺少竞赛编号，无法进入赛场。')
      const [problems, ladder, entries] = await Promise.all([
        api.contest.getProblems(id),
        api.contest.getLadder(id, { page: 1, size: 10 }),
        api.contest.listBattleEntries(id),
      ])
      return { problems, ladder: ladder.list, entries }
    },
    [id],
  )
  const selectedProblem = useMemo(
    () => resource.data?.problems.find((item) => item.id === problemId) || resource.data?.problems[0],
    [problemId, resource.data?.problems],
  )

  /** createEnv 使用后端返回的沙箱编号进入共享 IDE，并把恢复信息写入 URL。 */
  const createEnv = async () => {
    if (!id || !selectedProblem || !runtimeCode.trim() || !runtimeVersion.trim()) return
    setMessage('')
    setError('')
    try {
      const env = await api.contest.createEnv(id, selectedProblem.id, {
        runtime_code: runtimeCode.trim(),
        runtime_image_version: runtimeVersion.trim(),
        tool_codes: toolCodes.split(',').map((item) => item.trim()).filter(Boolean),
      })
      setSearchParams({ problemId: selectedProblem.id, sandboxId: env.sandbox_id }, { replace: true })
      setMessage('竞赛环境已创建。')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法创建竞赛环境。'))
    }
  }

  /** submitSolve 提交已保存代码，并立即读取后端生成的判分记录。 */
  const submitSolve = async () => {
    if (!id || !selectedProblem) return
    setMessage('')
    setError('')
    try {
      const created = await api.contest.submitSolve(id, selectedProblem.id, {
        content_ref: { answer: answer.trim() },
        code_storage_key: codeStorageKey || undefined,
        code_hash: codeHash || undefined,
        sandbox_ref: sandboxId || undefined,
      })
      const actual = await api.contest.getSubmission(created.id)
      setSubmission(actual)
      setMessage(actual.passed ? `判分完成，本次获得 ${actual.score} 分。` : '判分已完成，当前提交尚未通过。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法提交答案，请检查补充答案后重试。'))
    }
  }

  /** submitBattleEntry 使用已持久化代码作为参战物，避免要求学生手填存储引用。 */
  const submitBattleEntry = async () => {
    if (!id || !selectedProblem || !codeStorageKey || !codeHash) {
      setError('请先保存代码，再提交参战物。')
      return
    }
    setMessage('')
    setError('')
    try {
      await api.contest.submitBattleEntry(id, {
        problem_id: selectedProblem.id,
        role: Number(battleRole) as BattleRole,
        artifact_ref: codeStorageKey,
        code_hash: codeHash,
      })
      setMessage('参战物已提交。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法提交参战物。'))
    }
  }

  if (resource.status === 'loading') return <ResourceState status="loading" title="正在进入赛场" description="系统正在同步题目、榜单和参战记录。" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  if (!selectedProblem) return <ResourceState status="empty" title="赛场暂无题目" description="赛事发布题目后即可创建环境。" />

  if (!sandboxId) {
    return (
      <div className={styles.page}>
        <div className={styles.breadcrumb}>我的竞赛 / 进入赛场</div>
        <header className={styles.header}><h1 className={styles.title}>准备竞赛环境</h1></header>
        {error && <Callout variant="danger" title="环境创建失败">{error}</Callout>}
        <section className={styles.panel}>
          <div className={styles.grid}>
            <FormField className={styles.field} label="竞赛题目"><Select value={selectedProblem.id} options={(resource.data?.problems || []).map((item) => ({ label: `${item.seq}. ${problemTitle(item)}`, value: item.id }))} onChange={setProblemId} /></FormField>
            <FormField className={styles.field} label="运行环境"><Input value={runtimeCode} onChange={(event) => setRuntimeCode(event.target.value)} placeholder="填写赛事指定的运行环境" fullWidth /></FormField>
            <FormField className={styles.field} label="环境版本"><Input value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} placeholder="填写赛事指定的环境版本" fullWidth /></FormField>
            <FormField className={styles.field} label="辅助工具"><Input value={toolCodes} onChange={(event) => setToolCodes(event.target.value)} placeholder="多个工具用逗号分隔，可不填" fullWidth /></FormField>
          </div>
          <div className={styles.actions}><Button icon={<Play size={15} />} loading={pendingAction === 'environment'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('environment', createEnv)}>创建并进入环境</Button></div>
        </section>
      </div>
    )
  }

  const inspector = (
    <div className={styles.section}>
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>{problemTitle(selectedProblem)}</h2>
        <p className={styles.workspaceMeta}>分值 {selectedProblem.score}</p>
        {selectedProblem.face?.statement && <p className={styles.workspaceStatement}>{selectedProblem.face.statement}</p>}
        <FormField className={styles.darkField} label="补充答案"><Textarea value={answer} onChange={(event) => setAnswer(event.target.value)} placeholder="按题目要求填写答案；仅提交代码时可留空" rows={3} fullWidth /></FormField>
        <Button size="sm" icon={<Send size={15} />} loading={pendingAction === 'solve'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('solve', submitSolve)}>提交判分</Button>
        {submission && <p className={styles.workspaceMeta}>最近提交：{submission.passed ? '已通过' : '未通过'}，{submission.score} 分</p>}
      </section>
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>对抗赛参战物</h2>
        <Select value={battleRole} options={battleRoleOptions} onChange={setBattleRole} />
        <Button size="sm" icon={<Swords size={15} />} loading={pendingAction === 'battle'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('battle', submitBattleEntry)}>提交当前代码</Button>
        <p className={styles.workspaceMeta}>已提交 {resource.data?.entries.length || 0} 个参战版本</p>
      </section>
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>排行榜</h2>
        <ol className={styles.list}>
          {(resource.data?.ladder || []).map((rank) => <li className={styles.listItem} key={rank.team_id}>第 {rank.rank} 名，{rank.team_name}，{rank.score} 分</li>)}
        </ol>
      </section>
    </div>
  )

  return (
    <SandboxIdeWorkspace
      sandboxId={sandboxId}
      title="竞赛赛场"
      subtitle={problemTitle(selectedProblem)}
      inspector={inspector}
      onSaved={(result) => {
        setCodeStorageKey(result.code_storage_key)
        setCodeHash(result.code_hash)
        setMessage('代码已保存，可以提交判分或参战物。')
      }}
    />
  )
}

/** problemTitle 从公开题面提取标题，缺失时使用稳定的题目标识。 */
function problemTitle(problem: ContestProblem): string {
  return problem.title.trim() || `题目 ${problem.seq}`
}

export default StudentContestWorkspacePage
