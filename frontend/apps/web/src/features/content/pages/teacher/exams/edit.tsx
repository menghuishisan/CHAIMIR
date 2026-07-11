// TeacherExamsEditPage 创建试卷组卷规则，并查看已有试卷详情。

import React, { useCallback, useEffect, useState } from 'react'
import { PaperMode } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { FilePlus, RefreshCw, Save } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { paperModeOptions, parseJsonArray } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherExamsEditPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const paperId = searchParams.get('id') || ''
  const paper = useAsyncResource(() => (paperId ? api.content.getPaper(paperId) : Promise.resolve(null)), [paperId])
  const [name, setName] = useState('')
  const [mode, setMode] = useState(String(PaperMode.RANDOM))
  const [count, setCount] = useState('20')
  const [defaultScore, setDefaultScore] = useState('5')
  const [knowledgePoints, setKnowledgePoints] = useState('')
  const [manualItems, setManualItems] = useState('[]')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!paper.data) return
    setName(`${paper.data.paper.name} 副本`)
    setMode(String(paper.data.paper.gen_mode))
    setCount(String(paper.data.paper.gen_criteria.count || 20))
    setDefaultScore(String(paper.data.paper.gen_criteria.default_score || 5))
    setKnowledgePoints((paper.data.paper.gen_criteria.knowledge_points || []).join(','))
    setManualItems(JSON.stringify(paper.data.items.map((item) => ({ code: item.code, version: item.version, score: item.score })), null, 2))
  }, [paper.data])

  /**
   * createPaper 创建新的试卷规则；已有试卷使用重新组卷接口更新题目抽取结果。
   */
  const createPaper = useCallback(async () => {
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      await api.content.createPaper({
        name,
        gen_mode: Number(mode) as PaperMode,
        gen_criteria: {
          count: Number(count),
          default_score: Number(defaultScore),
          knowledge_points: knowledgePoints.split(',').map((value) => value.trim()).filter(Boolean),
        },
        items: parseJsonArray<{ code: string; version: string; score: number }>(manualItems, '手动题目列表必须是数组。', []),
      })
      setMessage('试卷规则已创建。')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '试卷规则保存失败，请检查内容后重试。'))
    } finally {
      setSaving(false)
    }
  }, [count, defaultScore, knowledgePoints, manualItems, mode, name])

  const regeneratePaper = useCallback(async () => {
    if (!paperId) return
    setError(null)
    setMessage(null)
    try {
      await api.content.regeneratePaper(paperId)
      setMessage('试卷已重新组卷。')
      paper.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '重新组卷失败，请稍后重试。'))
    }
  }, [paper, paperId])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><FilePlus size={28} />试卷组卷规则</h1>
          <p className={styles.subtitle}>创建新试卷规则；已有试卷可读取详情并重新组卷。</p>
        </div>
        {paperId && <Button variant="outline" icon={<RefreshCw size={16} />} onClick={regeneratePaper}>重新组卷</Button>}
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      {paper.status === 'error' && <ErrorState error={paper.error} onRetry={paper.reload} />}
      {paper.status === 'loading' && <LoadingState title="正在获取试卷" />}

      <section className={styles.panel}>
        <h2>规则配置</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>试卷名称<Input fullWidth value={name} onChange={(event) => setName(event.target.value)} /></label>
          <label className={styles.field}>组卷方式<Select fullWidth value={mode} options={paperModeOptions} onChange={setMode} /></label>
          <label className={styles.field}>抽题数量<Input fullWidth value={count} onChange={(event) => setCount(event.target.value)} /></label>
          <label className={styles.field}>默认分值<Input fullWidth value={defaultScore} onChange={(event) => setDefaultScore(event.target.value)} /></label>
          <label className={styles.fieldFull}>知识点<Input fullWidth value={knowledgePoints} onChange={(event) => setKnowledgePoints(event.target.value)} /></label>
          <label className={styles.fieldFull}>手动题目列表 JSON<Textarea value={manualItems} onChange={(event) => setManualItems(event.target.value)} rows={8} /></label>
        </div>
        <Button icon={<Save size={16} />} loading={saving} onClick={createPaper}>创建试卷规则</Button>
      </section>

      {paper.data && (
        <section className={styles.panel}>
          <h2>当前试卷题目</h2>
          {paper.data.items.map((item) => (
            <div className={styles.card} key={`${item.code}-${item.version}`}>
              <strong>{item.item.title}</strong>
              <span className={styles.muted}>{item.code} · {item.version} · {item.score} 分</span>
            </div>
          ))}
        </section>
      )}
    </div>
  )
}

export default TeacherExamsEditPage
