// 运行时详情页(平台深页,/platform-admin/runtimes/:runtimeId)。
//
// 这里做完运行时从登记到可用的后三步:登记镜像版本 → 为已发布组合预拉取 → 平台自检。
// 镜像证明、组合预拉取、平台自检和调度状态分别核对;创世初始化由运行时自己的声明处理。
// 镜像拉到节点或容器起得来都不算通过,不能把内置创世当成所有链的自检门禁。
// 预拉取属于组合闭包,必须显式输入 composition_digest;运行时镜像本身不保存默认或单布尔状态。
//
// 镜像摘要不让人手填:后端要求 digest 与镜像地址里的 @sha256: 部分完全一致,
// 手抄两遍必然出错 —— 这里从地址里解析出来只读展示。
//

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import {
  CircleSlash,
  CloudDownload,
  Container,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
  ArrowLeft,
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
import { useFormValidation } from '../../../../hooks/useFormValidation'
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
  // 目录刷新会卸载内容区,公开自检结果由页面持有,避免刚读取的失败详情随之消失。
  const [selftestResult, setSelftestResult] = useState<SandboxRuntimeSelftestStatus>()

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
        actions={
          <Button
            variant="outline"
            size="sm"
            leftIcon={ArrowLeft}
            onClick={() => navigate('/platform-admin/runtimes')}
          >
            返回列表
          </Button>
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
              key={runtimeId}
              runtimeId={runtimeId}
              images={images}
              onChanged={() => {
                images.reload()
                runtime.reload()
              }}
            />

            <SelftestSection
              key={runtimeId}
              runtime={data}
              images={images.data ?? []}
              result={selftestResult?.runtime_id === data.id ? selftestResult : undefined}
              onResult={setSelftestResult}
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
  const selftestImages = images.filter((item) => item.status === RuntimeImageStatus.AVAILABLE)

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
          { label: '可自检版本', value: `${selftestImages.length} 个` },
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
        description="镜像证明、组合分发、平台自检和调度状态分别核对。初始链状态按运行时声明准备,环境启动后仍须完成真实操作验证。"
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
                    : '没有内置创世的版本,按运行时声明初始化',
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
  const [busyKey, setBusyKey] = useState<string>()
  const [compositionDigest, setCompositionDigest] = useState('')
  const [prepullStates, setPrepullStates] = useState<Record<string, SandboxPrepullStatus>>({})
  const [disableTarget, setDisableTarget] = useState<SandboxRuntimeImage>()
  const [actionError, setActionError] = useState<string>()
  const actionInFlightRef = useRef(false)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      // 离页只结束页面观察,服务端已经接受的预拉取继续按原生命周期执行。
      mountedRef.current = false
    }
  }, [])

  /** prepull 触发预拉取并轮询真实节点状态,避免长请求被入口层中止后误报失败。 */
  const prepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      if (actionInFlightRef.current) return
      actionInFlightRef.current = true
      setBusyKey(`prepull/${image.id}`)
      setActionError(undefined)
      try {
        const digest = compositionDigest.trim()
        if (!digest) {
          setActionError('请先填写已发布组合摘要,再执行预拉取。')
          return
        }
        let triggeredStatus: SandboxPrepullStatus | undefined
        let started = false
        const triggerPromise = api.sandbox
          .prepullRuntimeImage(runtimeId, image.id, digest)
          .then((status) => {
            triggeredStatus = status
            return status
          })
        let triggerError: unknown
        let queryError: unknown
        const quickStatus = await Promise.race<SandboxPrepullStatus | undefined>([
          triggerPromise,
          wait(500).then(() => undefined),
        ])
        // 立即接住长请求失败;状态轮询负责展示任务结果,离页后不再写回旧界面。
        triggerPromise.catch((error) => {
          triggerError = error
        })
        if (!mountedRef.current) return
        if (quickStatus) {
          setPrepullStates((current) => ({ ...current, [image.id]: quickStatus }))
        }

        const deadline = Date.now() + appConfig.runtimeSelftestPollTimeoutMs
        let lastStatus: SandboxPrepullStatus | undefined = quickStatus
        while (mountedRef.current && Date.now() < deadline) {
          try {
            const queriedStatus =
              triggeredStatus ??
              (await api.sandbox.getRuntimeImagePrepull(runtimeId, image.id, digest))
            if (!mountedRef.current) return
            lastStatus = triggeredStatus ?? queriedStatus
            queryError = undefined
          } catch (error) {
            if (!mountedRef.current) return
            if (triggerError && !started) throw triggerError
            if (queryError) throw error
            queryError = error
            await wait(appConfig.runtimeSelftestPollIntervalMs)
            continue
          }

          if (lastStatus.prepull_status === ImagePrepullStatus.RUNNING) started = true
          // 触发尚未落库时GET可能先读到上次终态,不能把它当作本次任务结果。
          if (!started && !triggeredStatus) {
            if (triggerError) throw triggerError
            await wait(appConfig.runtimeSelftestPollIntervalMs)
            continue
          }
          setPrepullStates((current) => ({ ...current, [image.id]: lastStatus! }))

          if (lastStatus.prepull_status === ImagePrepullStatus.SUCCEEDED) {
            toast.success(`预拉取完成,${lastStatus.ready_nodes} 个节点已就绪`)
            return
          }
          if (lastStatus.prepull_status === ImagePrepullStatus.FAILED) {
            if (triggerError) throw triggerError
            throw new Error('预拉取没有完成,请读取进度后重试。')
          }
          await wait(appConfig.runtimeSelftestPollIntervalMs)
        }

        if (!mountedRef.current) return
        if (queryError) throw queryError
        if (lastStatus?.prepull_status === ImagePrepullStatus.RUNNING) {
          toast.info('预拉取仍在进行,请稍后查看进度。')
          return
        }
        if (triggerError) throw triggerError
        throw new Error('预拉取仍在进行,请稍后刷新进度。')
      } catch (error) {
        if (mountedRef.current) {
          setActionError(userFacingErrorMessage(error, '预拉取没有完成,请稍后查看进度。'))
        }
      } finally {
        actionInFlightRef.current = false
        if (mountedRef.current) setBusyKey(undefined)
      }
    },
    [compositionDigest, runtimeId]
  )

  /** refreshPrepull 只查询当前组合进度,不能重新触发或覆盖正在进行的尝试。 */
  const refreshPrepull = useCallback(
    async (image: SandboxRuntimeImage) => {
      if (actionInFlightRef.current) return
      actionInFlightRef.current = true
      setBusyKey(`query/${image.id}`)
      setActionError(undefined)
      try {
        const digest = compositionDigest.trim()
        if (!digest) {
          setActionError('请先填写已发布组合摘要,再查询预拉取进度。')
          return
        }
        const status = await api.sandbox.getRuntimeImagePrepull(runtimeId, image.id, digest)
        if (!mountedRef.current) return
        setPrepullStates((current) => ({ ...current, [image.id]: status }))
        if (status.prepull_status === ImagePrepullStatus.SUCCEEDED) {
          toast.success(`预拉取完成,${status.ready_nodes} 个节点已就绪`)
        } else if (status.prepull_status === ImagePrepullStatus.FAILED) {
          setActionError('预拉取没有完成,请重新发起预拉取。')
        } else {
          toast.info(
            status.prepull_status === ImagePrepullStatus.RUNNING
              ? '预拉取仍在进行,请稍后查看进度。'
              : '这份组合还未预拉取,请先发起预拉取。'
          )
        }
      } catch (error) {
        if (mountedRef.current) {
          setActionError(userFacingErrorMessage(error, '没能读到预拉取进度,请稍后重试。'))
        }
      } finally {
        actionInFlightRef.current = false
        if (mountedRef.current) setBusyKey(undefined)
      }
    },
    [compositionDigest, runtimeId]
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
          {prepullStates[image.id]?.prepull_status !== ImagePrepullStatus.SUCCEEDED &&
          prepullStates[image.id]?.prepull_status !== ImagePrepullStatus.RUNNING ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CloudDownload}
              loading={busyKey === `prepull/${image.id}`}
              disabled={busyKey !== undefined}
              onClick={() => void prepull(image)}
            >
              预拉取
            </Button>
          ) : null}
          <Button
            variant="ghost"
            size="sm"
            leftIcon={RefreshCw}
            loading={busyKey === `query/${image.id}`}
            disabled={busyKey !== undefined}
            onClick={() => void refreshPrepull(image)}
          >
            {prepullStates[image.id]?.prepull_status === ImagePrepullStatus.SUCCEEDED
              ? '刷新进度'
              : '查看进度'}
          </Button>
          {image.status === RuntimeImageStatus.AVAILABLE ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CircleSlash}
              disabled={busyKey !== undefined}
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
            disabled={busyKey !== undefined}
            placeholder="sha256:..."
            className="font-mono text-sm"
            onChange={(event) => {
              const digest = event.target.value
              if (digest.trim() !== compositionDigest.trim()) {
                // 进度只属于当前摘要,不能让上一次组合的成功或进行中状态冒用到新输入。
                setPrepullStates({})
                setActionError(undefined)
              }
              setCompositionDigest(digest)
            }}
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
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const digest = useMemo(() => digestFromImageUrl(imageUrl), [imageUrl])

  const { errors, validate } = useFormValidation(
    useCallback(
      () => ({
        imageUrl:
          digest === ''
            ? '镜像地址要带内容摘要,形如 harbor.example.com/chain/geth@sha256:...'
            : null,
        version: version.trim() === '' ? '请填写版本号' : null,
      }),
      [digest, version]
    )
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

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
    [digest, genesisBaked, imageUrl, onSaved, runtimeId, validate, version]
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
              自检需要至少一个可用镜像版本。初始链状态按运行时声明准备,不要求所有版本都内置创世状态。
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
  result?: SandboxRuntimeSelftestStatus
  onResult: (result: SandboxRuntimeSelftestStatus) => void
  onDone: () => void
}

/**
 * SelftestSection 触发自检并展示结果。
 * 自检会真起一个一次性沙箱跑通工作区与链能力,通过后运行时转为可用;
 * 前置不满足时按后端条件说明缺什么,不让人点了才看到失败。
 */
function SelftestSection({ runtime, images, result, onResult, onDone }: SelftestSectionProps) {
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const actionInFlightRef = useRef(false)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  const selftestImage = images.find((item) => item.status === RuntimeImageStatus.AVAILABLE)
  const blockers = useMemo(() => {
    const out: string[] = []
    if (!selftestImage) out.push('还没有可用的镜像版本')
    // 服务端已经写明不可部署原因时原样带出:自检点下去也只会按同一原因失败
    const reason = runtimeDisabledReason(runtime.adapter_spec)
    if (reason) out.push(reason)
    return out
  }, [runtime.adapter_spec, selftestImage])

  const showSelftestResult = useCallback(
    (status: RuntimeSelftestStatus) => {
      if (status === RuntimeSelftestStatus.PASSED) {
        toast.success('自检通过,运行时已开放给学校')
      } else {
        toast.error('自检没有通过,请查看下方原因。')
      }
      onDone()
    },
    [onDone]
  )

  const runSelftest = useCallback(async () => {
    if (actionInFlightRef.current) return
    actionInFlightRef.current = true
    setWorking(true)
    setActionError(undefined)
    let started = false
    let triggerError: unknown
    let queryError: unknown
    try {
      let triggeredResult: SandboxRuntimeSelftestStatus | undefined
      const triggerPromise = api.sandbox.runRuntimeSelftest(runtime.id).then((status) => {
        triggeredResult = status
        return status
      })
      const quickResult = await Promise.race<SandboxRuntimeSelftestStatus | undefined>([
        triggerPromise,
        wait(500).then(() => undefined),
      ])
      triggerPromise.catch((error) => {
        triggerError = error
      })
      if (!mountedRef.current) return

      const deadline = Date.now() + appConfig.runtimeSelftestPollTimeoutMs
      let lastResult = quickResult
      while (mountedRef.current && Date.now() < deadline) {
        try {
          const queriedResult =
            triggeredResult ?? (await api.sandbox.getRuntimeSelftest(runtime.id))
          if (!mountedRef.current) return
          lastResult = triggeredResult ?? queriedResult
          queryError = undefined
        } catch (error) {
          if (!mountedRef.current) return
          if (triggerError && !started) throw triggerError
          if (queryError) throw error
          queryError = error
          await wait(appConfig.runtimeSelftestPollIntervalMs)
          continue
        }

        if (lastResult.detail.result === 'running') started = true
        // POST的迟到完成结果属于本次自检;触发尚未确认时不能采纳GET读到的上次终态。
        if (!started && !triggeredResult) {
          if (triggerError) throw triggerError
          await wait(appConfig.runtimeSelftestPollIntervalMs)
          continue
        }
        onResult(lastResult)
        if (lastResult.selftest_status !== RuntimeSelftestStatus.PENDING) {
          showSelftestResult(lastResult.selftest_status)
          return
        }
        await wait(appConfig.runtimeSelftestPollIntervalMs)
      }

      if (!mountedRef.current) return
      if (queryError) throw queryError
      if (started) {
        toast.info('自检仍在进行,请稍后读取结果。')
        return
      }
      if (triggerError) throw triggerError
      toast.info('暂未读到本次自检结果,请稍后读取结果。')
    } catch (error) {
      if (mountedRef.current) {
        setActionError(userFacingErrorMessage(error, '暂时无法确认自检结果,请稍后读取结果。'))
      }
    } finally {
      actionInFlightRef.current = false
      if (mountedRef.current) setWorking(false)
    }
  }, [onResult, runtime.id, showSelftestResult])

  const loadSelftest = useCallback(async () => {
    if (actionInFlightRef.current) return
    actionInFlightRef.current = true
    setWorking(true)
    setActionError(undefined)
    try {
      const status = await api.sandbox.getRuntimeSelftest(runtime.id)
      if (!mountedRef.current) return
      onResult(status)
      if (status.selftest_status === RuntimeSelftestStatus.PENDING) {
        toast.info(
          status.detail.result === 'running'
            ? '自检仍在进行,请稍后读取结果。'
            : '这条运行时尚未自检,请点击开始自检。'
        )
      } else {
        showSelftestResult(status.selftest_status)
      }
    } catch (error) {
      if (mountedRef.current) {
        setActionError(userFacingErrorMessage(error, '没能读到自检结果,请稍后重试。'))
      }
    } finally {
      actionInFlightRef.current = false
      if (mountedRef.current) setWorking(false)
    }
  }, [onResult, runtime.id, showSelftestResult])

  // 自检明细只有四个稳定字段,逐个转成用户向文案:原因码不直接抛到界面上(§8 文案规范)
  const detailItems = useMemo(() => {
    const detail = result?.detail
    if (!detail) return []
    const out: { term: string; description: string; mono?: boolean }[] = []
    if (detail.result)
      out.push({ term: '自检结果', description: selftestResultText(detail.result) })
    if (detail.stage) out.push({ term: '停在哪一步', description: selftestStageText(detail.stage) })
    if (detail.reason) out.push({ term: '原因', description: selftestReasonText(detail.reason) })
    if (detail.trace_id) out.push({ term: '报障编号', description: detail.trace_id, mono: true })
    return out
  }, [result])

  const selftestStatus = result?.selftest_status ?? runtime.selftest_status
  const running = result?.detail.result === 'running'
  const failed = selftestStatus === RuntimeSelftestStatus.FAILED

  return (
    <PageSection
      title="接入自检"
      description="自检会真起一个一次性环境,跑通工作区读写与链能力后立即销毁。通过后运行时才对学校开放。"
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            loading={working}
            disabled={working}
            onClick={() => void loadSelftest()}
          >
            读取上次结果
          </Button>
          <Button
            variant="primary"
            leftIcon={ShieldCheck}
            loading={working}
            disabled={working || blockers.length > 0}
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
                {running
                  ? '自检正在进行,可以稍后读取结果。'
                  : selftestStatus === RuntimeSelftestStatus.PASSED
                    ? '这条运行时已经通过自检。改了声明或镜像版本后建议重新自检。'
                    : '自检通过后运行时才会转为可用。镜像已拉到节点、容器能起来都不算通过。'}
              </p>
            </div>
            <StatusIndicator
              tone={runtimeSelftestStatusTone(selftestStatus)}
              label={running ? '正在自检' : runtimeSelftestStatusLabel(selftestStatus)}
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
