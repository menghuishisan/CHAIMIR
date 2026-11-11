// 账号批量导入(账号管理页的流程页)。
//
// FE-7:导入分两步且中间态在服务端 —— 上传得到预览批次(preview_id),
// 确认后再提交。刷新页面不会丢已上传的文件,重新打开可继续用同一预览提交。
//
// 预览会逐行校验:有错误行时明确列出行号与原因,让管理员改表再来,
// 而不是让部分成功部分失败(后端 commit 以预览为准,不重新解析文件)。

import { useCallback, useState } from 'react'
import { Link } from 'react-router'
import { Download } from 'lucide-react'
import type { ImportActivationCode, ImportPreviewResponse } from '@chaimir/api-client'
import {
  Button,
  Callout,
  FormField,
  Input,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Progress,
  SegmentedControl,
  Steps,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ImportActionFooter } from '../../components/ImportActionFooter'
import { ImportPreviewPanel } from '../../components/ImportPreviewPanel'
import { ImportPreviewRecovery } from '../../components/ImportPreviewRecovery'
import { AccountImportBatches } from './account-batches'

/** 导入目标:教师与学生模板列不同,后端按 type 分别解析。 */
const IMPORT_TARGETS = [
  { value: 'teacher', label: '教师' },
  { value: 'student', label: '学生' },
] as const

type ImportTargetValue = (typeof IMPORT_TARGETS)[number]['value']

/** 导入分三步(设计 §7.5):下载模板 → 上传并预览校验 → 确认导入 */
const IMPORT_STEPS = [
  { key: 'template', label: '下载模板' },
  { key: 'preview', label: '上传并预览' },
  { key: 'commit', label: '确认导入' },
]

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
  const { errors, validate } = useFormValidation(() => ({
    file: file ? null : '请选择要导入的文件',
  }), target)
  // 导入时按组织名称匹配归属:教师要有院系、学生要有班级,还没建时先提示去建组织架构
  const org = useAsyncResource(
    () => Promise.all([api.identity.listDepartments(), api.identity.listClasses()]).then(([departments, classes]) => ({ departments, classes })),
    [],
    () => false,
  )
  const orgMissing =
    org.status === 'success' && (target === 'teacher' ? (org.data?.departments.length ?? 0) === 0 : (org.data?.classes.length ?? 0) === 0)

  /** downloadTemplate 下载当前导入对象的模板:文件名取自响应头,页面不自造 */
  const downloadTemplate = useCallback(async () => {
    setFormError(undefined)
    try {
      downloadAttachment(await api.identity.downloadAccountImportTemplate({ type: target }))
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '模板下载没有成功,请稍后重试。'))
    }
  }, [target])

  const runPreview = useCallback(async () => {
    if (working || !validate() || !file) return
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
  }, [file, target, validate, working])

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
            <Table
              columns={codeColumns}
              data={activationCodes}
              rowKey={(item) => item.account_id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):姓名一行、学工号一行,激活码等宽显示在右
              mobileCard={(item) => ({
                title: item.name,
                meta: <span className="font-mono">{item.no}</span>,
                badge: <span className="font-mono text-sm text-ink">{item.activation_code}</span>,
              })}
            />
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
    <Drawer open onOpenChange={(open) => !open && !working && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>批量导入账号</DrawerTitle>
          <DrawerDescription>
            先上传文件做校验预览,确认无误后提交开通。预览结果保存在服务器,中途离开也不会丢。
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-4">
          <Steps steps={IMPORT_STEPS} current={preview ? 2 : file ? 1 : 0} />
          {orgMissing ? (
            <Callout tone="warning" title={target === 'teacher' ? '还没有院系' : '还没有班级'}>
              <p>
                导入时按名称匹配{target === 'teacher' ? '院系' : '班级'},请先建好组织架构。{' '}
                <Link to="/school-admin/organization" className="font-medium text-primary underline-offset-2 hover:underline">
                  先去建组织架构
                </Link>
              </p>
            </Callout>
          ) : null}
          <FormField label="导入对象" required helper="教师与学生的模板列不同,请选对再下载模板和上传">
            <SegmentedControl
              aria-label="导入对象"
              options={IMPORT_TARGETS.map((item) => ({ value: item.value, label: item.label }))}
              value={target}
              disabled={working}
              onValueChange={(value) => {
                setTarget(value as ImportTargetValue)
                setPreview(undefined)
                setFile(undefined)
              }}
            />
          </FormField>

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" leftIcon={Download} disabled={working} onClick={() => void downloadTemplate()}>
              下载{target === 'teacher' ? '教师' : '学生'}导入模板
            </Button>
            <span className="text-sm text-ink-sub">按模板列填写,不要改表头。</span>
          </div>

          <FormField
            label="导入文件"
            htmlFor="import-file"
            required
            error={errors.file}
            helper="支持模板导出的 Excel 或 CSV 文件"
          >
            <Input
              id="import-file"
              key={target}
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

          {!preview ? <ImportPreviewRecovery key={target} target={target} disabled={working} onRestore={setPreview} /> : null}

          <ImportPreviewPanel
            preview={preview}
            emptyDescription="上传后会逐行校验手机号、学工号与归属组织。校验通过才能提交。"
          />

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {!preview ? <AccountImportBatches /> : null}
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
