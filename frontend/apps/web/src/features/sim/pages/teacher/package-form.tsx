// 仿真场景提交表单(仿真场景页内弹层)。
//
// 后端 validateBundleManifestMatchesRequest 要求表单元数据与包内 sim-package.json 的 meta 逐字一致
// (code/version/name/category 与 scale_limit),故本表单把这些字段全部显式渲染,
// 并在说明里点明「必须与包内声明一致」—— 不一致后端会拒绝,提前说清比事后报错好。
//
// 表单没有「运行方式」:执行位置按代码来源派生,教师与第三方提交的场景一律在平台的隔离容器内运行
// (见 docs/04-仿真可视化引擎/02-架构设计.md §8),提交请求里不存在这个字段,也就没有可选项。
//
// 编号命名空间由后端按登录账号强制为 teacher_<账号编号>__,页面据此在新建时给出前缀提示。

import { useCallback, useState } from 'react'
import { Upload } from 'lucide-react'
import type { SimPackageMeta } from '@chaimir/api-client'
import {
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
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useSession } from '../../../../components/RoleGuard'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 规模上限的三个键与后端 manifest meta.scale_limit 一致。 */
interface ScaleLimitForm {
  nodes: string
  maxTick: string
  maxEvents: string
}

/** 规模上限默认值:与 sim-sdk 作者模板同量级,教师按场景实际规模调整。 */
const DEFAULT_SCALE_LIMIT: ScaleLimitForm = { nodes: '50', maxTick: '5000', maxEvents: '10000' }

export interface SimPackageFormModalProps {
  /** 传入即为更新已有包(只有草稿与已退回可更新);缺省为新提交 */
  item?: SimPackageMeta
  onClose: () => void
  onSaved: () => void
}

/**
 * SimPackageFormModal 承载仿真包提交与更新。
 */
export function SimPackageFormModal({ item, onClose, onSaved }: SimPackageFormModalProps) {
  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{item ? '更新仿真场景' : '提交仿真场景'}</ModalTitle>
          <ModalDescription>
            表单里的编号、版本、名称、分类与规模上限必须与场景包内的声明完全一致,否则平台会拒收。
            场景提交后由平台在隔离环境里试跑并审核,通过后才能被课程使用。
          </ModalDescription>
        </ModalHeader>
        <PackageForm item={item} onClose={onClose} onSaved={onSaved} />
      </ModalContent>
    </Modal>
  )
}

interface PackageFormProps {
  item?: SimPackageMeta
  onClose: () => void
  onSaved: () => void
}

/**
 * PackageForm 渲染表单本体并提交 multipart 上传。
 */
function PackageForm({ item, onClose, onSaved }: PackageFormProps) {
  const { me } = useSession()
  const editing = item !== undefined

  const [code, setCode] = useState(item?.code ?? `teacher_${me.account.id}__`)
  const [version, setVersion] = useState(item?.version ?? '1.0.0')
  const [name, setName] = useState(item?.name ?? '')
  const [category, setCategory] = useState(item?.category ?? '')
  const [scale, setScale] = useState<ScaleLimitForm>(() => readScaleLimit(item))
  const [file, setFile] = useState<File>()
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [progress, setProgress] = useState<number>()
  const [submitting, setSubmitting] = useState(false)

  const codePrefix = `teacher_${me.account.id}__`

  const validate = useCallback((): boolean => {
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
        [scale.nodes, scale.maxTick, scale.maxEvents].every((value) => Number(value) > 0)
          ? null
          : '规模上限三项都要是大于 0 的整数',
    }
    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [category, code, codePrefix, file, name, scale, version])

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
    <form onSubmit={submit} noValidate>
      <ModalBody className="flex flex-col gap-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            label="场景编号"
            htmlFor="sim-code"
            required
            error={errors.code}
            helper={editing ? '更新时编号与版本不可修改' : `以 ${codePrefix} 开头`}
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

        <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
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
                value={scale.nodes}
                onChange={(event) => setScale((current) => ({ ...current, nodes: event.target.value }))}
              />
            </FormField>
            <FormField label="最大推演步数" htmlFor="sim-max-tick" required className="mb-0">
              <Input
                id="sim-max-tick"
                type="number"
                min="1"
                value={scale.maxTick}
                onChange={(event) => setScale((current) => ({ ...current, maxTick: event.target.value }))}
              />
            </FormField>
            <FormField label="最大事件数" htmlFor="sim-max-events" required className="mb-0">
              <Input
                id="sim-max-events"
                type="number"
                min="1"
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
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          取消
        </Button>
        <Button type="submit" variant="seal" leftIcon={Upload} loading={submitting}>
          {editing ? '重新提交审核' : '提交审核'}
        </Button>
      </ModalFooter>
    </form>
  )
}

/** readScaleLimit 从已有包的规模上限回填表单;缺失键取默认值(后端存的是开放对象)。 */
function readScaleLimit(item?: SimPackageMeta): ScaleLimitForm {
  const raw = item?.scale_limit ?? {}
  return {
    nodes: readPositiveInt(raw.nodes) ?? DEFAULT_SCALE_LIMIT.nodes,
    maxTick: readPositiveInt(raw.max_tick) ?? DEFAULT_SCALE_LIMIT.maxTick,
    maxEvents: readPositiveInt(raw.max_events) ?? DEFAULT_SCALE_LIMIT.maxEvents,
  }
}

/** readPositiveInt 读取正整数字段;非正整数视为未设置。 */
function readPositiveInt(value: unknown): string | undefined {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? String(parsed) : undefined
}
