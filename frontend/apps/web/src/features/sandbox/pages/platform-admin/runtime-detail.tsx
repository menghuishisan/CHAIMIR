// 运行时详情页(平台深页,/platform-admin/runtimes/:runtimeId)。
//
// 这里做完运行时从登记到可用的后三步:登记镜像版本 → 为已发布组合预拉取 → 平台自检。
// 五件事分开呈现,任一未过运行时就不该出现在教师的可选项里(对齐清单 §6.3 / §8.3):
// 镜像证明、内置创世、组合预拉取、平台自检、调度状态 —— 镜像拉到节点或容器起得来都不算通过。
// 预拉取属于组合闭包,必须显式输入 composition_digest;运行时镜像本身不保存默认或单布尔状态。
//
// 镜像摘要不让人手填:后端要求 digest 与镜像地址里的 @sha256: 部分完全一致,
// 手抄两遍必然出错 —— 这里从地址里解析出来只读展示。
//

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
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
  RuntimeStatus,
  type SandboxRuntime,
  type SandboxRuntimeImage,
  type SandboxRuntimeSelftestStatus,
  type SandboxPrepullStatus,
  type SandboxRuntimeSelftestDetail,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Checkbox,
  DataPanel,
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
  ObjectIdentity,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { appConfig } from '../../../../app/config'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  imagePrepullStatusLabel,
  runtimeImageStatusLabel,
  runtimeSelftestStatusLabel,
  runtimeStatusLabel,
} from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  imagePrepullStatusTone,
  runtimeSelftestStatusTone,
  runtimeStatusTone,
} from '../../statusPresentation'
import {
  runtimeContainerName,
  runtimeContainerPortCount,
  runtimeDisabledReason,
  runtimeWorkspaceDir,
} from '../../runtimeSpec'
import { selftestReasonText, selftestResultText, selftestStageText } from '../../selftestText'

/**
 * PlatformRuntimeDetailPage 承载单条运行时的镜像、预拉取与自检。
 */
export default function PlatformRuntimeDetailPage() {
  const { runtimeId = '' } = useParams<{ runtimeId: string }>()
  const navigate = useNavigate()

  // 单读走 getRuntime:深链首屏不再拉全量列表在浏览器里筛这一条
  const runtime = useAsyncResource(
    () => api.sandbox.getRuntime(runtimeId),
    [runtimeId],
    () => false
  )
  const images = useAsyncResource(
    () => api.sandbox.listRuntimeImages(runtimeId),
    [runtimeId],
    () => false
  )

  return (
    <PageScaffold>
      {/*
        归族:详情族(§6.5.3 第 ④)。h1 由 ObjectIdentity 的运行时名承担,
        故页面头只出面包屑,末节到「链运行时」为止(§6.5.0 通则 1)。
      */}
      <PageHeader
        kicker={
          <Breadcrumb
            items={[{ label: '底层资源' }, { label: '链运行时', href: '/platform-admin/runtimes' }]}
          />
        }
      />

      <ResourceState
        resource={runtime}
        emptyIcon={Server}
        emptyTitle="运行时不存在"
        emptyDescription="这条运行时可能已被移除,回列表重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => (
          <>
            <RuntimeOverview
              runtime={data}
              images={images.data ?? []}
              onBack={() => navigate('/platform-admin/runtimes')}
            />

            <ImagesSection
              runtimeId={runtimeId}
              images={images}
              onChanged={() => {
                images.reload()
                runtime.reload()
              }}
            />

            <SelftestSection
              runtime={data}
              images={images.data ?? []}
              onDone={() => {
                runtime.reload()
                images.reload()
              }}
            />
          </>
        )}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * RuntimeOverview 渲染运行时的对象身份区与声明属性表。
 */
function RuntimeOverview({
  runtime,
  images,
  onBack,
}: {
  runtime: SandboxRuntime
  images: SandboxRuntimeImage[]
  onBack: () => void
}) {
  const disabledReason = runtimeDisabledReason(runtime.adapter_spec)
  const available = runtime.status === RuntimeStatus.AVAILABLE
  const passed = runtime.selftest_status === RuntimeSelftestStatus.PASSED
  // 只有「开放 + 自检通过」才算教师可选:两者缺一个都不能显示成可用(§8.3)
  const selectable = available && passed
  const genesisImages = images.filter(
    (item) => item.genesis_baked && item.status === RuntimeImageStatus.AVAILABLE
  )

  return (
    <>
      {/*
        对象身份区:运行时名 + 开放状态与自检状态 + 关键属性横排(§6.5.3 第 ④)。
        两个状态各自成一个指示灯 —— 「对学校开放了吗」与「自检过了吗」是两件独立的事,
        合成一个标签会让「已开放但自检失败」这种要紧组合读不出来。
        适配清单里的目录、端口、组件数是声明细节,下沉到属性表。
      */}
      <ObjectIdentity
        name={runtime.name}
        status={
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
        subtitle={`${runtime.code} · ${runtime.eco} · 适配层级 ${runtime.adapter_level}`}
        actions={
          <Button variant="outline" onClick={onBack}>
            返回运行时列表
          </Button>
        }
        properties={[
          { label: '教师可否选用', value: selectable ? '可以选用' : '暂不可选' },
          { label: '镜像版本数', value: `${images.length} 个` },
          { label: '可自检版本', value: `${genesisImages.length} 个` },
          { label: '对外端口', value: `${runtimeContainerPortCount(runtime.adapter_spec)} 个` },
          {
            label: '主环境',
            value: <span className="font-mono">{runtimeContainerName(runtime.adapter_spec)}</span>,
          },
        ]}
      />

      {!selectable ? (
        <Callout tone="warning" title="这条运行时现在不会出现在教师的可选项里" className="mt-4">
          {disabledReason ||
            (passed
              ? '自检已通过,但运行时还没有转为对学校开放。'
              : '要先通过平台自检,运行时才会转为对学校开放。')}
        </Callout>
      ) : null}

      <PageSection
        title="接入链路"
        description="镜像证明、内置创世、组合预拉取、平台自检、调度状态是五件独立的事,任一未过都不算可用 —— 镜像拉到节点或容器起得来都不代表通过。"
        className="mt-6"
      >
        <div className="rounded-lg bg-surface p-5 shadow-xs">
          <DescriptionList
            columns={2}
            items={[
              {
                term: '镜像证明',
                description:
                  images.length > 0
                    ? `${images.length} 个版本已登记(登记时校验私有仓库、签名与漏洞扫描)`
                    : '还没有登记镜像版本',
              },
              {
                term: '内置创世',
                description:
                  genesisImages.length > 0
                    ? `${genesisImages.map((item) => item.version).join('、')} 已内置创世状态`
                    : '还没有「已启用且内置创世」的版本,自检起不来',
              },
              {
                term: '组合预拉取',
                description: '按已发布组合把镜像闭包分发到各节点,在下方镜像版本区按组合摘要执行',
              },
              {
                term: '平台自检',
                description: `${runtimeSelftestStatusLabel(runtime.selftest_status)}${
                  passed ? '' : ' · 通过后才对学校开放'
                }`,
              },
              {
                term: '调度状态',
                description: available
                  ? `${runtimeStatusLabel(runtime.status)} · 学校可以选用`
                  : `${runtimeStatusLabel(runtime.status)} · 暂不分配给学校`,
              },
              {
                term: '不可用原因',
                description: disabledReason || '无',
              },
              {
                term: '工作区目录',
                description: runtimeWorkspaceDir(runtime.adapter_spec),
                mono: true,
              },
              {
                term: '能力实现',
                description: runtime.capability_impl || runtime.plugin_ref || '清单声明的能力命令',
                mono: true,
              },
            ]}
          />
        </div>
      </PageSection>
    </>
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
  const [compositionDigest, setCompositionDigest] = useState('')
  const [prepullStates, setPrepullStates] = useState<Record<string, SandboxPrepullStatus>>({})
  const [disableTarget, setDisableTarget] = useState<SandboxRuntimeImage>()
  const [actionError, setActionError] = useState<string>()

  /** prepull 触发预拉取:平台按节点逐台拉镜像,耗时较长,故完成后回报节点就绪数。 */
  const prepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      setBusyId(image.id)
      setActionError(undefined)
      try {
        const digest = compositionDigest.trim()
        if (!digest) {
          setActionError('请先填写已发布组合摘要,再执行预拉取。')
          return
        }
        const status = await api.sandbox.prepullRuntimeImage(runtimeId, image.id, digest)
        setPrepullStates((current) => ({ ...current, [image.id]: status }))
        toast.success(
          status.prepull_status === ImagePrepullStatus.SUCCEEDED
            ? `预拉取完成,${status.ready_nodes} 个节点已就绪`
            : `预拉取已开始,当前 ${status.ready_nodes}/${status.desired_nodes} 个节点就绪`
        )
        onChanged()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '预拉取没有启动成功,请稍后重试。'))
      } finally {
        setBusyId(undefined)
      }
    },
    [compositionDigest, onChanged, runtimeId]
  )

  /** refreshPrepull 重新读取预拉取进度:拉取在后台进行,页面不轮询,由管理员按需刷新。 */
  const refreshPrepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      setBusyId(image.id)
      setActionError(undefined)
      try {
        const digest = compositionDigest.trim()
        if (!digest) {
          setActionError('请先填写已发布组合摘要,再查询预拉取进度。')
          return
        }
        const status = await api.sandbox.getRuntimeImagePrepull(runtimeId, image.id, digest)
        setPrepullStates((current) => ({ ...current, [image.id]: status }))
        toast.success(`${status.ready_nodes}/${status.desired_nodes} 个节点已就绪`)
        onChanged()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '没能读到预拉取进度,请稍后重试。'))
      } finally {
        setBusyId(undefined)
      }
    },
    [compositionDigest, onChanged, runtimeId]
  )

  const columns: TableColumn<SandboxRuntimeImage>[] = [
    {
      key: 'version',
      header: '版本',
      render: (image) => (
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="font-medium text-ink">{image.version}</span>
            {image.genesis_baked ? <Badge tone="neutral">内置创世</Badge> : null}
          </div>
          <div className="truncate font-mono text-xs text-ink-sub">{image.image_url}</div>
        </div>
      ),
    },
    {
      key: 'prepull_status',
      header: '组合预拉取',
      render: (image) => {
        const status = prepullStates[image.id]
        return (
          <div className="flex flex-col gap-1">
            <StatusIndicator
              tone={status ? imagePrepullStatusTone(status.prepull_status) : 'neutral'}
              label={status ? imagePrepullStatusLabel(status.prepull_status) : '未查询'}
            />
          </div>
        )
      },
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
          {prepullStates[image.id]?.prepull_status === ImagePrepullStatus.SUCCEEDED ? (
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
              {prepullStates[image.id]?.prepull_status === ImagePrepullStatus.RUNNING
                ? '查看进度'
                : '预拉取'}
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

        <FormField label="已发布组合摘要" helper="预拉取只针对该摘要锁定的完整镜像闭包">
          <Input
            value={compositionDigest}
            placeholder="sha256:..."
            className="font-mono text-sm"
            onChange={(event) => setCompositionDigest(event.target.value)}
          />
        </FormField>

        {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):镜像清单一次回齐,不分页也不筛选 */}
        <DataPanel label="镜像版本">
          <ResourceState
            resource={images}
            emptyIcon={Container}
            emptyTitle="还没有登记镜像版本"
            emptyDescription="登记一个带不可变摘要且内置创世的镜像版本,通过组合预拉取后再自检。"
            emptyAction={
              <Button variant="outline" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
                登记镜像版本
              </Button>
            }
            skeleton={
              <Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />
            }
          >
            {(list) => (
              <Table
                columns={columns}
                data={list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):版本号一行、镜像摘要一行,预拉取状态在右
                mobileCard={(item) => ({
                  title: item.version,
                  meta: digestFromImageUrl(item.image_url),
                  badge: (
                    <Badge tone={item.genesis_baked ? 'neutral' : 'warning'}>
                      {item.genesis_baked ? '内置创世' : '未内置创世'}
                    </Badge>
                  ),
                })}
              />
            )}
          </ResourceState>
        </DataPanel>

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
        })
        toast.success('镜像版本已登记,接下来做预拉取')
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(
            error,
            '登记没有成功。请确认镜像来自平台私有仓库,且已完成签名与漏洞扫描。'
          )
        )
      } finally {
        setWorking(false)
      }
    },
    [digest, genesisBaked, imageUrl, onSaved, runtimeId, version]
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
                placeholder="填写仓库地址@内容摘要"
                invalid={Boolean(errors.imageUrl)}
                onChange={(event) => setImageUrl(event.target.value)}
              />
            </FormField>

            <FormField label="内容摘要" helper="从镜像地址里自动读取,不需要手填">
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
            </div>

            <Callout tone="info">
              自检要求至少有一个可用且内置创世的镜像版本。不内置创世的版本可以登记,但不能用来自检。
            </Callout>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
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
  const [detail, setDetail] = useState<SandboxRuntimeSelftestDetail>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const selftestImage = images.find(
    (item) => item.status === RuntimeImageStatus.AVAILABLE && item.genesis_baked
  )
  const blockers = useMemo(() => {
    const out: string[] = []
    if (!selftestImage) out.push('还没有可用且内置创世的镜像版本')
    // 服务端已经写明不可部署原因时原样带出:自检点下去也只会按同一原因失败
    const reason = runtimeDisabledReason(runtime.adapter_spec)
    if (reason) out.push(reason)
    return out
  }, [runtime.adapter_spec, selftestImage])

  const showSelftestResult = useCallback((status: RuntimeSelftestStatus) => {
    toast.success(status === RuntimeSelftestStatus.PASSED ? '自检通过,运行时已开放给学校' : '自检没有通过,详情见下方')
    onDone()
  }, [onDone])

  const runSelftest = useCallback(async () => {
    setWorking(true)
    setActionError(undefined)
    let started = false
    let triggerError: unknown
    try {
      const triggerPromise = api.sandbox.runRuntimeSelftest(runtime.id)
      const quickResult = await Promise.race([
        triggerPromise.then((result) => ({ result })),
        new Promise<{ result?: undefined }>((resolve) => window.setTimeout(() => resolve({}), 500)),
      ])
      if (quickResult.result) {
        started = true
        setDetail(quickResult.result.detail)
        if (quickResult.result.selftest_status !== RuntimeSelftestStatus.PENDING) {
          showSelftestResult(quickResult.result.selftest_status)
          return
        }
      }
      if (!quickResult.result) {
        triggerPromise.catch((error) => {
          triggerError = error
        })
      }

      const deadline = Date.now() + appConfig.runtimeSelftestPollTimeoutMs
      while (Date.now() < deadline) {
        let result: SandboxRuntimeSelftestStatus
        try {
          result = await api.sandbox.getRuntimeSelftest(runtime.id)
        } catch (error) {
          if (triggerError) throw triggerError
          triggerError = error
          await wait(appConfig.runtimeSelftestPollIntervalMs)
          continue
        }
        setDetail(result.detail)
        if (result.selftest_status === RuntimeSelftestStatus.PENDING) {
          started = true
        } else if (started) {
          showSelftestResult(result.selftest_status)
          return
        } else if (triggerError) {
          throw triggerError
        }
        await wait(appConfig.runtimeSelftestPollIntervalMs)
      }
      throw new Error('自检仍在进行,请稍后读取结果')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '自检没有跑完,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [runtime.id, showSelftestResult])

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

  // 自检明细只有四个稳定字段,逐个转成用户向文案:原因码不直接抛到界面上(§8 文案规范)
  const detailItems = useMemo(() => {
    if (!detail) return []
    const out: { term: string; description: string; mono?: boolean }[] = []
    if (detail.result) out.push({ term: '自检结果', description: selftestResultText(detail.result) })
    if (detail.stage) out.push({ term: '停在哪一步', description: selftestStageText(detail.stage) })
    if (detail.reason) out.push({ term: '原因', description: selftestReasonText(detail.reason) })
    if (detail.trace_id) out.push({ term: '报障编号', description: detail.trace_id, mono: true })
    return out
  }, [detail])

  const failed = runtime.selftest_status === RuntimeSelftestStatus.FAILED

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
            variant="primary"
            leftIcon={ShieldCheck}
            loading={working}
            disabled={blockers.length > 0}
            onClick={() => void runSelftest()}
          >
            {failed ? '重新自检' : '开始自检'}
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

        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <h3 className="text-base font-semibold text-ink">自检状态</h3>
              <p className="mt-0.5 text-sm text-ink-sub">
                {runtime.selftest_status === RuntimeSelftestStatus.PASSED
                  ? '这条运行时已经通过自检。改了声明或镜像版本后建议重新自检。'
                  : '自检通过后运行时才会转为可用。镜像已拉到节点、容器能起来都不算通过。'}
              </p>
            </div>
            <StatusIndicator
              tone={runtimeSelftestStatusTone(runtime.selftest_status)}
              label={runtimeSelftestStatusLabel(runtime.selftest_status)}
            />
          </div>

          {detailItems.length > 0 ? (
            <DescriptionList dense items={detailItems} />
          ) : (
            <p className="text-sm text-ink-sub">
              还没有读取到分步结果。点「读取上次结果」查看,或直接开始一次新的自检。
            </p>
          )}
        </div>
      </div>
    </PageSection>
  )
}

/** wait 等待下一次状态查询,避免高频请求占用平台接口。 */
function wait(milliseconds: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, milliseconds))
}

/** digestFromImageUrl 从带摘要的镜像地址里取出摘要,规则与后端 digestFromImageURL 一致。 */
function digestFromImageUrl(imageUrl: string): string {
  const parts = imageUrl.trim().split('@')
  if (parts.length !== 2 || !parts[1].startsWith('sha256:')) return ''
  return parts[1]
}
