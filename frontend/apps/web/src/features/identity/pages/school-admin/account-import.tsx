// 账号批量导入(账号管理页的流程页)。
//
// FE-7:导入分两步且中间态在服务端 —— 上传得到预览批次(preview_id),
// 确认后再提交。刷新页面不会丢已上传的文件,重新打开可继续用同一预览提交。
//
// 预览会逐行校验:有错误行时明确列出行号与原因,让管理员改表再来,
// 而不是让部分成功部分失败(后端 commit 以预览为准,不重新解析文件)。

import { useCallback, useState } from 'react'
import { CircleCheck, FileSpreadsheet, Upload } from 'lucide-react'
import type { ImportActivationCode, ImportPreviewResponse } from '@chaimir/api-client'
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
  SegmentedControl,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 导入目标:教师与学生模板列不同,后端按 type 分别解析。 */
const IMPORT_TARGETS = [
  { value: 'teacher', label: '教师' },
  { value: 'student', label: '学生' },
] as const

type ImportTargetValue = (typeof IMPORT_TARGETS)[number]['value']

export interface AccountImportModalProps {
  onClose: () => void
  onCommitted: () => void
}

/**
 * AccountImportModal 承载导入预览与提交。
 */
export function AccountImportModal({ onClose, onCommitted }: AccountImportModalProps) {
  const [target, setTarget] = useState<ImportTargetValue>('student')
  const [file, setFile] = useState<File>()
  const [uploadProgress, setUploadProgress] = useState<number>()
  const [preview, setPreview] = useState<ImportPreviewResponse>()
  const [activationCodes, setActivationCodes] = useState<ImportActivationCode[]>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const runPreview = useCallback(async () => {
    if (!file) {
      setFormError('请选择要导入的文件')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.identity.previewAccountImport(target, file, setUploadProgress)
      setPreview(result)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '文件解析没有成功,请确认使用了最新模板。'))
    } finally {
      setWorking(false)
      setUploadProgress(undefined)
    }
  }, [file, target])

  const commit = useCallback(async () => {
    if (!preview) return
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.identity.commitAccountImport({ preview_id: preview.preview_id })
      // 激活码只在提交响应里出现一次,拿到就列出供管理员转交
      if (result.activation_codes && result.activation_codes.length > 0) {
        setActivationCodes(result.activation_codes)
        toast.success(`已开通 ${result.batch.success} 个账号,请转交激活码`)
        return
      }
      toast.success(`已开通 ${result.batch.success} 个账号`)
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

  const codeColumns: TableColumn<ImportActivationCode>[] = [
    { key: 'name', header: '姓名' },
    { key: 'no', header: '学工号', mono: true },
    {
      key: 'activation_code',
      header: '激活码',
      mono: true,
      render: (item) => <span className="font-mono text-sm text-ink">{item.activation_code}</span>,
    },
  ]

  // 激活码清单态:必须让管理员先记录,关闭即刷新列表
  if (activationCodes) {
    return (
      <Modal open onOpenChange={() => onCommitted()}>
        <ModalContent size="xl">
          <ModalHeader>
            <ModalTitle>导入完成,请转交激活码</ModalTitle>
            <ModalDescription>
              每个账号的激活码只显示这一次。请导出或抄录后分发给本人,关闭后无法再次查看。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-4">
            <Callout tone="warning">
              激活码等同于首次登录凭据,请通过本人可信的渠道转交,不要在群聊里公开。
            </Callout>
            <Table columns={codeColumns} data={activationCodes} rowKey={(item) => item.account_id} />
          </ModalBody>
          <ModalFooter>
            <Button variant="primary" onClick={() => onCommitted()}>
              我已记录,关闭
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    )
  }

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>批量导入账号</ModalTitle>
          <ModalDescription>
            先上传文件做校验预览,确认无误后提交开通。预览结果保存在服务器,中途离开也不会丢。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="导入对象" required helper="教师与学生的模板列不同,请选对再上传">
            <SegmentedControl
              aria-label="导入对象"
              options={IMPORT_TARGETS.map((item) => ({ value: item.value, label: item.label }))}
              value={target}
              onValueChange={(value) => {
                setTarget(value as ImportTargetValue)
                setPreview(undefined)
              }}
            />
          </FormField>

          <FormField
            label="导入文件"
            htmlFor="import-file"
            required
            helper="支持模板导出的 Excel 或 CSV 文件"
          >
            <Input
              id="import-file"
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
            <div className="flex flex-col gap-3 well p-4">
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
              上传后会逐行校验手机号、学工号与归属组织。校验通过才能提交。
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
                variant="primary"
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
              variant="primary"
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
