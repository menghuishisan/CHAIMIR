// 漏洞题服务端草稿写回与本机未同步输入恢复；并发冲突必须显式处理。
import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, VulnProblemStatus, type VulnProblem } from '@chaimir/api-client'
import { api } from '../../app/api'
import { useSession } from '../../components/RoleGuard'
import { errorDiagnostics, userFacingErrorMessage } from '../../utils/userFacingError'
import { vulnProblemFields, vulnProblemRequest, type VulnProblemFields } from './vulnProblemDraft'
import { reconcileRuntimeNames } from '../sandbox/composition'

interface Recovery {
  updatedAt: string
  fields: VulnProblemFields
}

// 连接检查证明不进入保存请求，也不能使草稿内容变脏。
function inputKey(fields: VulnProblemFields): string {
  return JSON.stringify({ ...fields, declaration: { ...fields.declaration, connectionCheck: undefined } })
}

// 本机内容也视作不可信输入；只恢复表单能够完整读取的形状。
function isFields(value: unknown): value is VulnProblemFields {
  if (!value || typeof value !== 'object') return false
  const row = value as Record<string, unknown>
  const strings = ['title', 'judgerCode', 'maxScore', 'sourceId', 'externalRef', 'level', 'runtimeMode', 'description', 'contractSource', 'forkBlock']
  const list = (name: string, keys: string[]) => Array.isArray(row[name]) && row[name].every((item: unknown) =>
    !!item && typeof item === 'object' && keys.every((key) => typeof (item as Record<string, unknown>)[key] === 'string'))
  const declaration = row.declaration as Record<string, unknown> | undefined
  return strings.every((key) => typeof row[key] === 'string') &&
    list('initSteps', ['runtimeInstance', 'op', 'payload', 'target']) && list('positiveSteps', ['runtimeInstance', 'op', 'payload', 'target']) &&
    list('assertions', ['runtimeInstance', 'label', 'target', 'field', 'op', 'value', 'expectedLabel', 'hint']) &&
    !!declaration && Array.isArray(declaration.runtimes) && typeof declaration.workspaceRuntimeInstance === 'string' &&
    Array.isArray(declaration.toolCodes) && declaration.toolCodes.every((code) => typeof code === 'string') &&
    Array.isArray(declaration.infraCodes) && declaration.infraCodes.every((code) => typeof code === 'string') && Array.isArray(declaration.links) &&
    declaration.links.every((link: unknown) => !!link && typeof link === 'object' &&
      ['source_component', 'source_endpoint', 'target_component', 'target_endpoint', 'protocol', 'access_scope', 'config_binding'].every((key) => typeof (link as Record<string, unknown>)[key] === 'string') && typeof (link as Record<string, unknown>).required_at_start === 'boolean') &&
    declaration.runtimes.every((runtime: unknown) => !!runtime && typeof runtime === 'object' &&
      ['clientId', 'instanceCode', 'runtimeCode', 'imageVersion'].every((key) => typeof (runtime as Record<string, unknown>)[key] === 'string') &&
      ['originalInstanceCode', 'pendingInstanceCode'].every((key) => (runtime as Record<string, unknown>)[key] === undefined || typeof (runtime as Record<string, unknown>)[key] === 'string'))
}

function readRecovery(key: string): { recovery?: Recovery; error?: string } {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return {}
    const value = JSON.parse(raw) as Partial<Recovery>
    if (!value || typeof value.updatedAt !== 'string' || !isFields(value.fields)) throw new Error('invalid recovery')
    return { recovery: value as Recovery }
  } catch (error) {
    console.error('漏洞题本机恢复记录无法读取', errorDiagnostics(error))
    return { error: '本机恢复记录无法读取，请确认内容已保存后再离开。' }
  }
}

/** 串行保存最新编辑；本机内容始终依附已经读取的服务端版本。 */
export function useVulnProblemDraft(problem: VulnProblem) {
  const { me } = useSession()
  const key = `chaimir:vuln-draft:${me.account.tenant_id}:${me.account.id}:${problem.id}`
  const [initial] = useState(() => {
    const local = readRecovery(key)
    return { ...local, fields: local.recovery?.fields ?? vulnProblemFields(problem) }
  })
  const [fields, setFields] = useState(initial.fields)
  const [dirty, setDirty] = useState(Boolean(initial.recovery))
  const [status, setStatus] = useState<'saved' | 'pending' | 'saving' | 'error' | 'conflict'>(
    initial.recovery ? initial.recovery.updatedAt === problem.updated_at ? 'pending' : 'conflict' : 'saved'
  )
  const [error, setError] = useState<string>()
  const [cacheError, setCacheError] = useState(initial.error)
  const current = useRef({ problem, fields: initial.fields, dirty: Boolean(initial.recovery), blocked: Boolean(initial.recovery && initial.recovery.updatedAt !== problem.updated_at) })
  const saving = useRef<Promise<boolean> | null>(null)
  const mounted = useRef(true)

  const cache = useCallback((recovery: Recovery | null) => {
    try {
      if (recovery) localStorage.setItem(key, JSON.stringify(recovery))
      else localStorage.removeItem(key)
      if (mounted.current) setCacheError(undefined)
    } catch (cause) {
      console.error('漏洞题本机恢复记录无法写入', errorDiagnostics(cause))
      if (mounted.current) setCacheError('本机恢复记录无法保存，请先保存到服务器再离开。')
    }
  }, [key])

  const save = useCallback((): Promise<boolean> => {
    if (saving.current) return saving.current
    if (current.current.blocked) return Promise.resolve(false)
    const operation = (async () => {
      while (current.current.dirty) {
        const snapshot = current.current.fields
        let request
        try { request = vulnProblemRequest(snapshot, current.current.problem) }
        catch {
          if (mounted.current) { setStatus('error'); setError('请检查标题、案例编号、实例名称、数字和参数格式；当前输入仍保留在页面中。') }
          return false
        }
        if (mounted.current) { setStatus('saving'); setError(undefined) }
        try {
          const result = await api.contest.updateVulnProblem(problem.id, { ...request, expected_updated_at: current.current.problem.updated_at })
          const pending = current.current.fields
          current.current.dirty = inputKey(snapshot) !== inputKey(pending)
          // 后续保存以这次写回的别名为基准，未提交的新别名仍保留在输入中。
          const savedNames = new Map(snapshot.declaration.runtimes.map((runtime) => [runtime.clientId, runtime.instanceCode]))
          current.current.fields = { ...pending, declaration: { ...pending.declaration, runtimes: pending.declaration.runtimes.map((runtime) => ({
            ...runtime, originalInstanceCode: savedNames.get(runtime.clientId) ?? runtime.originalInstanceCode,
          })) } }
          current.current.problem = result
          if (mounted.current) {
            cache(current.current.dirty ? { updatedAt: result.updated_at, fields: current.current.fields } : null)
            setFields(current.current.fields); setDirty(current.current.dirty); setStatus(current.current.dirty ? 'pending' : 'saved')
          }
        } catch (cause) {
          current.current.blocked = cause instanceof ApiError && cause.code === '11006'
          if (mounted.current) { setStatus(current.current.blocked ? 'conflict' : 'error'); setError(userFacingErrorMessage(cause, '草稿保存没有成功，请重试。')) }
          return false
        }
        if (!mounted.current) return !current.current.dirty
      }
      return true
    })()
    const tracked = operation.finally(() => { if (saving.current === tracked) saving.current = null })
    saving.current = tracked
    return tracked
  }, [cache, problem.id])

  const update = useCallback((patch: Partial<VulnProblemFields>) => {
    if (current.current.blocked) return
    const previous = current.current.fields
    const next = { ...previous, ...patch }
    if (patch.declaration) {
      // 组合控件已迁移工作区和连接，这里只迁移漏洞题自己的引用。
      const { runtimes, renames } = reconcileRuntimeNames(previous.declaration.runtimes, patch.declaration.runtimes)
      next.declaration = { ...patch.declaration, runtimes }
      if (renames.size > 0) {
        const rename = <T extends { runtimeInstance: string }>(item: T): T => ({ ...item, runtimeInstance: renames.get(item.runtimeInstance) ?? item.runtimeInstance })
        next.initSteps = next.initSteps.map(rename)
        next.positiveSteps = next.positiveSteps.map(rename)
        next.assertions = next.assertions.map(rename)
      }
    }
    current.current.fields = next
    setFields(next)
    if (inputKey(previous) === inputKey(next)) return
    current.current.dirty = true
    setDirty(true); setStatus('pending'); setError(undefined)
    cache({ updatedAt: current.current.problem.updated_at, fields: next })
  }, [cache])

  // 编辑事件更新恢复记录；Effect 只负责向服务器同步，不派生表单状态。
  useEffect(() => {
    if (!dirty || status !== 'pending') return
    const timer = window.setTimeout(() => void save(), 600)
    return () => window.clearTimeout(timer)
  }, [dirty, fields, save, status])

  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])

  const resolveConflict = useCallback(async (keepInput: boolean) => {
    try {
      const latest = await api.contest.getVulnProblem(problem.id)
      if (!mounted.current) return
      if (latest.status !== VulnProblemStatus.DRAFT) { setError('这道题已结束编辑，不能继续修改。'); return }
      const next = keepInput ? current.current.fields : vulnProblemFields(latest)
      current.current = { problem: latest, fields: next, dirty: keepInput, blocked: false }
      setFields(next); setDirty(keepInput); setStatus(keepInput ? 'pending' : 'saved'); setError(undefined)
      cache(keepInput ? { updatedAt: latest.updated_at, fields: next } : null)
    } catch (cause) { setError(userFacingErrorMessage(cause, '暂时无法读取最新草稿，请稍后重试。')) }
  }, [cache, problem.id])

  return { fields, update, save, dirty, status, error, cacheError, resolveConflict }
}
