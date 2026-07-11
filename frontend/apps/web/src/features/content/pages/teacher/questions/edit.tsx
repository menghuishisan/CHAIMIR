// TeacherQuestionEditPage 创建、更新并发布内容中心题目全量版本。

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import type { ContentAttachmentUpload } from '@chaimir/api-client'
import { ContentDifficulty, ContentType, ContentVisibility } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { Download, Edit2, EyeOff, Paperclip, Save, Send } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../content.module.css'
import { contentDifficultyOptions, contentTypeOptions, contentVisibilityOptions, parseJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


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
  const [attachmentFile, setAttachmentFile] = useState<File | null>(null)
  const [attachment, setAttachment] = useState<ContentAttachmentUpload>()
  const [attachmentBusy, setAttachmentBusy] = useState(false)

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
      setError(userFacingErrorMessage(actionError, '题目保存失败，请检查内容后重试。'))
    } finally {
      setSaving(false)
      setPublishing(false)
    }
  }, [body, categoryId, code, difficulty, item, itemId, knowledgePoints, sensitiveFields, tags, title, type, version, visibility])

  /** uploadAttachment 上传当前资源附件并保存对象引用。 */
  const uploadAttachment = async () => {
    if (!itemId || !attachmentFile) return
    setAttachmentBusy(true)
    setError(null)
    try {
      setAttachment(await api.content.uploadAttachment(attachmentFile, itemId))
      setMessage('附件已上传。')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '附件上传失败，请检查文件后重试。'))
    } finally {
      setAttachmentBusy(false)
    }
  }

  /** issueAttachmentGrant 为已上传附件签发短时下载授权。 */
  const issueAttachmentGrant = async () => {
    if (!itemId || !attachment) return
    setAttachmentBusy(true)
    setError(null)
    try {
      const grant = await api.content.issueAttachmentDownloadGrant({ resource_id: itemId, object_ref: attachment.object_ref })
      setMessage(`附件下载授权有效至 ${grant.expires_at}。`)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '附件下载授权创建失败，请稍后重试。'))
    } finally {
      setAttachmentBusy(false)
    }
  }

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
        <h2><Paperclip size={18} />题目附件</h2>
        {!itemId && <Callout variant="info" title="请先保存题目">题目保存后即可上传附件。</Callout>}
        <input type="file" disabled={!itemId} onChange={(event) => setAttachmentFile(event.target.files?.[0] || null)} />
        <div className={styles.actions}>
          <Button variant="outline" icon={<Paperclip size={15} />} disabled={!itemId || !attachmentFile} loading={attachmentBusy} onClick={() => void uploadAttachment()}>上传附件</Button>
          <Button variant="outline" icon={<Download size={15} />} disabled={!attachment} loading={attachmentBusy} onClick={() => void issueAttachmentGrant()}>创建下载授权</Button>
        </div>
        {attachment && <p className={styles.muted}>{attachment.file_name}，{attachment.size} 字节</p>}
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
