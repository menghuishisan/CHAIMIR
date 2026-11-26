// 教师提交和更新仿真场景包，表单元数据必须与包内声明一致。

import { useCallback, useState } from 'react'
import { Upload } from 'lucide-react'
import type { SimPackageMeta } from '@chaimir/api-client'
import {
  Button,
  Callout,
  FormField,
  Input,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { useSession } from '../../../../components/RoleGuard'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

interface ScaleLimitForm {
  nodes: string
  maxTick: string
  maxEvents: string
}

const DEFAULT_SCALE_LIMIT: ScaleLimitForm = { nodes: '50', maxTick: '5000', maxEvents: '10000' }
const SCALE_LIMIT_MAX = { nodes: 10_000, maxTick: 100_000, maxEvents: 100_000 }

export interface SimPackageFormModalProps {
  /** 传入即为更新已有包(只有草稿与已退回可更新);缺省为新提交 */
  item?: SimPackageMeta
  onClose: () => void
  onSaved: () => void
}

export function SimPackageFormModal({ item, onClose, onSaved }: SimPackageFormModalProps) {
  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>{item ? '更新仿真场景' : '提交仿真场景'}</DrawerTitle>
          <DrawerDescription>
            表单里的编号、版本、名称、分类与规模上限必须与场景包内的声明完全一致,否则平台会拒收。
            场景提交后由平台在隔离环境里试跑并审核,通过后才能被课程使用。
          </DrawerDescription>
        </DrawerHeader>
        <PackageForm item={item} onClose={onClose} onSaved={onSaved} />
      </DrawerContent>
    </Drawer>
  )
}

interface PackageFormProps {
  item?: SimPackageMeta
  onClose: () => void
  onSaved: () => void
}

function PackageForm({ item, onClose, onSaved }: PackageFormProps) {
  const { me } = useSession()
  const editing = item !== undefined

  const [code, setCode] = useState(item?.code ?? `teacher_${me.account.id}__`)
  const [version, setVersion] = useState(item?.version ?? '1.0.0')
  const [name, setName] = useState(item?.name ?? '')
  const [category, setCategory] = useState(item?.category ?? '')
  const [scale, setScale] = useState<ScaleLimitForm>(() => readScaleLimit(item))
  const [file, setFile] = useState<File>()
  const [formError, setFormError] = useState<string>()
  const [progress, setProgress] = useState<number>()
  const [submitting, setSubmitting] = useState(false)

  const codePrefix = `teacher_${me.account.id}__`

  const { errors, validate } = useFormValidation(useCallback(() => {
    const next: Record<string, string | null> = {
      code: /^[a-z][a-z0-9_]{1,31}__[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/.test(code.trim())
        ? code.trim().startsWith(codePrefix)
          ? null
          : `编号必须以 ${codePrefix} 开头,这是你的场景命名空间`
        : '编号形如 teacher_账号编号__场景名,场景名用小写字母、数字与连字符',
      version: /^\d+\.\d+\.\d+$/.test(version.trim()) ? null : '版本号形如 1.0.0',
      name: name.trim() === '' ? '请输入场景名称' : null,
      category: /^[a-z][a-z0-9_-]{1,31}$/.test(category.trim())
        ? null
        : '分类用小写字母、数字、下划线或连字符,如 consensus',
      file: file ? null : '请选择场景包文件',
      scale:
        validScaleLimit(scale.nodes, SCALE_LIMIT_MAX.nodes) &&
        validScaleLimit(scale.maxTick, SCALE_LIMIT_MAX.maxTick) &&
        validScaleLimit(scale.maxEvents, SCALE_LIMIT_MAX.maxEvents)
          ? null
          : '节点数须为 1 到 10,000 的整数，步数和事件数须为 1 到 100,000 的整数。',
    }
    return next
  }, [category, code, codePrefix, file, name, scale, version]))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate() || !file) return

      setSubmitting(true)
      setFormError(undefined)
      const payload = {
        bundle: file,
        code: code.trim(),
        version: version.trim(),
        name: name.trim(),
        category: category.trim(),
        scale_limit: {
          nodes: Number(scale.nodes),
          max_tick: Number(scale.maxTick),
          max_events: Number(scale.maxEvents),
        },
      }
      try {
        if (editing) {
          await api.sim.updatePackage(item.id, payload, setProgress)
          toast.success('场景已更新并重新提交审核')
        } else {
          await api.sim.submitPackage(payload, setProgress)
          toast.success('场景已提交,等待平台审核')
        }
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(
            error,
            editing ? '场景更新失败,请检查包内容后重试。' : '场景提交失败,请检查包内容后重试。',
          ),
        )
      } finally {
        setSubmitting(false)
        setProgress(undefined)
      }
    },
    [category, code, editing, file, item?.id, name, onSaved, scale, validate, version],
  )

  return (
    <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
      <DrawerBody className="flex flex-col gap-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            label="场景短名"
            htmlFor="sim-code"
            required
            error={errors.code}
            helper={editing ? '更新时短名与版本不可修改' : `以 ${codePrefix} 开头`}
          >
            <Input
              id="sim-code"
              value={code}
              readOnly={editing}
              invalid={Boolean(errors.code)}
              onChange={(event) => setCode(event.target.value)}
            />
          </FormField>
          <FormField label="版本号" htmlFor="sim-version" required error={errors.version} helper="形如 1.0.0">
            <Input
              id="sim-version"
              value={version}
              readOnly={editing}
              invalid={Boolean(errors.version)}
              onChange={(event) => setVersion(event.target.value)}
            />
          </FormField>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="场景名称" htmlFor="sim-name" required error={errors.name}>
            <Input
              id="sim-name"
              value={name}
              invalid={Boolean(errors.name)}
              onChange={(event) => setName(event.target.value)}
            />
          </FormField>
          <FormField
            label="领域分类"
            htmlFor="sim-category"
            required
            error={errors.category}
            helper="如 consensus、crypto、network"
          >
            <Input
              id="sim-category"
              value={category}
              invalid={Boolean(errors.category)}
              onChange={(event) => setCategory(event.target.value)}
            />
          </FormField>
        </div>

        <div className="flex flex-col gap-3 rounded-md bg-surface-hover p-4">
          <div className="text-sm font-medium text-ink">规模上限</div>
          <p className="text-xs text-ink-sub">
            运行时按这三个上限约束状态规模与执行步数。填写的值必须与场景包内声明的一致。
          </p>
          <div className="grid gap-4 sm:grid-cols-3">
            <FormField label="最大节点数" htmlFor="sim-nodes" required className="mb-0">
              <Input
                id="sim-nodes"
                type="number"
                min="1"
                max={SCALE_LIMIT_MAX.nodes}
                value={scale.nodes}
                onChange={(event) => setScale((current) => ({ ...current, nodes: event.target.value }))}
              />
            </FormField>
            <FormField label="最大推演步数" htmlFor="sim-max-tick" required className="mb-0">
              <Input
                id="sim-max-tick"
                type="number"
                min="1"
                max={SCALE_LIMIT_MAX.maxTick}
                value={scale.maxTick}
                onChange={(event) => setScale((current) => ({ ...current, maxTick: event.target.value }))}
              />
            </FormField>
            <FormField label="最大事件数" htmlFor="sim-max-events" required className="mb-0">
              <Input
                id="sim-max-events"
                type="number"
                min="1"
                max={SCALE_LIMIT_MAX.maxEvents}
                value={scale.maxEvents}
                onChange={(event) => setScale((current) => ({ ...current, maxEvents: event.target.value }))}
              />
            </FormField>
          </div>
          {errors.scale ? <Callout tone="danger">{errors.scale}</Callout> : null}
        </div>

        <FormField
          label="场景包文件"
          htmlFor="sim-bundle"
          required
          error={errors.file}
          helper="ZIP 或 TAR 归档,根目录需含 sim-package.json,并在 meta.entry 指明入口 .mjs 文件。平台会校验包内声明并做安全扫描。"
        >
          <Input
            id="sim-bundle"
            type="file"
            accept=".zip,.tar,.tar.gz,.tgz"
            invalid={Boolean(errors.file)}
            onChange={(event) => setFile(event.target.files?.[0])}
          />
        </FormField>

        {progress !== undefined ? (
          <Callout tone="info">场景包上传中 {progress}%,请不要关闭窗口。</Callout>
        ) : null}

        {formError ? <Callout tone="danger">{formError}</Callout> : null}
      </DrawerBody>
      <DrawerFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          取消
        </Button>
        <Button type="submit" variant="primary" leftIcon={Upload} loading={submitting}>
          {editing ? '重新提交审核' : '提交审核'}
        </Button>
      </DrawerFooter>
    </form>
  )
}

function readScaleLimit(item?: SimPackageMeta): ScaleLimitForm {
  if (!item) return DEFAULT_SCALE_LIMIT
  const { nodes, max_tick, max_events } = item.scale_limit
  return { nodes: String(nodes), maxTick: String(max_tick), maxEvents: String(max_events) }
}

function validScaleLimit(value: string, max: number): boolean {
  const number = Number(value)
  return Number.isInteger(number) && number > 0 && number <= max
}
