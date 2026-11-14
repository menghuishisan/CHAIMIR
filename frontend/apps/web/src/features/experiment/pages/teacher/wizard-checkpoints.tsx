// 实验编排向导:检查点步(第 4 步)。
//
// 检查点是实验的判分点:每个检查点绑定一个判题方式(M3 判题器目录)与一道题目(M5 锁版本),
// 两者都从已注册清单里选,不让教师手填代码或题目编号。
// 检查点可以绑定到某个环境或场景 —— 判题时按该组件的产出判分。

import { useCallback, useMemo, useState } from 'react'
import { Plus, Target, Trash2 } from 'lucide-react'
import type { CheckpointConfig, JudgerCatalog } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
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
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { type AsyncResourceState, useAsyncResource } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { judgerTypeLabel } from '../../../../utils/labels/judge'
import { ContentItemSelect } from '../../../content/components/ContentItemSelect'
import type { ExperimentDraft } from './wizard-state'

export interface WizardCheckpointsStepProps {
  draft: ExperimentDraft
  errors: Record<string, string | null>
  onChange: (patch: Partial<ExperimentDraft>) => void
}

/**
 * WizardCheckpointsStep 管理检查点配置。
 */
export function WizardCheckpointsStep({ draft, errors, onChange }: WizardCheckpointsStepProps) {
  const [modal, setModal] = useState<{ index?: number } | undefined>()
  // 判题器目录在这一步读一次:表格把判题器编码换成名称,表单弹窗用它列选项
  const judgers = useAsyncResource(
    () => api.judge.getJudgerCatalog(),
    [],
    (value) => value.judgers.length === 0
  )
  const judgerNames = useMemo(
    () => new Map((judgers.data?.judgers ?? []).map((item) => [item.code, item.name])),
    [judgers.data]
  )

  /** judgerText 表格里判题方式的说法:目录里的名称;目录没读到时先占位,目录里没有的说明已经下线 */
  const judgerText = (code: string): string => {
    const name = judgerNames.get(code)
    if (name) return name
    if (judgers.status === 'loading') return '…'
    return judgers.status === 'error' ? '暂时读不到' : '已下线的判题方式'
  }

  const writeCheckpoints = useCallback(
    (checkpoints: CheckpointConfig[]) => {
      onChange({ components: { ...draft.components, checkpoints } })
    },
    [draft.components, onChange],
  )

  const totalScore = draft.components.checkpoints.reduce((sum, item) => sum + item.score, 0)

  const columns: TableColumn<CheckpointConfig>[] = [
    {
      key: 'id',
      header: '检查点',
      render: (checkpoint) => (
        <span className="font-medium text-ink">
          检查点 {draft.components.checkpoints.indexOf(checkpoint) + 1}
        </span>
      ),
    },
    {
      key: 'judger',
      header: '判题方式',
      render: (checkpoint) => <Badge tone="neutral">{judgerText(checkpoint.judger)}</Badge>,
    },
    {
      key: 'item_code',
      header: '题目',
      render: (checkpoint) => (
        <span className="font-mono text-xs text-ink-sub">
          {checkpoint.item_code} · {checkpoint.item_version}
        </span>
      ),
    },
    {
      key: 'bind',
      header: '绑定组件',
      render: (checkpoint) => (
        <div className="flex flex-wrap gap-1.5">
          {checkpoint.env_id ? <Badge tone="neutral">已绑定代码环境</Badge> : null}
          {checkpoint.sim_id ? <Badge tone="neutral">已绑定仿真场景</Badge> : null}
          {!checkpoint.env_id && !checkpoint.sim_id ? (
            <span className="text-ink-sub">不限</span>
          ) : null}
        </div>
      ),
    },
    { key: 'score', header: '分值', align: 'right', mono: true },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (checkpoint) => {
        const index = draft.components.checkpoints.indexOf(checkpoint)
        return (
          <div className="flex justify-end gap-1">
            <Button variant="ghost" size="sm" onClick={() => setModal({ index })}>
              编辑
            </Button>
            <IconButton
              variant="ghost"
              size="sm"
              icon={Trash2}
              aria-label={`删除检查点 ${index + 1}`}
              onClick={() =>
                writeCheckpoints(draft.components.checkpoints.filter((_, i) => i !== index))
              }
            />
          </div>
        )
      },
    },
  ]

  return (
    <div className="flex flex-col gap-4">
      {errors.checkpoints ? <Callout tone="danger">{errors.checkpoints}</Callout> : null}

      <Card>
        <CardHeader
          title="检查点"
          description="学生在实验里逐个通过检查点。每个检查点单独判分,合计即实验得分。"
          actions={
            <div className="flex items-center gap-2">
              <Badge tone={totalScore === 100 ? 'success' : 'neutral'}>合计 {totalScore} 分</Badge>
              <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setModal({})}>
                添加检查点
              </Button>
            </div>
          }
        />
        <CardBody>
          {draft.components.checkpoints.length === 0 ? (
            <Empty
              icon={Target}
              title="还没有检查点"
              description="没有检查点的实验不产生分数,只能按实验报告批改。需要自动判分就添加检查点。"
              action={
                <Button variant="outline" size="sm" leftIcon={Plus} onClick={() => setModal({})}>
                  添加检查点
                </Button>
              }
            />
          ) : (
            <Table
              columns={columns}
              data={draft.components.checkpoints}
              rowKey={(checkpoint) => checkpoint.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):检查点序号一行、判题方式与题目一行,分值在右
              mobileCard={(checkpoint) => {
                const index = draft.components.checkpoints.indexOf(checkpoint)
                return {
                  title: `检查点 ${index + 1}`,
                  meta: `${judgerText(checkpoint.judger)} · ${checkpoint.item_code} · ${checkpoint.item_version}`,
                  badge: <Badge tone="neutral">{checkpoint.score} 分</Badge>,
                  action: (
                    <div className="flex gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setModal({ index })}>
                        编辑
                      </Button>
                      <IconButton
                        variant="ghost"
                        size="sm"
                        icon={Trash2}
                        aria-label={`删除检查点 ${index + 1}`}
                        onClick={() =>
                          writeCheckpoints(
                            draft.components.checkpoints.filter((_, i) => i !== index),
                          )
                        }
                      />
                    </div>
                  ),
                }
              }}
            />
          )}
        </CardBody>
      </Card>

      {totalScore !== 100 && draft.components.checkpoints.length > 0 ? (
        <Callout tone="info" title="分值合计不是 100">
          发布前校验会提示这一点。如果实验还有报告分,合计不足 100 是正常的。
        </Callout>
      ) : null}

      {modal ? (
        <CheckpointFormModal
          checkpoint={
            modal.index !== undefined ? draft.components.checkpoints[modal.index] : undefined
          }
          usedIds={draft.components.checkpoints.map((item) => item.id)}
          judgers={judgers}
          draft={draft}
          onClose={() => setModal(undefined)}
          onSave={(checkpoint) => {
            const checkpoints =
              modal.index !== undefined
                ? draft.components.checkpoints.map((item, i) =>
                    i === modal.index ? checkpoint : item,
                  )
                : [...draft.components.checkpoints, checkpoint]
            writeCheckpoints(checkpoints)
            setModal(undefined)
          }}
        />
      ) : null}
    </div>
  )
}

interface CheckpointFormModalProps {
  checkpoint?: CheckpointConfig
  usedIds: string[]
  /** 步级读到的判题器目录 */
  judgers: AsyncResourceState<JudgerCatalog>
  draft: ExperimentDraft
  onClose: () => void
  onSave: (checkpoint: CheckpointConfig) => void
}

/**
 * CheckpointFormModal 配置单个检查点。
 */
function CheckpointFormModal({
  checkpoint,
  usedIds,
  draft,
  judgers,
  onClose,
  onSave,
}: CheckpointFormModalProps) {
  const editing = checkpoint !== undefined
  const [id, setId] = useState(checkpoint?.id ?? '')
  const [judger, setJudger] = useState(checkpoint?.judger ?? '')
  const [itemCode, setItemCode] = useState(checkpoint?.item_code ?? '')
  const [itemVersion, setItemVersion] = useState(checkpoint?.item_version ?? '')
  const [score, setScore] = useState(String(checkpoint?.score ?? 20))
  const [bindTarget, setBindTarget] = useState(
    checkpoint?.env_id ? `env:${checkpoint.env_id}` : checkpoint?.sim_id ? `sim:${checkpoint.sim_id}` : '',
  )

  const judgerOptions = useMemo(
    () =>
      (judgers.data?.judgers ?? []).map((item) => ({
        value: item.code,
        label: `${item.name} · ${judgerTypeLabel(item.type)}`,
      })),
    [judgers.data],
  )


  const bindOptions = useMemo(
    () => [
      { value: '', label: '不绑定(按整体产出判分)' },
      ...draft.components.envs.map((env, index) => ({ value: `env:${env.id}`, label: `代码环境 ${index + 1}` })),
      ...draft.components.sims.map((sim, index) => ({ value: `sim:${sim.id}`, label: `仿真场景 ${index + 1}` })),
    ],
    [draft.components.envs, draft.components.sims],
  )

  const { errors, validate } = useFormValidation(() => ({
    id: id.trim() === '' ? '请给这个检查点起一个名称,阶段解锁与参数传递会引用它'
      : !editing && usedIds.includes(id.trim()) ? '这个名称已被其他检查点使用,请换一个' : null,
    judger: judger === '' ? '请选择判题方式' : null,
    item: itemCode === '' || itemVersion === '' ? '请选择判分依据的题目' : null,
    score: score.trim() === '' || !Number.isFinite(Number(score)) || Number(score) <= 0 ? '分值需要是大于 0 的数字' : null,
  }))

  const submit = useCallback(() => {
    if (!validate()) return
    const scoreValue = Number(score)

    const [bindKind, bindId] = bindTarget === '' ? ['', ''] : bindTarget.split(':')
    onSave({
      id: id.trim(),
      judger,
      item_code: itemCode,
      item_version: itemVersion,
      score: scoreValue,
      env_id: bindKind === 'env' ? bindId : undefined,
      sim_id: bindKind === 'sim' ? bindId : undefined,
      // 判题额外输入由判题器自身的题目配置提供;编排层不构造判题参数,
      // 避免把判题细节搬到教师界面(答案黑盒:判题配置对教师界面也不展开)
      extra_input: checkpoint?.extra_input,
      mode: checkpoint?.mode,
    })
  }, [
    bindTarget,
    checkpoint?.extra_input,
    checkpoint?.mode,
    id,
    itemCode,
    itemVersion,
    judger,
    onSave,
    score,
    validate,
  ])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑检查点' : '添加检查点'}</ModalTitle>
          <ModalDescription>
            判题方式决定怎么判(跑测试、查链上状态、比对答案口令),题目决定判什么。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField
            label="检查点名称"
            htmlFor="checkpoint-id"
            required
            helper="用便于识别的短名,例如 deploy 或 exploit;阶段解锁与参数传递会引用它"
            error={errors.id}
          >
            <Input
              id="checkpoint-id"
              value={id}
              invalid={Boolean(errors.id)}
              onBlur={() => validate(['id'])}
              disabled={editing}
              onChange={(event) => setId(event.target.value)}
            />
          </FormField>

          <ResourceState
            resource={judgers}
            emptyIcon={Target}
            emptyTitle="平台还没有可用判题器"
            emptyDescription="请联系平台管理员在判题器里注册并自检。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() => (
              <FormField label="判题方式" htmlFor="checkpoint-judger" required error={errors.judger}>
                <Select
                  id="checkpoint-judger"
                  options={judgerOptions}
                  value={judger}
                  invalid={Boolean(errors.judger)}
                  onBlur={() => validate(['judger'])}
                  placeholder="选择判题方式"
                  onValueChange={setJudger}
                />
              </FormField>
            )}
          </ResourceState>

          <FormField
            label="判分依据题目"
            htmlFor="checkpoint-item"
            required
            helper="题目按当前版本锁定,之后题库改动不影响已发布实验"
            error={errors.item}
          >
            <ContentItemSelect
              id="checkpoint-item"
              label="判分依据题目"
              value={itemCode === '' ? undefined : { code: itemCode, version: itemVersion }}
              invalid={Boolean(errors.item)}
              onChange={(item) => {
                setItemCode(item.code)
                setItemVersion(item.version)
              }}
            />
          </FormField>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="分值" htmlFor="checkpoint-score" required error={errors.score}>
              <Input
                id="checkpoint-score"
                type="number"
                min="1"
                value={score}
                invalid={Boolean(errors.score)}
                onBlur={() => validate(['score'])}
                onChange={(event) => setScore(event.target.value)}
              />
            </FormField>
            <FormField
              label="绑定组件"
              htmlFor="checkpoint-bind"
              helper="判分时看哪个环境或场景的产出"
            >
              <Select
                id="checkpoint-bind"
                options={bindOptions}
                value={bindTarget}
                onValueChange={setBindTarget}
              />
            </FormField>
          </div>

        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" onClick={submit}>
            {editing ? '保存检查点' : '添加检查点'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
