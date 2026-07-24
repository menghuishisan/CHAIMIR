// ContentItemActions 提供题库资源预览、版本、复制、共享和生命周期操作。

import React, { useState } from 'react'
import type { ContentItem, ContentItemSnapshot } from '@chaimir/api-client'
import { ContentType, ContentVisibility } from '@chaimir/api-client'
import { Button, Callout, Input, Modal, Table, useConfirm, ResourceState } from '@chaimir/ui'
import { Archive, Copy, Eye, GitBranch, Share2, Trash2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { contentStatusLabel } from '../../../../../utils'
import styles from '../../content.module.css'

export interface ContentItemActionsProps {
  item: ContentItem
  onChanged: () => void
}

/** ContentItemActions 把资源的次级动作收敛到一个管理弹窗。 */
export function ContentItemActions({ item, onChanged }: ContentItemActionsProps): React.ReactElement {
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [face, setFace] = useState<ContentItemSnapshot>()
  const [versions, setVersions] = useState<ContentItem[]>([])
  const [newVersion, setNewVersion] = useState('')
  const [cloneCode, setCloneCode] = useState('')
  const [cloneVersion, setCloneVersion] = useState('v1')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  /** openManager 同时读取学生可见题面和版本历史。 */
  const openManager = async () => {
    setOpen(true)
    setLoading(true)
    setError('')
    try {
      const [nextFace, nextVersions] = await Promise.all([
        api.content.getItemFace(item.code, item.version),
        api.content.getVersions(item.code),
      ])
      setFace(nextFace)
      setVersions(nextVersions)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '资源详情读取失败，请稍后重试。'))
    } finally {
      setLoading(false)
    }
  }

  /** runAction 执行资源动作并同步列表与弹窗状态。 */
  const runAction = async (action: () => Promise<unknown>, success: string, close = false) => {
    setError('')
    setMessage('')
    try {
      await action()
      setMessage(success)
      onChanged()
      if (close) setOpen(false)
      else if (open) await openManager()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '资源操作失败，请稍后重试。'))
    }
  }

  return (
    <>
      <Button variant="outline" size="sm" icon={<Eye size={14} />} onClick={() => void openManager()}>管理</Button>
      <Button
        variant="ghost"
        size="sm"
        icon={<Share2 size={14} />}
        onClick={() => void runAction(
          () => item.visibility === ContentVisibility.SHARED ? api.content.unshareItem(String(item.id)) : api.content.shareItem(String(item.id)),
          item.visibility === ContentVisibility.SHARED ? '资源已退出共享库。' : '资源已加入共享库。',
        )}
      >{item.visibility === ContentVisibility.SHARED ? '取消共享' : '共享'}</Button>
      <Modal open={open} title={`${item.title} · 资源管理`} size="lg" onClose={() => setOpen(false)}>
        {loading && <ResourceState status="loading" title="正在获取资源详情" />}
        {message && <Callout variant="success" title="操作完成">{message}</Callout>}
        {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
        {face && <Callout variant="info" title="学生可见题面"><strong>{face.title}</strong><br />{contentFaceSummary(face)}</Callout>}
        <Table rows={versions} rowKey={(row) => String(row.id)} ariaLabel="内容版本历史" emptyTitle="暂无版本" emptyDescription="当前资源还没有版本记录。" columns={[
          { key: 'version', title: '版本', dataIndex: 'version', priority: 'primary' },
          { key: 'status', title: '状态', render: (row) => contentStatusLabel(row.status) },
          { key: 'time', title: '更新时间', dataIndex: 'updated_at' },
        ]} />
        <section className={styles.panel}>
          <h2>创建新版本</h2>
          <Input value={newVersion} onChange={(event) => setNewVersion(event.target.value)} placeholder="新版本号" fullWidth />
          <Button icon={<GitBranch size={14} />} disabled={!newVersion.trim()} onClick={() => void runAction(() => api.content.createNewVersion(item.code, { source_version: item.version, new_version: newVersion.trim() }), '新版本草稿已创建。')}>创建版本</Button>
        </section>
        <section className={styles.panel}>
          <h2>复制为新资源</h2>
          <Input value={cloneCode} onChange={(event) => setCloneCode(event.target.value)} placeholder="新资源编号" fullWidth />
          <Input value={cloneVersion} onChange={(event) => setCloneVersion(event.target.value)} placeholder="初始版本" fullWidth />
          <Button icon={<Copy size={14} />} disabled={!cloneCode.trim() || !cloneVersion.trim()} onClick={() => void runAction(() => api.content.cloneItem(item.code, item.version, { new_code: cloneCode.trim(), new_version: cloneVersion.trim() }), '资源副本已创建。')}>创建副本</Button>
        </section>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Archive size={14} />} onClick={async () => {
            const confirmed = await confirm({ title: '停用资源', description: '停用后不能再在新课程、实验或竞赛中引用该版本。', confirmLabel: '确认停用' })
            if (confirmed) await runAction(() => api.content.deprecateItem(String(item.id)), '资源已停用。', true)
          }}>停用资源</Button>
          <Button variant="danger" icon={<Trash2 size={14} />} onClick={async () => {
            const confirmed = await confirm({ title: '删除资源', description: '只有未被课程、实验或竞赛引用的资源才能删除，确定继续吗？', confirmLabel: '确认删除' })
            if (confirmed) await runAction(() => api.content.deleteItem(String(item.id)), '资源已删除。', true)
          }}>删除资源</Button>
        </div>
      </Modal>
    </>
  )
}

/** contentFaceSummary 从已脱敏题面中提取对教师有意义的正文摘要。 */
function contentFaceSummary(face: ContentItemSnapshot): string {
  if (face.type === ContentType.EXPERIMENT_TEMPLATE && 'description' in face.body) return face.body.description
  if ((face.type === ContentType.CONTEST_PROBLEM || face.type === ContentType.THEORY_QUESTION) && 'statement' in face.body) return face.body.statement
  return '题面已按学生视角加载，判题配置和参考答案未展示。'
}
