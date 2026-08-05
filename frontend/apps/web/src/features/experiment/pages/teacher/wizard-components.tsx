// 实验编排向导:环境与仿真步(第 2 步)。
//
// 代码环境从 M2 编排目录里选运行时与工具(不让教师手填 runtime_code);
// 仿真场景从 M4 已发布仿真包里选,版本按包的版本清单选。
// 组件标识(id)由教师给一个便于识别的短名,后续阶段与检查点引用它 ——
// 这是编排内部的引用键,故要求可读而不是自动生成的编号。

import { useCallback, useMemo, useState } from 'react'
import { Network, Plus, Server, Trash2 } from 'lucide-react'
import { SIM_PACKAGE_STATUS } from '@chaimir/api-client'
import type { EnvComponent, SimComponent } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
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
import { useOrchestrationCatalog } from '../../../sandbox/useOrchestrationCatalog'
import { sandboxToolKindLabel } from '../../../../utils/labels/sandbox'
import { simCategoryLabel } from '../../../../utils/labels/sim'
import type { ExperimentDraft } from './wizard-state'

/** 仿真包选择器一次取回的条数:后端分页上限 100。 */
const SIM_PICKER_SIZE = 100

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
            checkpoint.env_id === env.id ? { ...checkpoint, env_id: undefined } : checkpoint,
          ),
        },
      })
    },
    [draft.components, onChange],
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
            checkpoint.sim_id === sim.id ? { ...checkpoint, sim_id: undefined } : checkpoint,
          ),
        },
      })
    },
    [draft.components, onChange],
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
                  className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-line p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-base text-ink">{env.id}</div>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge tone="neutral">{env.runtime_code}</Badge>
                      {env.tools.map((tool) => (
                        <Badge key={tool} tone="jade">
                          {tool}
                        </Badge>
                      ))}
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
                      aria-label={`删除环境 ${env.id}`}
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
                  className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-line p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-base text-ink">{sim.id}</div>
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
                      aria-label={`删除场景 ${sim.id}`}
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
  env?: EnvComponent
  usedIds: string[]
  onClose: () => void
  onSave: (env: EnvComponent) => void
}

/**
 * EnvFormModal 配置一个代码环境。
 * 运行时与工具都从 M2 已注册清单里选:手填运行时代码会在学生进入时才发现拼错。
 */
function EnvFormModal({ env, usedIds, onClose, onSave }: EnvFormModalProps) {
  const editing = env !== undefined
  const [id, setId] = useState(env?.id ?? '')
  const [runtimeCode, setRuntimeCode] = useState(env?.runtime_code ?? '')
  const [imageVersion, setImageVersion] = useState(env?.runtime_image_version ?? '')
  const [tools, setTools] = useState<string[]>(env?.tools ?? [])
  const [keepAlive, setKeepAlive] = useState(env?.keep_alive ?? false)
  const [keepAliveMinutes, setKeepAliveMinutes] = useState(String(env?.keep_alive_minutes ?? 30))
  const [snapshotEnabled, setSnapshotEnabled] = useState(env?.snapshot_enabled ?? false)
  const [formError, setFormError] = useState<string>()

  const catalog = useOrchestrationCatalog()
  const imageOptions = useMemo(() => catalog.imageOptions(runtimeCode), [catalog, runtimeCode])

  const submit = useCallback(() => {
    const trimmedId = id.trim()
    if (trimmedId === '') {
      setFormError('请给这个环境起一个标识,后面的阶段与检查点会引用它')
      return
    }
    if (!editing && usedIds.includes(trimmedId)) {
      setFormError('这个标识已被其他环境使用,请换一个')
      return
    }
    if (runtimeCode === '') {
      setFormError('请选择运行时')
      return
    }
    setFormError(undefined)
    onSave({
      id: trimmedId,
      runtime_code: runtimeCode,
      runtime_image_version: imageVersion === '' ? undefined : imageVersion,
      tools,
      keep_alive: keepAlive,
      keep_alive_minutes: keepAlive ? Number(keepAliveMinutes) || undefined : undefined,
      snapshot_enabled: snapshotEnabled,
      init_code_ref: env?.init_code_ref,
      init_script_ref: env?.init_script_ref,
      snapshot_retention_minutes: env?.snapshot_retention_minutes,
    })
  }, [
    editing,
    env?.init_code_ref,
    env?.init_script_ref,
    env?.snapshot_retention_minutes,
    id,
    imageVersion,
    keepAlive,
    keepAliveMinutes,
    onSave,
    runtimeCode,
    snapshotEnabled,
    tools,
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
            label="环境标识"
            htmlFor="env-id"
            required
            helper="用便于识别的短名,例如 main 或 attack。阶段与检查点按这个名字引用环境"
          >
            <Input
              id="env-id"
              value={id}
              disabled={editing}
              onChange={(event) => setId(event.target.value)}
            />
          </FormField>

          <ResourceState
            resource={catalog.resource}
            emptyIcon={Server}
            emptyTitle="平台还没有可用运行时"
            emptyDescription="请联系平台管理员在链运行时里注册并自检运行时。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() => (
              <div className="flex flex-col gap-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField label="运行时" htmlFor="env-runtime" required>
                    <Select
                      id="env-runtime"
                      options={catalog.runtimeOptions}
                      value={runtimeCode}
                      placeholder="选择运行时"
                      onValueChange={(value) => {
                        setRuntimeCode(value)
                        setImageVersion('')
                      }}
                    />
                  </FormField>
                  <FormField
                    label="镜像版本"
                    htmlFor="env-image"
                    helper="不选则用运行时的默认镜像"
                  >
                    <Select
                      id="env-image"
                      options={imageOptions}
                      value={imageVersion}
                      placeholder={
                        runtimeCode === ''
                          ? '请先选择运行时'
                          : imageOptions.length > 0
                            ? '使用默认镜像'
                            : '该运行时暂无镜像'
                      }
                      disabled={imageOptions.length === 0}
                      onValueChange={setImageVersion}
                    />
                  </FormField>
                </div>

                <FormField label="可用工具" helper="学生在这个环境里能打开哪些工具">
                  {catalog.tools.length > 0 ? (
                    <div className="flex flex-col gap-2">
                      {catalog.tools.map((tool) => (
                        <Checkbox
                          key={tool.code}
                          checked={tools.includes(tool.code)}
                          label={`${tool.name} · ${sandboxToolKindLabel(tool.kind)}`}
                          onCheckedChange={(checked) =>
                            setTools((current) =>
                              checked === true
                                ? [...current, tool.code]
                                : current.filter((code) => code !== tool.code),
                            )
                          }
                        />
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-ink-sub">
                      平台还没有可用工具,请联系平台管理员在沙箱工具里注册。
                    </p>
                  )}
                </FormField>
              </div>
            )}
          </ResourceState>

          <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
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
          </div>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="seal" onClick={submit}>
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
    () => api.sim.getPackages({ status: SIM_PACKAGE_STATUS.PUBLISHED, page: 1, size: SIM_PICKER_SIZE }),
    [],
    () => false,
  )

  const versions = useAsyncResource(
    () => (packageCode ? api.sim.getPackageVersions(packageCode) : Promise.resolve([])),
    [packageCode],
    () => false,
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
      .map((item) => ({ value: item.code, label: `${item.name} · ${simCategoryLabel(item.category)}` }))
  }, [packages.data])

  const versionOptions = useMemo(
    () =>
      (versions.data ?? [])
        .filter((item) => item.status === SIM_PACKAGE_STATUS.PUBLISHED)
        .map((item) => ({ value: item.version, label: item.version })),
    [versions.data],
  )

  const submit = useCallback(() => {
    const trimmedId = id.trim()
    if (trimmedId === '') {
      setFormError('请给这个场景起一个标识')
      return
    }
    if (!editing && usedIds.includes(trimmedId)) {
      setFormError('这个标识已被其他场景使用,请换一个')
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
            label="场景标识"
            htmlFor="sim-id"
            required
            helper="用便于识别的短名,阶段与检查点按这个名字引用场景"
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
          <Button variant="seal" onClick={submit}>
            {editing ? '保存场景' : '添加场景'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
