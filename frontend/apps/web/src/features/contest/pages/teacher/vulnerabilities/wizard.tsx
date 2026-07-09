// 教师漏洞题导入页：从漏洞源或手工草稿导入后端漏洞题。

import React, { useState } from 'react'
import type { VulnProblem } from '@chaimir/api-client'
import { VulnLevel, VulnRuntimeMode } from '@chaimir/api-client'
import { Button, Input, Select, Textarea } from '@chaimir/ui'
import { UploadCloud } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { parseJsonObject, vulnLevelOptions, vulnRuntimeModeOptions } from '../../../../../utils/index'

const TeacherVulnerabilityWizardPage: React.FC = () => {
  const [sourceId, setSourceId] = useState('')
  const [externalRef, setExternalRef] = useState('')
  const [title, setTitle] = useState('')
  const [level, setLevel] = useState(String(VulnLevel.B))
  const [runtimeMode, setRuntimeMode] = useState(String(VulnRuntimeMode.ISOLATED))
  const [draftBody, setDraftBody] = useState('{}')
  const [problem, setProblem] = useState<VulnProblem | null>(null)
  const [message, setMessage] = useState('')
  const sources = useAsyncResource(() => api.contest.listVulnSources(), [])

  const importProblem = async (fromSource: boolean) => {
    if (!title.trim()) return
    setMessage('')
    try {
      const payload = {
        source_id: sourceId ? Number(sourceId) : undefined,
        external_ref: externalRef.trim() || undefined,
        title: title.trim(),
        level: Number(level) as VulnLevel,
        runtime_mode: Number(runtimeMode) as VulnRuntimeMode,
        draft_body: parseJsonObject(draftBody),
      }
      const result = fromSource ? await api.contest.importVulnSourceProblem(payload) : await api.contest.importVulnProblem(payload)
      setProblem(result)
      setMessage('漏洞题草稿已导入。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法导入漏洞题，请检查草稿内容格式。')
    }
  }

  if (sources.status === 'loading') {
    return <LoadingState title="正在读取漏洞源" description="系统正在同步可导入的漏洞源。" />
  }

  if (sources.status === 'error') {
    return <ErrorState error={sources.error} onRetry={sources.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 漏洞题工坊 / 导入漏洞题</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <UploadCloud className={styles.titleIcon} size={28} />
          导入漏洞题
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <section className={`${styles.panel} ${styles.section}`}>
        <div className={styles.grid}>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="source-id">漏洞源</label>
            <Select
              id="source-id"
              value={sourceId}
              options={[{ label: '不绑定漏洞源', value: '' }, ...(sources.data ?? []).map((source) => ({ label: source.name, value: source.id }))]}
              onChange={setSourceId}
            />
          </div>
          <div className={styles.field}><label className={styles.label} htmlFor="external-ref">外部引用</label><Input id="external-ref" value={externalRef} onChange={(event) => setExternalRef(event.target.value)} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="title">标题</label><Input id="title" value={title} onChange={(event) => setTitle(event.target.value)} fullWidth /></div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="level">等级</label>
            <Select id="level" value={level} options={vulnLevelOptions} onChange={setLevel} />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="runtime-mode">运行方式</label>
            <Select id="runtime-mode" value={runtimeMode} options={vulnRuntimeModeOptions} onChange={setRuntimeMode} />
          </div>
        </div>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="draft-body">草稿内容</label>
          <Textarea id="draft-body" className={styles.jsonEditor} value={draftBody} onChange={(event) => setDraftBody(event.target.value)} fullWidth />
        </div>
        <div className={styles.actions}>
          <Button variant="outline" onClick={() => importProblem(false)}>导入草稿</Button>
          <Button icon={<UploadCloud size={16} />} onClick={() => importProblem(true)}>从漏洞源导入</Button>
        </div>
      </section>

      {problem && (
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>导入结果</h2>
          <p className={styles.muted}>漏洞题 {problem.title} 已生成草稿，编号 {problem.id}。</p>
        </section>
      )}
    </div>
  )
}

export default TeacherVulnerabilityWizardPage
