// GradeDecisionDialog 收集成绩审核和申诉处理所需的真实意见。

import React from 'react'
import { Button, Modal, Textarea, FormField } from '@chaimir/ui'
import styles from '../pages/grade.module.css'

export interface GradeDecisionDialogProps {
  open: boolean
  title: string
  description: string
  confirmLabel: string
  value: string
  loading?: boolean
  danger?: boolean
  onChange: (value: string) => void
  onClose: () => void
  onConfirm: () => void
}

/** GradeDecisionDialog 要求审核人填写可被学生或教师理解的处理说明。 */
export function GradeDecisionDialog({ open, title, description, confirmLabel, value, loading, danger, onChange, onClose, onConfirm }: GradeDecisionDialogProps): React.ReactElement {
  return <Modal
    open={open}
    title={title}
    size="sm"
    closeOnOverlayClick={false}
    onClose={onClose}
    footer={<><Button variant="ghost" onClick={onClose}>取消</Button><Button variant={danger ? 'danger' : 'primary'} loading={loading} disabled={!value.trim()} onClick={onConfirm}>{confirmLabel}</Button></>}
  >
    <div className={styles.decisionDialog}>
      <p>{description}</p>
      <FormField className={styles.field} label="处理说明"><Textarea rows={5} value={value} placeholder="说明核验结果、处理理由和下一步建议" onChange={(event) => onChange(event.target.value)} /></FormField>
    </div>
  </Modal>
}
