// 组织架构批量导入(组织架构页的流程页)。
//
// 与账号导入同一模式(FE-7:预览态落服务端,确认后提交),但目标只有组织一种,
// 故不需要选导入对象。一份表里同时建院系、专业与班级,层级关系按表里的列解析。

import { useCallback, useState } from 'react'
import { CircleCheck, Download, FileSpreadsheet, Upload } from 'lucide-react'
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
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Progress,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

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

  const downloadTemplate = useCallback(async () => {
    setFormError(undefined)
    try {
      const result = await api.identity.downloadOrgImportTemplate()
      const url = URL.createObjectURL(result.blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = result.fileName
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '模板下载没有成功,请稍后重试。'))
    }
  }, [])

  const runPreview = useCallback(async () => {
    if (!file) {
      setFormError('请选择要导入的文件')
      return
    }
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
  }, [file])

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

  const errorRows = (preview?.rows ?? []).filter((row) => row.error)

  const errorColumns: TableColumn<{ line: number; error?: string }>[] = [
    { key: 'line', header: '行号', align: 'right', mono: true },
    {
      key: 'error',
      header: '问题',
      render: (row) => <span className="text-sm text-danger">{row.error}</span>,
    },
  ]

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>批量导入组织架构</ModalTitle>
          <ModalDescription>
            一份表里同时建院系、专业与班级。先上传做校验预览,确认无误后提交。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
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
            helper="支持模板导出的 Excel 或 CSV 文件"
          >
            <Input
              id="org-import-file"
              type="file"
              accept=".xlsx,.csv"
              onChange={(event) => {
                setFile(event.target.files?.[0])
                setPreview(undefined)
              }}
            />
          </FormField>

          {uploadProgress !== undefined ? (
            <Progress value={uploadProgress} label="正在上传文件" />
          ) : null}

          {preview ? (
            <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
              <div className="flex flex-wrap items-center gap-2">
                <Badge tone="neutral">共 {preview.total} 行</Badge>
                <Badge tone="success">可导入 {preview.valid} 行</Badge>
                {preview.invalid > 0 ? <Badge tone="danger">有问题 {preview.invalid} 行</Badge> : null}
              </div>

              {preview.invalid > 0 ? (
                <>
                  <Callout tone="warning" title="有问题的行不会被导入">
                    修正这些行后重新上传,或直接提交只导入没有问题的行。
                  </Callout>
                  <Table columns={errorColumns} data={errorRows} rowKey={(row) => String(row.line)} />
                </>
              ) : (
                <Callout tone="success" title="校验通过">
                  全部 {preview.valid} 行都可以导入。
                </Callout>
              )}
            </div>
          ) : (
            <Callout tone="info">
              已存在的院系、专业与班级按名称匹配,不会重复创建。
            </Callout>
          )}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          {preview ? (
            <>
              <Button variant="outline" leftIcon={Upload} onClick={() => setPreview(undefined)}>
                换个文件
              </Button>
              <Button
                variant="seal"
                leftIcon={CircleCheck}
                loading={working}
                disabled={preview.valid === 0}
                onClick={() => void commit()}
              >
                提交导入 {preview.valid} 行
              </Button>
            </>
          ) : (
            <Button
              variant="seal"
              leftIcon={FileSpreadsheet}
              loading={working}
              disabled={!file}
              onClick={() => void runPreview()}
            >
              上传并校验
            </Button>
          )}
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
