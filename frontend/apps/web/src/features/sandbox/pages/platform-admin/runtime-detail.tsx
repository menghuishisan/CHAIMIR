// 运行时详情页(平台深页,/platform-admin/runtimes/:runtimeId)。
//
// 这里做完运行时从登记到可用的后三步:登记镜像版本 → 预拉取到各节点 → 自检。
// 顺序是后端的硬前置(RunRuntimeSelftest 要求默认镜像已预拉取成功且内置创世),
// 所以页面按这个顺序排,并在前置没满足时说明为什么还不能自检,而不是让人点了才知道。
//
// 镜像摘要不让人手填:后端要求 digest 与镜像地址里的 @sha256: 部分完全一致,
// 手抄两遍必然出错 —— 这里从地址里解析出来只读展示。
//
// 后端没有单条运行时读取接口,故从运行时列表里定位这一条。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  CircleSlash,
  CloudDownload,
  Container,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
} from 'lucide-react'
import {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeSelftestStatus,
  type SandboxRuntime,
  type SandboxRuntimeImage,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  DescriptionList,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  imagePrepullStatusLabel,
  imagePrepullStatusTone,
  runtimeImageStatusLabel,
  runtimeSelftestStatusLabel,
  runtimeSelftestStatusTone,
  runtimeStatusLabel,
  runtimeStatusTone,
} from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/**
 * PlatformRuntimeDetailPage 承载单条运行时的镜像、预拉取与自检。
 */
export default function PlatformRuntimeDetailPage() {
  const { runtimeId = '' } = useParams<{ runtimeId: string }>()
  const navigate = useNavigate()

  const runtimes = useAsyncResource(() => api.sandbox.listRuntimes(), [], () => false)
  const images = useAsyncResource(
    () => api.sandbox.listRuntimeImages(runtimeId),
    [runtimeId],
    () => false,
  )

  const runtime = useMemo(
    () => (runtimes.data ?? []).find((item) => item.id === runtimeId),
    [runtimeId, runtimes.data],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '底层资源' },
              { label: '链运行时', href: '/platform-admin/runtimes' },
              { label: runtime ? runtime.name : '运行时详情' },
            ]}
          />
        }
        title={runtime ? runtime.name : '运行时详情'}
        description="登记镜像版本、预拉取到各节点,再自检。自检通过后这条运行时才对学校开放。"
        icon={Server}
        actions={
          <Button variant="outline" onClick={() => navigate('/platform-admin/runtimes')}>
            返回运行时列表
          </Button>
        }
      />

      <ResourceState
        resource={runtimes}
        emptyIcon={Server}
        emptyTitle="还没有登记链运行时"
        emptyDescription="回列表登记第一条运行时。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {() =>
          runtime ? (
            <RuntimeOverview runtime={runtime} images={images.data ?? []} />
          ) : (
            <Callout tone="warning" title="没有找到这条运行时">
              运行时可能已被移除,回列表重新选择。
            </Callout>
          )
        }
      </ResourceState>

      <ImagesSection
        runtimeId={runtimeId}
        images={images}
        onChanged={() => {
          images.reload()
          runtimes.reload()
        }}
      />

      {runtime ? (
        <SelftestSection
          runtime={runtime}
          images={images.data ?? []}
          onDone={() => {
            runtimes.reload()
            images.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/**
 * RuntimeOverview 渲染运行时声明摘要与当前状态。
 */
function RuntimeOverview({
  runtime,
  images,
}: {
  runtime: SandboxRuntime
  images: SandboxRuntimeImage[]
}) {
  const defaultImage = images.find((item) => item.is_default)

  return (
    <PageSection title="当前状态" description="声明内容在运行时列表页的「修改声明」里改。">
      <div className="flex flex-col gap-4">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="开放状态"
            value={runtimeStatusLabel(runtime.status)}
            icon={Server}
            hint={runtime.status === 1 ? '学校可以选用' : '暂不分配给学校'}
          />
          <Stat
            label="自检结果"
            value={runtimeSelftestStatusLabel(runtime.selftest_status)}
            icon={ShieldCheck}
          />
          <Stat label="镜像版本数" value={images.length} icon={Container} />
          <Stat
            label="默认镜像"
            value={defaultImage ? defaultImage.version : '未指定'}
            icon={Container}
            hint={defaultImage ? '自检与新环境用这个版本' : '指定后才能自检'}
          />
        </div>

        <Card>
          <CardHeader
            title={runtime.code}
            description={`${runtime.eco} · 适配层级 ${runtime.adapter_level}`}
            actions={
              <div className="flex flex-wrap items-center gap-2">
                <StatusIndicator
                  tone={runtimeStatusTone(runtime.status)}
                  label={runtimeStatusLabel(runtime.status)}
                />
                <StatusIndicator
                  tone={runtimeSelftestStatusTone(runtime.selftest_status)}
                  label={runtimeSelftestStatusLabel(runtime.selftest_status)}
                />
              </div>
            }
          />
          <CardBody>
            <DescriptionList
              columns={2}
              items={[
                { term: '工作区目录', description: runtime.adapter_spec.workspace_dir, mono: true },
                {
                  term: '主容器',
                  description: runtime.adapter_spec.runtime_container.name,
                  mono: true,
                },
                {
                  term: '对外端口',
                  description: `${runtime.adapter_spec.runtime_container.ports?.length ?? 0} 个`,
                },
                {
                  term: '附加容器',
                  description: `${runtime.adapter_spec.infra_sidecars?.length ?? 0} 个`,
                },
                {
                  term: '默认工具',
                  description:
                    (runtime.adapter_spec.default_tool_codes ?? []).join('、') || '未指定',
                },
                {
                  term: '能力实现',
                  description: runtime.capability_impl || runtime.plugin_ref || '清单声明的能力命令',
                  mono: true,
                },
              ]}
            />
          </CardBody>
        </Card>
      </div>
    </PageSection>
  )
}

interface ImagesSectionProps {
  runtimeId: string
  images: ReturnType<typeof useAsyncResource<SandboxRuntimeImage[]>>
  onChanged: () => void
}

/**
 * ImagesSection 列出镜像版本并承载登记、预拉取与停用。
 */
function ImagesSection({ runtimeId, images, onChanged }: ImagesSectionProps) {
  const [createOpen, setCreateOpen] = useState(false)
  const [busyId, setBusyId] = useState<string>()
  const [disableTarget, setDisableTarget] = useState<SandboxRuntimeImage>()
  const [actionError, setActionError] = useState<string>()

  /** prepull 触发预拉取:平台按节点逐台拉镜像,耗时较长,故完成后回报节点就绪数。 */
  const prepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      setBusyId(image.id)
      setActionError(undefined)
      try {
        const status = await api.sandbox.prepullRuntimeImage(runtimeId, image.id)
        toast.success(
          status.prepull_status === ImagePrepullStatus.SUCCEEDED
            ? `预拉取完成,${status.ready_nodes} 个节点已就绪`
            : `预拉取已开始,当前 ${status.ready_nodes}/${status.desired_nodes} 个节点就绪`,
        )
        onChanged()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '预拉取没有启动成功,请稍后重试。'))
      } finally {
        setBusyId(undefined)
      }
    },
    [onChanged, runtimeId],
  )

  /** refreshPrepull 重新读取预拉取进度:拉取在后台进行,页面不轮询,由管理员按需刷新。 */
  const refreshPrepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      setBusyId(image.id)
      setActionError(undefined)
      try {
        const status = await api.sandbox.getRuntimeImagePrepull(runtimeId, image.id)
        toast.success(`${status.ready_nodes}/${status.desired_nodes} 个节点已就绪`)
        onChanged()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '没能读到预拉取进度,请稍后重试。'))
      } finally {
        setBusyId(undefined)
      }
    },
    [onChanged, runtimeId],
  )

  const columns: TableColumn<SandboxRuntimeImage>[] = [
    {
      key: 'version',
      header: '版本',
      render: (image) => (
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="font-medium text-ink">{image.version}</span>
            {image.is_default ? <Badge tone="jade">默认</Badge> : null}
            {image.genesis_baked ? <Badge tone="neutral">内置创世</Badge> : null}
          </div>
          <div className="truncate font-mono text-xs text-ink-sub">{image.image_url}</div>
        </div>
      ),
    },
    {
      key: 'prepull_status',
      header: '预拉取',
      render: (image) => (
        <div className="flex flex-col gap-1">
          <StatusIndicator
            tone={imagePrepullStatusTone(image.prepull_status)}
            label={imagePrepullStatusLabel(image.prepull_status)}
          />
          {image.prepulled_at ? (
            <span className="font-mono text-xs text-ink-faint">
              {formatDateTime(image.prepulled_at)}
            </span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (image) => (
        <Badge tone={image.status === RuntimeImageStatus.AVAILABLE ? 'success' : 'neutral'}>
          {runtimeImageStatusLabel(image.status)}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (image) => (
        <div className="flex items-center justify-end gap-1">
          {image.prepull_status === ImagePrepullStatus.SUCCEEDED ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={RefreshCw}
              loading={busyId === image.id}
              onClick={() => void refreshPrepull(image)}
            >
              刷新进度
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CloudDownload}
              loading={busyId === image.id}
              onClick={() => void prepull(image)}
            >
              {image.prepull_status === ImagePrepullStatus.RUNNING ? '查看进度' : '预拉取'}
            </Button>
          )}
          {image.status === RuntimeImageStatus.AVAILABLE ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CircleSlash}
              onClick={() => setDisableTarget(image)}
            >
              停用
            </Button>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <PageSection
      title="镜像版本"
      description="每个版本对应一个已签名的镜像。预拉取会把镜像分发到所有计算节点,学生创建环境时才不用等下载。"
      actions={
        <Button variant="outline" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
          登记镜像版本
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={images}
          emptyIcon={Container}
          emptyTitle="还没有登记镜像版本"
          emptyDescription="登记一个内置创世的版本并设为默认,预拉取成功后才能自检。"
          emptyAction={
            <Button variant="outline" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              登记镜像版本
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(list) => <Table columns={columns} data={list} rowKey={(item) => item.id} />}
        </ResourceState>

        <Callout tone="info">
          镜像必须来自平台私有仓库且通过签名与漏洞扫描,登记时后端会校验;停用版本不影响已在运行的环境。
        </Callout>
      </div>

      {createOpen ? (
        <ImageFormModal
          runtimeId={runtimeId}
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            onChanged()
          }}
        />
      ) : null}

      {disableTarget ? (
        <DisableImageModal
          runtimeId={runtimeId}
          image={disableTarget}
          onClose={() => setDisableTarget(undefined)}
          onDone={() => {
            setDisableTarget(undefined)
            onChanged()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface ImageFormModalProps {
  runtimeId: string
  onClose: () => void
  onSaved: () => void
}

/**
 * ImageFormModal 登记一个镜像版本。
 * 摘要从镜像地址里解析,不让人手填:后端要求两者完全一致,抄两遍只会引入错误。
 */
function ImageFormModal({ runtimeId, onClose, onSaved }: ImageFormModalProps) {
  const [imageUrl, setImageUrl] = useState('')
  const [version, setVersion] = useState('')
  const [genesisBaked, setGenesisBaked] = useState(true)
  const [isDefault, setIsDefault] = useState(true)
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const digest = useMemo(() => digestFromImageUrl(imageUrl), [imageUrl])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const next: Record<string, string | null> = {
        imageUrl:
          digest === ''
            ? '镜像地址要带内容摘要,形如 harbor.example.com/chain/geth@sha256:...'
            : null,
        version: version.trim() === '' ? '请填写版本号' : null,
      }
      setErrors(next)
      if (Object.values(next).some((value) => value !== null)) return

      setFormError(undefined)
      setWorking(true)
      try {
        await api.sandbox.registerRuntimeImage(runtimeId, {
          image_url: imageUrl.trim(),
          version: version.trim(),
          digest,
          genesis_baked: genesisBaked,
          is_default: isDefault,
        })
        toast.success('镜像版本已登记,接下来做预拉取')
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(
            error,
            '登记没有成功。请确认镜像来自平台私有仓库,且已完成签名与漏洞扫描。',
          ),
        )
      } finally {
        setWorking(false)
      }
    },
    [digest, genesisBaked, imageUrl, isDefault, onSaved, runtimeId, version],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>登记镜像版本</ModalTitle>
          <ModalDescription>
            镜像要来自平台私有仓库,并且已经过签名验证与漏洞扫描 —— 不满足的镜像会被拒绝登记。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="镜像地址"
              htmlFor="image-url"
              required
              error={errors.imageUrl}
              helper="必须带 @sha256 内容摘要,保证每次拉到的是同一份镜像"
            >
              <Input
                id="image-url"
                className="font-mono text-sm"
                value={imageUrl}
                placeholder="harbor.example.com/chain/geth@sha256:0f1e…"
                invalid={Boolean(errors.imageUrl)}
                onChange={(event) => setImageUrl(event.target.value)}
              />
            </FormField>

            <FormField
              label="内容摘要"
              helper="从镜像地址里自动读取,不需要手填"
            >
              <Input readOnly className="font-mono text-sm" value={digest || '等待填写镜像地址'} />
            </FormField>

            <FormField
              label="版本号"
              htmlFor="image-version"
              required
              error={errors.version}
              helper="教师与判题器按版本号引用这个镜像"
            >
              <Input
                id="image-version"
                value={version}
                placeholder="v1.14.0"
                invalid={Boolean(errors.version)}
                onChange={(event) => setVersion(event.target.value)}
              />
            </FormField>

            <div className="flex flex-col gap-2">
              <Checkbox
                checked={genesisBaked}
                label="镜像内已内置创世状态"
                onCheckedChange={(checked) => setGenesisBaked(checked === true)}
              />
              <Checkbox
                checked={isDefault}
                label="设为默认版本(自检与新建环境都用它)"
                onCheckedChange={(checked) => setIsDefault(checked === true)}
              />
            </div>

            <Callout tone="info">
              自检要求默认版本内置创世且预拉取成功。不内置创世的版本可以登记,但不能用来自检。
            </Callout>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              登记版本
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface DisableImageModalProps {
  runtimeId: string
  image: SandboxRuntimeImage
  onClose: () => void
  onDone: () => void
}

/**
 * DisableImageModal 停用一个镜像版本。
 */
function DisableImageModal({ runtimeId, image, onClose, onDone }: DisableImageModalProps) {
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      await api.sandbox.disableRuntimeImage(runtimeId, image.id)
      toast.success('镜像版本已停用')
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '停用没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [image.id, onDone, runtimeId])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>停用这个镜像版本</ModalTitle>
          <ModalDescription>
            停用后不再用于新建环境。已在运行的环境不受影响,直到自然回收。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '版本', description: image.version },
              { term: '镜像地址', description: image.image_url, mono: true },
            ]}
          />
          {image.is_default ? (
            <Callout tone="warning" title="这是默认版本">
              停用默认版本后,自检与新建环境会找不到可用镜像。请先登记新版本并设为默认。
            </Callout>
          ) : null}
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="danger" loading={working} onClick={() => void submit()}>
            确认停用
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface SelftestSectionProps {
  runtime: SandboxRuntime
  images: SandboxRuntimeImage[]
  onDone: () => void
}

/**
 * SelftestSection 触发自检并展示结果。
 * 自检会真起一个一次性沙箱跑通工作区与链能力,通过后运行时转为可用;
 * 前置不满足时按后端条件说明缺什么,不让人点了才看到失败。
 */
function SelftestSection({ runtime, images, onDone }: SelftestSectionProps) {
  const [detail, setDetail] = useState<Record<string, unknown>>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const defaultImage = images.find((item) => item.is_default)
  const blockers = useMemo(() => {
    const out: string[] = []
    if (!defaultImage) out.push('还没有设为默认的镜像版本')
    else {
      if (!defaultImage.genesis_baked) out.push('默认版本没有内置创世状态')
      if (defaultImage.prepull_status !== ImagePrepullStatus.SUCCEEDED)
        out.push('默认版本还没有在所有节点预拉取成功')
    }
    return out
  }, [defaultImage])

  const runSelftest = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    try {
      const result = await api.sandbox.runRuntimeSelftest(runtime.id)
      setDetail(result.detail)
      toast.success(
        result.selftest_status === RuntimeSelftestStatus.PASSED
          ? '自检通过,运行时已开放给学校'
          : '自检没有通过,详情见下方',
      )
      onDone()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '自检没有跑完,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [onDone, runtime.id])

  const loadSelftest = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    try {
      const result = await api.sandbox.getRuntimeSelftest(runtime.id)
      setDetail(result.detail)
      toast.success(`上次自检:${runtimeSelftestStatusLabel(result.selftest_status)}`)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '没能读到自检结果,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [runtime.id])

  const detailItems = useMemo(() => {
    if (!detail) return []
    return Object.entries(detail).map(([key, value]) => ({
      term: selftestStepLabel(key),
      description:
        typeof value === 'string'
          ? value
          : typeof value === 'boolean'
            ? value
              ? '通过'
              : '未通过'
            : typeof value === 'number'
              ? String(value)
              : '有更详细的记录,请查看服务端日志',
    }))
  }, [detail])

  return (
    <PageSection
      title="接入自检"
      description="自检会真起一个一次性环境,跑通工作区读写与链能力后立即销毁。通过后运行时才对学校开放。"
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" size="sm" loading={working} onClick={() => void loadSelftest()}>
            读取上次结果
          </Button>
          <Button
            variant="seal"
            leftIcon={ShieldCheck}
            loading={working}
            disabled={blockers.length > 0}
            onClick={() => void runSelftest()}
          >
            开始自检
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        {blockers.length > 0 ? (
          <Callout tone="warning" title="还不能自检">
            {blockers.join(';')}。补齐后再来。
          </Callout>
        ) : null}

        <Card>
          <CardHeader
            title="自检状态"
            description={
              runtime.selftest_status === RuntimeSelftestStatus.PASSED
                ? '这条运行时已经通过自检。改了声明或换了默认镜像后建议重新自检。'
                : '自检通过后运行时才会转为可用。'
            }
            actions={
              <StatusIndicator
                tone={runtimeSelftestStatusTone(runtime.selftest_status)}
                label={runtimeSelftestStatusLabel(runtime.selftest_status)}
              />
            }
          />
          <CardBody>
            {detailItems.length > 0 ? (
              <DescriptionList dense items={detailItems} />
            ) : (
              <p className="text-sm text-ink-sub">
                还没有读取到分步结果。点「读取上次结果」查看,或直接开始一次新的自检。
              </p>
            )}
          </CardBody>
        </Card>
      </div>
    </PageSection>
  )
}

/** digestFromImageUrl 从带摘要的镜像地址里取出摘要,规则与后端 digestFromImageURL 一致。 */
function digestFromImageUrl(imageUrl: string): string {
  const parts = imageUrl.trim().split('@')
  if (parts.length !== 2 || !parts[1].startsWith('sha256:')) return ''
  return parts[1]
}

/** 自检分步的用户向名称:未登记的键给通用名,不把内部标识抛到界面上。 */
const SELFTEST_STEP_LABELS: Record<string, string> = {
  workspace: '工作区读写',
  terminal: '终端接入',
  deploy: '合约部署',
  tx: '发起交易',
  query: '链上查询',
  reset: '重置链状态',
  message: '失败说明',
  error: '失败说明',
}

/** selftestStepLabel 返回自检分步名称。 */
function selftestStepLabel(key: string): string {
  return SELFTEST_STEP_LABELS[key] ?? '其他检查项'
}
