// 组织架构批量导入(组织架构页的流程页)。
//
// 与账号导入同一模式(FE-7:预览态落服务端,确认后提交),但目标只有组织一种,
// 故不需要选导入对象。一份表里同时建院系、专业与班级,层级关系按表里的列解析。

import { useCallback, useState } from 'react'
import { Download } from 'lucide-react'
import type { ImportPreviewResponse } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalHeader,
  ModalTitle,
  Progress,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ImportActionFooter } from '../../components/ImportActionFooter'
import { ImportPreviewPanel } from '../../components/ImportPreviewPanel'
import { ImportPreviewRecovery } from '../../components/ImportPreviewRecovery'

export interface OrgImportModalProps {
  onClose: () => void
  onCommitted: () => void
}

/**
 * OrgImportModal 承载组织架构导入预览与提交。
 */
export function OrgImportModal({ onClose, onCommitted }: OrgImportModalProps) {
  const [file, setFile] = useState<File>()
  const [uploadProgress, setUploadProgress] = useState<number>()
  const [preview, setPreview] = useState<ImportPreviewResponse>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(() => ({
    file: file ? null : '请选择要导入的文件',
  }))

  const downloadTemplate = useCallback(async () => {
    setFormError(undefined)
    try {
      const result = await api.identity.downloadOrgImportTemplate()
      downloadAttachment(result)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '模板下载没有成功,请稍后重试。'))
    }
  }, [])

  const runPreview = useCallback(async () => {
    if (working || !validate() || !file) return
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.identity.previewOrgImport(file, setUploadProgress)
      setPreview(result)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '文件解析没有成功,请确认使用了最新模板。'))
    } finally {
      setWorking(false)
      setUploadProgress(undefined)
    }
  }, [file, validate, working])

  const commit = useCallback(async () => {
    if (!preview) return
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.identity.commitOrgImport({ preview_id: preview.preview_id })
      toast.success(`已导入 ${result.batch.success} 行组织结构`)
      onCommitted()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '导入提交没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [onCommitted, preview])

  return (
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>批量导入组织架构</ModalTitle>
          <ModalDescription>
            一份表里同时建院系、专业与班级。先上传做校验预览,确认无误后提交。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <div className="grid gap-2 sm:grid-cols-3" aria-label="组织导入步骤">
            {[
              ['1', '下载模板'],
              ['2', '上传并预览'],
              ['3', '确认提交'],
            ].map(([step, label]) => (
              <div key={step} className="flex items-center gap-2 rounded-lg border border-border bg-surface p-3">
                <Badge tone={preview ? 'success' : 'neutral'}>{step}</Badge>
                <span className="text-sm text-ink">{label}</span>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" leftIcon={Download} onClick={() => void downloadTemplate()}>
              下载导入模板
            </Button>
            <span className="text-sm text-ink-sub">按模板列填写,层级关系由列决定。</span>
          </div>

          <FormField
            label="导入文件"
            htmlFor="org-import-file"
            required
            error={errors.file}
            helper="支持模板导出的 Excel 或 CSV 文件"
          >
            <Input
              id="org-import-file"
              disabled={working}
              type="file"
              accept=".xlsx,.csv"
              invalid={Boolean(errors.file)}
              onBlur={() => validate(['file'])}
              onChange={(event) => {
                const selected = event.target.files?.[0]
                if (!selected) return
                setFile(selected)
                setPreview(undefined)
              }}
            />
          </FormField>

          {uploadProgress !== undefined ? (
            <Progress value={uploadProgress} label="正在上传文件" />
          ) : null}

          {!preview ? <ImportPreviewRecovery target="org" disabled={working} onRestore={setPreview} /> : null}

          <ImportPreviewPanel
            preview={preview}
            emptyDescription="已存在的院系、专业与班级按名称匹配,不会重复创建。"
          />

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ImportActionFooter
          preview={preview}
          fileSelected={Boolean(file)}
          working={working}
          onClose={onClose}
          onReset={() => setPreview(undefined)}
          onPreview={() => void runPreview()}
          onCommit={() => void commit()}
        />
      </ModalContent>
    </Modal>
  )
}
