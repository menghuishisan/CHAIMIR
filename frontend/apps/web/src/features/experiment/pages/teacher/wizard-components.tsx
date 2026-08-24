// 实验编排向导:环境与仿真步(第 2 步)。
//
// 代码环境按声明式组合提交:主运行时 + 镜像版本 + 学生工具 + 基础设施 + 组件参数。
// 镜像地址、digest、启动命令、安全上下文与网络策略都由服务端编译器产出,前端不编辑也不回显为输入
// (docs/对齐-后端待补齐清单-2026-08-23.md §6.3 / §7.5)。
// 兼容性也不在前端算:目录不下发「哪个运行时允许哪些工具」,保存时由服务端编译器判定并回报。
//
// 仿真场景从 M4 已发布仿真包里选,版本按包的版本清单选。
// 组件标识(id)由教师给一个便于识别的短名,后续阶段与检查点引用它 ——
// 这是编排内部的引用键,故要求可读而不是自动生成的编号。

import { useCallback, useMemo, useState } from 'react'
import { Link2, Network, Plus, Server, Trash2 } from 'lucide-react'
import {
  PAGINATION_MAX_SIZE,
  SANDBOX_ACCESS_PROFILE,
  SIM_PACKAGE_STATUS,
  type EnvComponentRequest,
  type SimComponent,
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
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Select,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { CompositionDeclarationFields } from '../../../sandbox/components/CompositionDeclarationFields'
import {
  compositionDeclarationError,
  compositionSpecFromDeclaration,
  declarationFromSpec,
  derivedInfraFromSpec,
} from '../../../sandbox/composition'
import { simCategoryLabel } from '../../../../utils/labels/sim'
import type { ExperimentDraft } from './wizard-state'

export interface WizardComponentsStepProps {
  draft: ExperimentDraft
  errors: Record<string, string | null>
  onChange: (patch: Partial<ExperimentDraft>) => void
}

/**
 * WizardComponentsStep 管理代码环境与仿真场景组件。
 */
export function WizardComponentsStep({ draft, errors, onChange }: WizardComponentsStepProps) {
  const [envModal, setEnvModal] = useState<{ index?: number } | undefined>()
  const [simModal, setSimModal] = useState<{ index?: number } | undefined>()

  const removeEnv = useCallback(
    (index: number) => {
      const env = draft.components.envs[index]
      onChange({
        components: {
          ...draft.components,
          envs: draft.components.envs.filter((_, i) => i !== index),
          // 同步清理阶段与检查点里对该环境的引用,避免留下悬空引用
          stages: draft.components.stages.map((stage) => ({
            ...stage,
            components: {
              ...stage.components,
              envs: stage.components.envs?.filter((id) => id !== env.id),
            },
          })),
          checkpoints: draft.components.checkpoints.map((checkpoint) =>
            checkpoint.env_id === env.id ? { ...checkpoint, env_id: undefined } : checkpoint
          ),
        },
      })
    },
    [draft.components, onChange]
  )

  const removeSim = useCallback(
    (index: number) => {
      const sim = draft.components.sims[index]
      onChange({
        components: {
          ...draft.components,
          sims: draft.components.sims.filter((_, i) => i !== index),
          stages: draft.components.stages.map((stage) => ({
            ...stage,
            components: {
              ...stage.components,
              sims: stage.components.sims?.filter((id) => id !== sim.id),
            },
          })),
          checkpoints: draft.components.checkpoints.map((checkpoint) =>
            checkpoint.sim_id === sim.id ? { ...checkpoint, sim_id: undefined } : checkpoint
          ),
        },
      })
    },
    [draft.components, onChange]
  )

  return (
    <div className="flex flex-col gap-4">
      {errors.components ? <Callout tone="danger">{errors.components}</Callout> : null}

      <Card>
        <CardHeader
          title="代码环境"
          description="学生在实验里使用的沙箱环境。可以配置多个,在阶段里分别启用。"
          actions={
            <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setEnvModal({})}>
              添加环境
            </Button>
          }
        />
        <CardBody>
          {draft.components.envs.length === 0 ? (
            <Empty
              icon={Server}
              title="还没有代码环境"
              description="需要学生写代码或操作链上合约的实验,至少配置一个环境。"
              action={
                <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setEnvModal({})}>
                  添加环境
                </Button>
              }
            />
          ) : (
            <div className="flex flex-col gap-2">
              {draft.components.envs.map((env, index) => (
                <div
                  key={env.id}
                  className="flex flex-wrap items-center justify-between gap-3 well p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-base text-ink">代码环境 {index + 1}</div>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge tone="neutral">
                        {env.primary_runtime.runtime_code} · {env.primary_runtime.image_version}
                      </Badge>
                      {env.tools.map((tool) => (
                        <Badge key={tool.code} tone="jade">
                          {tool.code}
                        </Badge>
                      ))}
                      {env.infra.map((item) => (
                        <Badge key={item.code} tone="info">
                          {item.code}
                        </Badge>
                      ))}
                      {env.links.length > 0 ? (
                        <Badge tone="neutral">{env.links.length} 条组件连接</Badge>
                      ) : null}
                      {env.keep_alive ? <Badge tone="info">保留环境</Badge> : null}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button variant="ghost" size="sm" onClick={() => setEnvModal({ index })}>
                      编辑
                    </Button>
                    <IconButton
                      variant="ghost"
                      size="sm"
                      icon={Trash2}
                      aria-label={`删除第 ${index + 1} 个代码环境`}
                      onClick={() => removeEnv(index)}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      <Card>
        <CardHeader
          title="仿真场景"
          description="需要学生观察算法过程时配置。仿真场景可与代码环境组合使用。"
          actions={
            <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setSimModal({})}>
              添加场景
            </Button>
          }
        />
        <CardBody>
          {draft.components.sims.length === 0 ? (
            <Empty
              icon={Network}
              title="还没有仿真场景"
              description="不需要仿真的实验可以跳过这一项。"
            />
          ) : (
            <div className="flex flex-col gap-2">
              {draft.components.sims.map((sim, index) => (
                <div
                  key={sim.id}
                  className="flex flex-wrap items-center justify-between gap-3 well p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-base text-ink">仿真场景 {index + 1}</div>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge tone="neutral">{sim.package_code}</Badge>
                      <Badge tone="neutral">{sim.version}</Badge>
                      <span className="font-mono text-xs text-ink-sub">随机种子 {sim.seed}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button variant="ghost" size="sm" onClick={() => setSimModal({ index })}>
                      编辑
                    </Button>
                    <IconButton
                      variant="ghost"
                      size="sm"
                      icon={Trash2}
                      aria-label={`删除第 ${index + 1} 个仿真场景`}
                      onClick={() => removeSim(index)}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>

      {envModal ? (
        <EnvFormModal
          env={envModal.index !== undefined ? draft.components.envs[envModal.index] : undefined}
          usedIds={draft.components.envs.map((env) => env.id)}
          onClose={() => setEnvModal(undefined)}
          onSave={(env) => {
            const envs =
              envModal.index !== undefined
                ? draft.components.envs.map((item, i) => (i === envModal.index ? env : item))
                : [...draft.components.envs, env]
            onChange({ components: { ...draft.components, envs } })
            setEnvModal(undefined)
          }}
        />
      ) : null}

      {simModal ? (
        <SimFormModal
          sim={simModal.index !== undefined ? draft.components.sims[simModal.index] : undefined}
          usedIds={draft.components.sims.map((sim) => sim.id)}
          onClose={() => setSimModal(undefined)}
          onSave={(sim) => {
            const sims =
              simModal.index !== undefined
                ? draft.components.sims.map((item, i) => (i === simModal.index ? sim : item))
                : [...draft.components.sims, sim]
            onChange({ components: { ...draft.components, sims } })
            setSimModal(undefined)
          }}
        />
      ) : null}
    </div>
  )
}

interface EnvFormModalProps {
  env?: EnvComponentRequest
  usedIds: string[]
  onClose: () => void
  onSave: (env: EnvComponentRequest) => void
}

/**
 * EnvFormModal 声明一个代码环境。
 * 运行时、镜像版本、工具与基础设施都从 M2 编排目录里选:手填编码会在学生进入时才发现拼错。
 * 连接由服务端编译器按组件声明产出,这里只读回显上次结果,不凭猜测新建。
 */
function EnvFormModal({ env, usedIds, onClose, onSave }: EnvFormModalProps) {
  const editing = env !== undefined
  const [id, setId] = useState(env?.id ?? '')
  const [declaration, setDeclaration] = useState(() => declarationFromSpec(env))
  const [keepAlive, setKeepAlive] = useState(env?.keep_alive ?? false)
  const [keepAliveMinutes, setKeepAliveMinutes] = useState(String(env?.keep_alive_minutes ?? 30))
  const [snapshotEnabled, setSnapshotEnabled] = useState(env?.snapshot_enabled ?? false)
  const [snapshotRetentionMinutes, setSnapshotRetentionMinutes] = useState(
    String(env?.snapshot_retention_minutes ?? 30)
  )
  const [formError, setFormError] = useState<string>()

  const derivedInfra = useMemo(() => derivedInfraFromSpec(env), [env])
  // 连接原样带回,故引用要稳定:否则提交回调每次渲染都重建
  const links = useMemo(() => env?.links ?? [], [env?.links])

  const submit = useCallback(() => {
    const trimmedId = id.trim()
    if (trimmedId === '') {
      setFormError('请给这个环境起一个名称,后面的阶段与检查点会引用它')
      return
    }
    if (!editing && usedIds.includes(trimmedId)) {
      setFormError('这个名称已被其他环境使用,请换一个')
      return
    }
    const declarationError = compositionDeclarationError(declaration)
    if (declarationError !== undefined) {
      setFormError(declarationError)
      return
    }
    const keepAliveDuration = Number(keepAliveMinutes)
    if (keepAlive && (!Number.isInteger(keepAliveDuration) || keepAliveDuration <= 0)) {
      setFormError('保留时长必须填写大于 0 的整数分钟数')
      return
    }
    const snapshotRetention = Number(snapshotRetentionMinutes)
    if (snapshotEnabled && (!Number.isInteger(snapshotRetention) || snapshotRetention <= 0)) {
      setFormError('快照保留时长必须填写大于 0 的整数分钟数')
      return
    }
    setFormError(undefined)
    const spec = compositionSpecFromDeclaration({
      id: trimmedId,
      declaration,
      accessProfile: SANDBOX_ACCESS_PROFILE.EXPERIMENT,
      derivedInfra,
      links,
    })
    onSave({
      id: spec.id,
      primary_runtime: spec.primary_runtime,
      infra: spec.infra ?? [],
      tools: spec.tools ?? [],
      links: spec.links ?? [],
      access_profile: spec.access_profile,
      init_code_ref: env?.init_code_ref,
      init_script_ref: env?.init_script_ref,
      keep_alive: keepAlive,
      keep_alive_minutes: keepAlive ? keepAliveDuration : 0,
      snapshot_enabled: snapshotEnabled,
      snapshot_retention_minutes: snapshotEnabled ? snapshotRetention : 0,
    })
  }, [
    declaration,
    derivedInfra,
    editing,
    env?.init_code_ref,
    env?.init_script_ref,
    id,
    keepAlive,
    keepAliveMinutes,
    links,
    onSave,
    snapshotEnabled,
    snapshotRetentionMinutes,
    usedIds,
  ])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑代码环境' : '添加代码环境'}</ModalTitle>
          <ModalDescription>
            运行时决定学生拿到什么链与工具链;工具决定他们能用终端、网页 IDE 还是命令工具。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField
            label="环境名称"
            htmlFor="env-id"
            required
            helper="用便于识别的短名,例如 main 或 attack;阶段与检查点会按此名称引用环境"
          >
            <Input
              id="env-id"
              value={id}
              disabled={editing}
              onChange={(event) => setId(event.target.value)}
            />
          </FormField>

          <CompositionDeclarationFields
            idPrefix="env"
            value={declaration}
            onChange={setDeclaration}
            toolsHelper="学生在这个环境里能打开哪些工具"
            derivedInfraCodes={derivedInfra.map((item) => item.code)}
          />

          {links.length > 0 ? (
            <div className="flex flex-col gap-2 well p-4">
              <div className="flex items-center gap-2">
                <Link2 className="size-4 text-ink-sub" aria-hidden />
                <h4 className="text-sm font-semibold text-ink">组件连接(只读)</h4>
              </div>
              <p className="text-sm text-ink-sub">
                连接由平台按组件声明编译并冻结,保存时会重新校验。要改动请联系平台管理员调整组件声明。
              </p>
              <DescriptionList
                dense
                items={links.map((link) => ({
                  term: `${link.source_component} → ${link.target_component}`,
                  description: `${link.protocol} · ${link.target_endpoint}`,
                  mono: true,
                }))}
              />
            </div>
          ) : null}

          <div className="flex flex-col gap-3 well p-4">
            <Checkbox
              checked={keepAlive}
              label="学生退出后保留环境一段时间"
              onCheckedChange={(checked) => setKeepAlive(checked === true)}
            />
            {keepAlive ? (
              <FormField
                label="保留时长(分钟)"
                htmlFor="env-keepalive"
                helper="超过后环境会被回收,学生下次进入会重新准备"
              >
                <Input
                  id="env-keepalive"
                  type="number"
                  min="1"
                  value={keepAliveMinutes}
                  onChange={(event) => setKeepAliveMinutes(event.target.value)}
                />
              </FormField>
            ) : null}
            <Checkbox
              checked={snapshotEnabled}
              label="回收前保存工作区快照"
              onCheckedChange={(checked) => setSnapshotEnabled(checked === true)}
            />
            {snapshotEnabled ? (
              <FormField
                label="快照保留时长(分钟)"
                htmlFor="env-snapshot-retention"
                helper="超过后平台会清理快照,避免长期占用学校存储空间"
              >
                <Input
                  id="env-snapshot-retention"
                  type="number"
                  min="1"
                  value={snapshotRetentionMinutes}
                  onChange={(event) => setSnapshotRetentionMinutes(event.target.value)}
                />
              </FormField>
            ) : null}
          </div>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" onClick={submit}>
            {editing ? '保存环境' : '添加环境'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}


interface SimFormModalProps {
  sim?: SimComponent
  usedIds: string[]
  onClose: () => void
  onSave: (sim: SimComponent) => void
}

/**
 * SimFormModal 配置一个仿真场景组件。
 * 随机种子决定推演的可复现性:同一种子每次推演结果一致,便于讲解与批改。
 */
function SimFormModal({ sim, usedIds, onClose, onSave }: SimFormModalProps) {
  const editing = sim !== undefined
  const [id, setId] = useState(sim?.id ?? '')
  const [packageCode, setPackageCode] = useState(sim?.package_code ?? '')
  const [version, setVersion] = useState(sim?.version ?? '')
  const [seed, setSeed] = useState(String(sim?.seed ?? 1))
  const [formError, setFormError] = useState<string>()

  const packages = useAsyncResource(
    () =>
      api.sim.getPackages({
        status: SIM_PACKAGE_STATUS.PUBLISHED,
        page: 1,
        size: PAGINATION_MAX_SIZE,
      }),
    [],
    () => false
  )

  const versions = useAsyncResource(
    () => (packageCode ? api.sim.getPackageVersions(packageCode) : Promise.resolve([])),
    [packageCode],
    () => false
  )

  const packageOptions = useMemo(() => {
    // 同一 code 可能有多个版本行,选择器按 code 去重
    const seen = new Set<string>()
    return (packages.data?.list ?? [])
      .filter((item) => {
        if (seen.has(item.code)) return false
        seen.add(item.code)
        return true
      })
      .map((item) => ({
        value: item.code,
        label: `${item.name} · ${simCategoryLabel(item.category)}`,
      }))
  }, [packages.data])

  const versionOptions = useMemo(
    () =>
      (versions.data ?? [])
        .filter((item) => item.status === SIM_PACKAGE_STATUS.PUBLISHED)
        .map((item) => ({ value: item.version, label: item.version })),
    [versions.data]
  )

  const submit = useCallback(() => {
    const trimmedId = id.trim()
    if (trimmedId === '') {
      setFormError('请给这个场景起一个名称')
      return
    }
    if (!editing && usedIds.includes(trimmedId)) {
      setFormError('这个名称已被其他场景使用,请换一个')
      return
    }
    if (packageCode === '' || version === '') {
      setFormError('请选择仿真场景与版本')
      return
    }
    setFormError(undefined)
    onSave({
      id: trimmedId,
      package_code: packageCode,
      version,
      seed: Number(seed) || 1,
      // 场景初始参数由仿真包自身声明默认值;编排层不构造参数对象,
      // 需要按阶段传参时用阶段的参数绑定(第 3 步)表达
      params: sim?.params ?? {},
    })
  }, [editing, id, onSave, packageCode, seed, sim?.params, usedIds, version])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑仿真场景' : '添加仿真场景'}</ModalTitle>
          <ModalDescription>
            场景按固定随机种子运行,同一种子每次推演结果一致,便于课堂讲解与批改。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField
            label="场景名称"
            htmlFor="sim-id"
            required
            helper="用便于识别的短名;阶段与检查点会按此名称引用场景"
          >
            <Input
              id="sim-id"
              value={id}
              disabled={editing}
              onChange={(event) => setId(event.target.value)}
            />
          </FormField>

          <ResourceState
            resource={packages}
            emptyIcon={Network}
            emptyTitle="平台还没有可用仿真场景"
            emptyDescription="提交仿真包并通过平台审核后可在此选择。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() => (
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField label="仿真场景" htmlFor="sim-package" required>
                  <Select
                    id="sim-package"
                    options={packageOptions}
                    value={packageCode}
                    placeholder={packageOptions.length > 0 ? '选择场景' : '暂无可用场景'}
                    disabled={packageOptions.length === 0}
                    onValueChange={(value) => {
                      setPackageCode(value)
                      setVersion('')
                    }}
                  />
                </FormField>
                <FormField label="场景版本" htmlFor="sim-version" required>
                  <Select
                    id="sim-version"
                    options={versionOptions}
                    value={version}
                    placeholder={
                      packageCode === ''
                        ? '请先选择场景'
                        : versionOptions.length > 0
                          ? '选择版本'
                          : '该场景暂无已发布版本'
                    }
                    disabled={versionOptions.length === 0}
                    onValueChange={setVersion}
                  />
                </FormField>
              </div>
            )}
          </ResourceState>

          <FormField
            label="随机种子"
            htmlFor="sim-seed"
            required
            helper="同一种子保证每个学生看到相同的推演过程"
          >
            <Input
              id="sim-seed"
              type="number"
              min="1"
              value={seed}
              onChange={(event) => setSeed(event.target.value)}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" onClick={submit}>
            {editing ? '保存场景' : '添加场景'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
