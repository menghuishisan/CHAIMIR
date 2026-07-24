// CourseLifecycleActions 提供课程邀请码、复制、共享、结课和归档操作。

import React, { useState } from 'react'
import type { Course } from '@chaimir/api-client'
import { Button, Callout, Input, Modal, useConfirm } from '@chaimir/ui'
import { Archive, CheckCircle, Copy, KeyRound, Settings2, Share2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../teaching.module.css'

/** CourseLifecycleActions 把低频课程动作收敛到管理弹窗。 */
export function CourseLifecycleActions({ course, onChanged }: { course: Course; onChanged: () => void }): React.ReactElement {
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)
  const [cloneName, setCloneName] = useState(`${course.name} 副本`)
  const [inviteCode, setInviteCode] = useState(course.invite_code || '')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [pendingAction, setPendingAction] = useState('')

  /** runAction 执行课程生命周期动作并刷新列表。 */
  const runAction = async (key: string, action: () => Promise<Course>, success: string) => {
    if (pendingAction) return
    setPendingAction(key)
    setError('')
    setMessage('')
    try {
      const result = await action()
      setInviteCode(result.invite_code || inviteCode)
      setMessage(success)
      onChanged()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '课程操作失败，请稍后重试。'))
    } finally {
      setPendingAction('')
    }
  }

  return (
    <>
      <Button variant="ghost" size="sm" icon={<Settings2 size={14} />} onClick={() => setOpen(true)}>更多管理</Button>
      <Modal open={open} title={`${course.name} · 课程管理`} size="md" onClose={() => setOpen(false)}>
        {message && <Callout variant="success" title="操作完成">{message}</Callout>}
        {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
        <section className={styles.panel}>
          <h2>课程邀请码</h2>
          <p className={styles.muted}>{inviteCode || '尚未生成邀请码'}</p>
          <Button variant="outline" icon={<KeyRound size={14} />} loading={pendingAction === 'invite'} disabled={Boolean(pendingAction)} onClick={() => void runAction('invite', () => api.teaching.refreshInviteCode(String(course.id)), '邀请码已刷新。')}>刷新邀请码</Button>
        </section>
        <section className={styles.panel}>
          <h2>复制课程</h2>
          <Input value={cloneName} onChange={(event) => setCloneName(event.target.value)} fullWidth />
          <Button variant="outline" icon={<Copy size={14} />} loading={pendingAction === 'clone'} disabled={Boolean(pendingAction) || !cloneName.trim()} onClick={() => void runAction('clone', () => api.teaching.cloneCourse(String(course.id), { name: cloneName.trim() }), '课程副本已创建。')}>创建副本</Button>
        </section>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Share2 size={14} />} loading={pendingAction === 'share'} disabled={Boolean(pendingAction)} onClick={() => void runAction('share', () => api.teaching.shareCourse(String(course.id)), '课程已加入共享库。')}>共享课程</Button>
          <Button variant="outline" icon={<CheckCircle size={14} />} loading={pendingAction === 'end'} disabled={Boolean(pendingAction)} onClick={() => void runAction('end', () => api.teaching.endCourse(String(course.id)), '课程已结课。')}>结束课程</Button>
          <Button variant="danger" icon={<Archive size={14} />} loading={pendingAction === 'archive'} disabled={Boolean(pendingAction)} onClick={async () => {
            const confirmed = await confirm({ title: '归档课程', description: '归档后课程将停止日常教学使用，确定继续吗？', confirmLabel: '确认归档' })
            if (confirmed) await runAction('archive', () => api.teaching.archiveCourse(String(course.id)), '课程已归档。')
          }}>归档课程</Button>
        </div>
      </Modal>
    </>
  )
}
