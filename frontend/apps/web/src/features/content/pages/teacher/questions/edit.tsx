// TeacherQuestionEditPage 创建、更新并发布内容中心题目全量版本。

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { ContentDifficulty, ContentType, ContentVisibility } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { Edit2, EyeOff, Save, Send } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { contentDifficultyOptions, contentTypeOptions, contentVisibilityOptions, parseJsonObject } from '../../../../../utils/index'


const TeacherQuestionEditPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const itemId = searchParams.get('id') || ''
  const categories = useAsyncResource(() => api.content.listCategories(), [])
  const item = useAsyncResource(async () => {
    if (!itemId) return null
    const items = await api.content.getItems({ page: 1, size: 100 })
    const found = items.list.find((candidate) => String(candidate.id) === itemId)
    return found ? api.content.getItemFull(found.code, found.version) : null
  }, [itemId])
  const [code, setCode] = useState('')
  const [version, setVersion] = useState('v1')
  const [type, setType] = useState(String(ContentType.THEORY_QUESTION))
  const [title, setTitle] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [difficulty, setDifficulty] = useState(String(ContentDifficulty.INTRO))
  const [visibility, setVisibility] = useState(String(ContentVisibility.PRIVATE))
  const [tags, setTags] = useState('')
  const [knowledgePoints, setKnowledgePoints] = useState('')
  const [body, setBody] = useState('{}')
  const [sensitiveFields, setSensitiveFields] = useState('')
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!item.data) return
    setCode(item.data.code)
    setVersion(item.data.version)
    setType(String(item.data.type))
    setTitle(item.data.title)
    setCategoryId(String(item.data.category_id || ''))
    setDifficulty(String(item.data.difficulty))
    setVisibility(String(item.data.visibility))
    setTags(item.data.tags.join(','))
    setKnowledgePoints(item.data.knowledge_points.join(','))
    setBody(JSON.stringify(item.data.body, null, 2))
    setSensitiveFields((item.data.sensitive_fields || []).join(','))
  }, [item.data])

  const categoryOptions = useMemo(() => [
    { value: '', label: '选择分类' },
    ...(categories.data || []).map((category) => ({ value: String(category.id), label: category.name })),
  ], [categories.data])

  /**
   * saveItem 创建或更新内容版本，并可立即发布。
   */
  const saveItem = useCallback(async (publish: boolean) => {
    setSaving(!publish)
    setPublishing(publish)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        type: Number(type) as ContentType,
        title,
        category_id: Number(categoryId),
        difficulty: Number(difficulty) as ContentDifficulty,
        tags: tags.split(',').map((value) => value.trim()).filter(Boolean),
        knowledge_points: knowledgePoints.split(',').map((value) => value.trim()).filter(Boolean),
        visibility: Number(visibility) as ContentVisibility,
        body: parseJsonObject(body),
        sensitive_fields: sensitiveFields.split(',').map((value) => value.trim()).filter(Boolean),
      }
      const saved = itemId
        ? await api.content.updateItem(itemId, payload)
        : await api.content.createItem({ ...payload, code, version })
      if (publish) {
        await api.content.publishItem(String(saved.id))
      }
      setMessage(publish ? '题目已保存并发布。' : '题目草稿已保存。')
      item.reload()
    } catch (actionError) {
      setError((actionError as ApiError).message || (actionError as Error).message || '题目保存失败，请检查内容后重试。')
    } finally {
      setSaving(false)
      setPublishing(false)
    }
  }, [body, categoryId, code, difficulty, item, itemId, knowledgePoints, sensitiveFields, tags, title, type, version, visibility])

  const loading = categories.status === 'loading' || item.status === 'loading'
  const firstError = categories.error || item.error

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Edit2 size={28} />编排题目</h1>
          <p className={styles.subtitle}>学生可见题面和隐藏判分字段统一写入内容中心全量版本。</p>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      {firstError && <ErrorState error={firstError} onRetry={() => { categories.reload(); item.reload() }} />}
      {loading && <LoadingState title="正在获取题目数据" />}

      <section className={styles.panel}>
        <h2>题目元数据</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>内容编号<Input fullWidth value={code} onChange={(event) => setCode(event.target.value)} readOnly={Boolean(itemId)} /></label>
          <label className={styles.field}>版本<Input fullWidth value={version} onChange={(event) => setVersion(event.target.value)} readOnly={Boolean(itemId)} /></label>
          <label className={styles.field}>类型<Select fullWidth value={type} options={contentTypeOptions} onChange={setType} /></label>
          <label className={styles.field}>标题<Input fullWidth value={title} onChange={(event) => setTitle(event.target.value)} /></label>
          <label className={styles.field}>分类<Select fullWidth value={categoryId} options={categoryOptions} onChange={setCategoryId} /></label>
          <label className={styles.field}>难度<Select fullWidth value={difficulty} options={contentDifficultyOptions} onChange={setDifficulty} /></label>
          <label className={styles.field}>可见范围<Select fullWidth value={visibility} options={contentVisibilityOptions} onChange={setVisibility} /></label>
          <label className={styles.field}>标签<Input fullWidth value={tags} onChange={(event) => setTags(event.target.value)} /></label>
          <label className={styles.field}>知识点<Input fullWidth value={knowledgePoints} onChange={(event) => setKnowledgePoints(event.target.value)} /></label>
        </div>
      </section>

      <section className={styles.panel}>
        <h2><EyeOff size={18} />全量内容</h2>
        <label className={styles.fieldFull}>正文 JSON<Textarea value={body} onChange={(event) => setBody(event.target.value)} rows={10} /></label>
        <label className={styles.fieldFull}>学生不可见字段<Input fullWidth value={sensitiveFields} onChange={(event) => setSensitiveFields(event.target.value)} /></label>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Save size={16} />} loading={saving} onClick={() => saveItem(false)}>保存草稿</Button>
          <Button icon={<Send size={16} />} loading={publishing} onClick={() => saveItem(true)}>保存并发布</Button>
        </div>
      </section>
    </div>
  )
}

export default TeacherQuestionEditPage
