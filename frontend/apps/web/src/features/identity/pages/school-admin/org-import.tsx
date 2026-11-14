// 组织架构批量导入(组织架构页内的右侧大抽屉,与账号导入同一套交互,设计 §7.5)。
//
// 与账号导入同一模式(FE-7:预览态落服务端,确认后提交),但目标只有组织一种,
// 故不需要选导入对象。一份表里同时建院系、专业与班级,层级关系按表里的列解析。

import { useCallback, useState } from 'react'
import { Download } from 'lucide-react'
import type { ImportPreviewResponse } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  FormField,
  Input,
  Progress,
  Steps,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ImportActionFooter } from '../../components/ImportActionFooter'
import { ImportPreviewPanel } from '../../components/ImportPreviewPanel'
import { ImportPreviewRecovery } from '../../components/ImportPreviewRecovery'

/** 导入分三步(设计 §7.5):下载模板 → 上传并预览校验 → 确认导入 */
const IMPORT_STEPS = [
  { key: 'template', label: '下载模板' },
  { key: 'preview', label: '上传并预览' },
  { key: 'commit', label: '确认导入' },
]

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
    <Drawer open onOpenChange={(open) => !open && !working && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>批量导入组织架构</DrawerTitle>
          <DrawerDescription>
            一份表里同时建院系、专业与班级。先上传做校验预览,确认无误后提交。
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-4">
          <Steps steps={IMPORT_STEPS} current={preview ? 2 : file ? 1 : 0} />
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
        </DrawerBody>
        <ImportActionFooter
          preview={preview}
          fileSelected={Boolean(file)}
          working={working}
          onClose={onClose}
          onReset={() => setPreview(undefined)}
          onPreview={() => void runPreview()}
          onCommit={() => void commit()}
        />
      </DrawerContent>
    </Drawer>
  )
}
