// 运行时详情(平台深页,/platform-admin/runtimes/:runtimeId)。设计 §7.6:
// 页头为运行时名 + 状态 +「教师可否选用」,主操作按状态给:没有镜像时「登记镜像版本」,有镜像未自检时「运行自检」,
// 已可用时不给;下方接入流程条,卡住的一步高亮并写原因;主体三个分组:
// 「镜像版本」表格(版本、镜像摘要、组合预拉取状态与进度、「预拉取」「停用」)、
// 「接入自检」(运行自检、上次结果逐项列出)、「接入配置」只读摘要 +「修改配置」。指标条和「返回运行时列表」去掉。
//
// 后端口径:自检只要求有一个可用镜像版本,停用的运行时不能自检;预拉取按「镜像 + 已发布组合摘要」进行,
// 学生开环境时才校验对应组合的预拉取 —— 所以预拉取放在镜像表里按组合做,不算运行时开放的前置。
// 预拉取与自检都是长操作:先触发,再轮询真实状态,离开页面只停止观察,不打断服务端任务。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router'
import {
  CircleSlash,
  CloudDownload,
  Container,
  Plus,
  RefreshCw,
  Server,
  Settings2,
  ShieldCheck,
} from 'lucide-react'
import {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeSelftestStatus,
  RuntimeStatus,
  type SandboxPrepullStatus,
  type SandboxRuntime,
  type SandboxRuntimeImage,
  type SandboxRuntimeSelftestStatus,
} from '@chaimir/api-client'
import {
  Badge,
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
  Progress,
  Skeleton,
  StatusIndicator,
  Steps,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { appConfig } from '../../../../app/config'
import { ResourceState } from '../../../../components/ResourceState'
import { locationLabel, useAsyncResource, usePageLocation } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import {
  imagePrepullStatusLabel,
  runtimeImageStatusLabel,
  runtimeSelftestStatusLabel,
  runtimeStatusLabel,
} from '../../../../utils/labels/sandbox'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ONBOARDING_STEPS, runtimeOnboarding } from '../../runtimeOnboarding'
import {
  runtimeContainerName,
  runtimeContainerPortCount,
  runtimeDisabledReason,
  runtimeHasDeployCommand,
  runtimeSidecarCount,
  runtimeWorkspaceDir,
} from '../../runtimeSpec'
import { selftestReasonText, selftestResultText, selftestStageText } from '../../selftestText'
import {
  imagePrepullStatusTone,
  runtimeSelftestStatusTone,
  runtimeStatusTone,
} from '../../statusPresentation'
import { RuntimeFormModal } from './runtime-form'

/** 适配层级的一句话说明 */
const ADAPTER_LEVEL_LABELS: Record<number, string> = {
  1: '只托管环境',
  2: '声明标准链能力',
  3: '自带插件实现',
}

/** PlatformRuntimeDetailPage 一条运行时的镜像、自检与接入配置 */
export default function PlatformRuntimeDetailPage() {
  const { runtimeId = '' } = useParams<{ runtimeId: string }>()
  // 自检结果由页面持有:运行时重新读取时不丢失刚拿到的分步结果
  const [selftestResult, setSelftestResult] = useState<SandboxRuntimeSelftestStatus>()
  const [imageFormOpen, setImageFormOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)

  const runtime = useAsyncResource(
    () => api.sandbox.getRuntime(runtimeId),
    [runtimeId],
    () => false
  )
  const images = useAsyncResource(
    () => api.sandbox.listRuntimeImages(runtimeId),
    [runtimeId],
    (value) => value.length === 0
  )
  const reloadAll = useCallback(() => {
    runtime.reload()
    images.reload()
  }, [images, runtime])
  const selftest = useRuntimeSelftest(runtimeId, setSelftestResult, reloadAll)

  usePageLocation([{ label: locationLabel(runtime, (data) => data.name, '运行时详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={runtime}
        emptyIcon={Server}
        emptyTitle="没有找到这条运行时"
        emptyDescription="运行时可能已被移除,回链运行时重新选择。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) => {
          const imageList = images.data ?? []
          const availableImages = imageList.filter(
            (item) => item.status === RuntimeImageStatus.AVAILABLE
          ).length
          const imagesKnown = images.status === 'success' || images.status === 'empty'
          const progress = runtimeOnboarding({
            status: data.status,
            selftest: data.selftest_status,
            availableImages: imagesKnown ? availableImages : undefined,
            disabledReason: runtimeDisabledReason(data.adapter_spec),
          })
          const selectable =
            data.status === RuntimeStatus.AVAILABLE &&
            data.selftest_status === RuntimeSelftestStatus.PASSED
          const disabled = data.status === RuntimeStatus.DISABLED
          const blockers = selftestBlockers(data, availableImages)
          // 主操作按状态:没有镜像 → 登记镜像版本;有镜像未通过自检 → 运行自检;可用或已停用 → 不给
          const primary =
            disabled || selectable || !imagesKnown ? null : availableImages === 0 ? (
              <Button variant="primary" leftIcon={Plus} onClick={() => setImageFormOpen(true)}>
                登记镜像版本
              </Button>
            ) : data.selftest_status !== RuntimeSelftestStatus.PASSED ? (
              <Button
                variant="primary"
                leftIcon={ShieldCheck}
                loading={selftest.working}
                disabled={blockers.length > 0}
                onClick={() => void selftest.run()}
              >
                运行自检
              </Button>
            ) : null

          return (
            <>
              <PageHeader
                title={data.name}
                status={
                  <span className="flex flex-wrap items-center gap-2">
                    <StatusIndicator
                      tone={runtimeStatusTone(data.status)}
                      label={runtimeStatusLabel(data.status)}
                    />
                    <StatusIndicator
                      tone={selectable ? 'success' : 'neutral'}
                      appearance="text"
                      label={selectable ? '教师可以选用' : '教师暂不可选'}
                    />
                  </span>
                }
                meta={[
                  { value: <span className="font-mono">{data.code}</span> },
                  { value: data.eco },
                  { value: ADAPTER_LEVEL_LABELS[data.adapter_level] ?? '适配层级未登记' },
                ]}
                actions={primary}
              />

              <OnboardingFlow
                done={progress.done}
                reason={progress.next}
                failed={progress.failedIndex !== undefined}
              />

              <div className="mt-6 flex flex-col gap-6">
                <ImagesSection
                  runtimeId={runtimeId}
                  images={images}
                  onRegister={() => setImageFormOpen(true)}
                  onChanged={reloadAll}
                />
                <div className="grid gap-6 lg:grid-cols-2">
                  <SelftestCard
                    runtime={data}
                    result={selftestResult?.runtime_id === data.id ? selftestResult : undefined}
                    blockers={blockers}
                    working={selftest.working}
                    actionError={selftest.actionError}
                    onRun={() => void selftest.run()}
                    onLoad={() => void selftest.load()}
                  />
                  <ConfigCard runtime={data} onEdit={() => setConfigOpen(true)} />
                </div>
              </div>

              {imageFormOpen ? (
                <ImageFormModal
                  runtimeId={runtimeId}
                  onClose={() => setImageFormOpen(false)}
                  onSaved={() => {
                    setImageFormOpen(false)
                    reloadAll()
                  }}
                />
              ) : null}
              {configOpen ? (
                <RuntimeFormModal
                  runtime={data}
                  onClose={() => setConfigOpen(false)}
                  onSaved={() => {
                    setConfigOpen(false)
                    reloadAll()
                  }}
                />
              ) : null}
            </>
          )
        }}
      </ResourceState>
    </PageScaffold>
  )
}

/** selftestBlockers 自检的前置不满足时说明缺什么(与后端同一口径),不让人点了才看到失败 */
function selftestBlockers(runtime: SandboxRuntime, availableImages: number): string[] {
  const out: string[] = []
  if (runtime.status === RuntimeStatus.DISABLED)
    out.push('运行时已停用,先在「修改配置」里改回接入中')
  if (availableImages === 0) out.push('还没有可用的镜像版本')
  const reason = runtimeDisabledReason(runtime.adapter_spec)
  if (reason) out.push(reason)
  return out
}

/** OnboardingFlow 接入流程条:走完的打勾,卡住的一步高亮并写原因 */
function OnboardingFlow({
  done,
  reason,
  failed,
}: {
  done: number
  reason?: string
  failed: boolean
}) {
  const steps = ONBOARDING_STEPS.map((label, index) => ({
    key: label,
    label,
    description: index === done ? reason : undefined,
  }))
  return (
    <Card>
      <CardBody>
        <Steps steps={steps} current={done} />
        {failed && reason ? <p className="mt-3 text-sm text-danger">{reason}</p> : null}
      </CardBody>
    </Card>
  )
}

interface ImagesSectionProps {
  runtimeId: string
  images: ReturnType<typeof useAsyncResource<SandboxRuntimeImage[]>>
  onRegister: () => void
  onChanged: () => void
}

/**
 * ImagesSection 镜像版本:登记、按组合预拉取(轮询真实节点状态)与停用。
 * 预拉取只针对一个已发布组合的完整镜像闭包,所以先填组合摘要;进度只属于当前摘要。
 */
function ImagesSection({ runtimeId, images, onRegister, onChanged }: ImagesSectionProps) {
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
        <span className="flex flex-wrap items-center gap-1.5">
          <span className="font-medium text-ink">{image.version}</span>
          {image.genesis_baked ? <Badge tone="neutral">内置创世</Badge> : null}
        </span>
      ),
    },
    {
      key: 'digest',
      header: '镜像摘要',
      render: (image) => {
        const digest = digestFromImageUrl(image.image_url)
        return (
          <span
            className="block max-w-56 truncate font-mono text-xs text-ink-sub"
            title={image.image_url}
          >
            {digest || '没有摘要'}
          </span>
        )
      },
    },
    {
      key: 'prepull',
      header: '组合预拉取',
      render: (image) => <PrepullCell status={prepullStates[image.id]} />,
    },
    {
      key: 'status',
      header: '状态',
      render: (image) => (
        <StatusIndicator
          appearance="text"
          tone={image.status === RuntimeImageStatus.AVAILABLE ? 'success' : 'neutral'}
          label={runtimeImageStatusLabel(image.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: <span className="sr-only">操作</span>,
      align: 'right',
      render: (image) => {
        const prepullStatus = prepullStates[image.id]?.prepull_status
        return (
          <span className="flex items-center justify-end gap-1">
            {image.status === RuntimeImageStatus.AVAILABLE &&
            prepullStatus !== ImagePrepullStatus.SUCCEEDED &&
            prepullStatus !== ImagePrepullStatus.RUNNING ? (
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
              查看进度
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
          </span>
        )
      },
    },
  ]

  return (
    <Card>
      <CardHeader
        title="镜像版本"
        description="镜像要来自平台私有仓库并通过签名与漏洞扫描。预拉取按已发布组合进行,完成后学生开这个组合的环境不用等下载。"
        actions={
          <Button variant="outline" size="sm" leftIcon={Plus} onClick={onRegister}>
            登记镜像版本
          </Button>
        }
      />
      <CardBody className="flex flex-col gap-4">
        <FormField
          label="已发布组合摘要"
          htmlFor="prepull-digest"
          helper="预拉取与进度只针对这个组合锁定的完整镜像"
        >
          <Input
            id="prepull-digest"
            value={compositionDigest}
            disabled={busyKey !== undefined}
            placeholder="sha256:..."
            className="font-mono text-sm sm:max-w-md"
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
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
        <ResourceState
          resource={images}
          emptyIcon={Container}
          emptyTitle="还没有登记镜像版本"
          emptyDescription="登记一个带不可变摘要的镜像版本后,才能运行自检。"
          emptyAction={
            <Button variant="outline" leftIcon={Plus} onClick={onRegister}>
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
              // <md 换行卡:版本一行、摘要一行,状态在右
              mobileCard={(item) => ({
                title: item.version,
                meta: digestFromImageUrl(item.image_url) || '没有摘要',
                badge: (
                  <StatusIndicator
                    appearance="text"
                    tone={item.status === RuntimeImageStatus.AVAILABLE ? 'success' : 'neutral'}
                    label={runtimeImageStatusLabel(item.status)}
                  />
                ),
              })}
            />
          )}
        </ResourceState>
      </CardBody>

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
    </Card>
  )
}

/** PrepullCell 当前组合的预拉取状态;进行中显示节点就绪进度 */
function PrepullCell({ status }: { status?: SandboxPrepullStatus }) {
  if (!status) return <span className="text-sm text-ink-sub">未查询</span>
  const share = status.desired_nodes > 0 ? (status.ready_nodes / status.desired_nodes) * 100 : 0
  return (
    <span className="flex min-w-32 flex-col gap-1">
      <StatusIndicator
        appearance="text"
        tone={imagePrepullStatusTone(status.prepull_status)}
        label={imagePrepullStatusLabel(status.prepull_status)}
      />
      {status.prepull_status === ImagePrepullStatus.RUNNING ? (
        <Progress value={share} label="节点就绪" />
      ) : status.desired_nodes > 0 ? (
        <span className="text-xs tabular-nums text-ink-sub">
          {status.ready_nodes}/{status.desired_nodes} 个节点就绪
        </span>
      ) : null}
    </span>
  )
}

interface SelftestCardProps {
  runtime: SandboxRuntime
  result?: SandboxRuntimeSelftestStatus
  blockers: string[]
  working: boolean
  actionError?: string
  onRun: () => void
  onLoad: () => void
}

/**
 * SelftestCard 接入自检:起一个一次性环境跑通工作区与链能力后立即销毁,通过后运行时才对学校开放。
 * 分步结果逐项列出;原因码转成用户向文案,不直接抛到界面上。
 */
function SelftestCard({
  runtime,
  result,
  blockers,
  working,
  actionError,
  onRun,
  onLoad,
}: SelftestCardProps) {
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
    <Card>
      <CardHeader
        title="接入自检"
        actions={
          <StatusIndicator
            tone={runtimeSelftestStatusTone(selftestStatus)}
            label={running ? '正在自检' : runtimeSelftestStatusLabel(selftestStatus)}
          />
        }
      />
      <CardBody className="flex flex-col gap-4">
        <p className="text-sm text-ink-sub">
          {running
            ? '自检正在进行,可以稍后读取结果。'
            : selftestStatus === RuntimeSelftestStatus.PASSED
              ? '已经通过自检。改了会影响启动方式的配置或换了镜像后,需要重新自检。'
              : '自检会起一个一次性环境,跑通工作区读写与链能力后立即销毁。镜像拉到节点、容器能起来都不算通过。'}
        </p>
        {blockers.length > 0 ? (
          <Callout tone="warning" title="还不能自检">
            {blockers.join(';')}。
          </Callout>
        ) : null}
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
        {detailItems.length > 0 ? (
          <DescriptionList dense items={detailItems} />
        ) : (
          <p className="text-sm text-ink-sub">
            还没有读到分步结果。可以读取上次结果,或开始一次新的自检。
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            leftIcon={ShieldCheck}
            loading={working}
            disabled={working || blockers.length > 0}
            onClick={onRun}
          >
            {failed || selftestStatus === RuntimeSelftestStatus.PASSED ? '重新自检' : '运行自检'}
          </Button>
          <Button variant="ghost" disabled={working} onClick={onLoad}>
            读取上次结果
          </Button>
        </div>
      </CardBody>
    </Card>
  )
}

/** ConfigCard 接入配置的只读摘要;改动在大抽屉里完成 */
function ConfigCard({ runtime, onEdit }: { runtime: SandboxRuntime; onEdit: () => void }) {
  return (
    <Card>
      <CardHeader
        title="接入配置"
        actions={
          <Button variant="outline" size="sm" leftIcon={Settings2} onClick={onEdit}>
            修改配置
          </Button>
        }
      />
      <CardBody>
        <DescriptionList
          dense
          items={[
            {
              term: '适配层级',
              description: ADAPTER_LEVEL_LABELS[runtime.adapter_level] ?? '未登记',
            },
            { term: '链能力来源', description: capabilitySourceLabel(runtime) },
            {
              term: '工作区目录',
              description: runtimeWorkspaceDir(runtime.adapter_spec) || '未填写',
              mono: true,
            },
            {
              term: '主环境',
              description: runtimeContainerName(runtime.adapter_spec) || '未填写',
              mono: true,
            },
            {
              term: '对外端口',
              description: `${runtimeContainerPortCount(runtime.adapter_spec)} 个`,
            },
            { term: '附加组件', description: `${runtimeSidecarCount(runtime.adapter_spec)} 个` },
          ]}
        />
      </CardBody>
    </Card>
  )
}

/**
 * capabilitySourceLabel 说明这条运行时的链能力从哪来。
 * 三种来源(内置实现 / 插件 / 声明的能力命令)由后端三选一要求,界面直接说结论。
 */
function capabilitySourceLabel(runtime: SandboxRuntime): string {
  if (runtime.plugin_ref.trim() !== '') return '运行时插件'
  if (runtime.capability_impl.trim() !== '') return '平台内置实现'
  if (runtimeHasDeployCommand(runtime.adapter_spec)) return '声明的链能力命令'
  return '只托管环境,链操作由学生自己完成'
}

/**
 * useRuntimeSelftest 触发自检并轮询结果;读取上次结果。
 * 触发请求可能被入口层中止,所以先短暂等待触发结果,再按查询接口轮询真实状态;离页后不再写回界面。
 */
function useRuntimeSelftest(
  runtimeId: string,
  onResult: (result: SandboxRuntimeSelftestStatus) => void,
  onDone: () => void
) {
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

  const showSelftestResult = useCallback(
    (status: RuntimeSelftestStatus) => {
      if (status === RuntimeSelftestStatus.PASSED) {
        toast.success('自检通过,运行时已开放给学校')
      } else {
        toast.error('自检没有通过,请查看接入自检里的原因。')
      }
      onDone()
    },
    [onDone]
  )

  const run = useCallback(async () => {
    if (actionInFlightRef.current) return
    actionInFlightRef.current = true
    setWorking(true)
    setActionError(undefined)
    let started = false
    let triggerError: unknown
    let queryError: unknown
    try {
      let triggeredResult: SandboxRuntimeSelftestStatus | undefined
      const triggerPromise = api.sandbox.runRuntimeSelftest(runtimeId).then((status) => {
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
          const queriedResult = triggeredResult ?? (await api.sandbox.getRuntimeSelftest(runtimeId))
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
  }, [onResult, runtimeId, showSelftestResult])

  const load = useCallback(async () => {
    if (actionInFlightRef.current) return
    actionInFlightRef.current = true
    setWorking(true)
    setActionError(undefined)
    try {
      const status = await api.sandbox.getRuntimeSelftest(runtimeId)
      if (!mountedRef.current) return
      onResult(status)
      if (status.selftest_status === RuntimeSelftestStatus.PENDING) {
        toast.info(
          status.detail.result === 'running'
            ? '自检仍在进行,请稍后读取结果。'
            : '这条运行时还没有自检。'
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
  }, [onResult, runtimeId, showSelftestResult])

  return { working, actionError, run, load }
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
        toast.success('镜像版本已登记,接下来运行自检')
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
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>登记镜像版本</ModalTitle>
          <ModalDescription>
            镜像要来自平台私有仓库,并且已经过签名验证与漏洞扫描,不满足的会被拒绝登记。
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
                placeholder="仓库地址@sha256:内容摘要"
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
            <Checkbox
              checked={genesisBaked}
              label="镜像内已内置创世状态"
              onCheckedChange={(checked) => setGenesisBaked(checked === true)}
            />
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" disabled={working} onClick={onClose}>
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

/** DisableImageModal 停用一个镜像版本:不再用于新建环境,已在运行的环境不受影响 */
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
    <Modal open onOpenChange={(open) => !open && !working && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>停用镜像版本 {image.version}</ModalTitle>
          <ModalDescription>
            停用后不再用于新建环境。已在运行的环境不受影响,直到自然回收。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <p className="break-all font-mono text-xs text-ink-sub">{image.image_url}</p>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
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
