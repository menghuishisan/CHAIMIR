import { useCallback, useState } from 'react'
import { Plus, Swords } from 'lucide-react'
import {
  ContestMode,
  ContentType,
  type BattleRule,
  type Contest,
  type ContestProblem,
  type ContestProblemRequest,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
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
  PageSection,
  SegmentedControl,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import type { AsyncResourceState } from '../../../../hooks/useAsyncResource'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { ContentItemSelect } from '../../../content/components/ContentItemSelect'
import { battleRoleLabel, battleRuleLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { BATTLE_RULES } from '../../options'
import { battleEntryRoles } from '../../rules'

export interface ContestProblemsProps {
  contest: Contest
  problems: AsyncResourceState<ContestProblem[]>
  readOnly?: boolean
}

export function ContestProblems({ contest, problems, readOnly = false }: ContestProblemsProps) {
  const [modal, setModal] = useState<{ problem?: ContestProblem } | undefined>()

  const isBattle = contest.mode === ContestMode.BATTLE
  const totalScore = (problems.data ?? []).reduce((sum, item) => sum + item.score, 0)

  const columns: TableColumn<ContestProblem>[] = [
    { key: 'seq', header: '序号', align: 'right', mono: true, width: '88px' },
    {
      key: 'title',
      header: '赛题',
      render: (problem) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">
            {problem.title || `第 ${problem.seq} 题`}
          </div>
          <div className="truncate font-mono text-xs text-ink-sub">
            {problem.item_code} · {problem.item_version}
          </div>
        </div>
      ),
    },
    { key: 'score', header: '分值', align: 'right', mono: true },
    {
      key: 'dynamic_score',
      header: '动态分',
      render: (problem) => {
        const decay = problem.dynamic_score?.decay_per_solve ?? 0
        if (decay <= 0) return <span className="text-ink-sub">固定分值</span>
        return (
          <span className="text-sm text-ink-sub">
            每通过一队降 {decay} 分,最低 {problem.dynamic_score?.min_score ?? problem.score} 分
          </span>
        )
      },
    },
    {
      key: 'battle_rule',
      header: '题目类型',
      render: (problem) =>
        problem.battle_rule ? (
          <Badge tone="cinnabar">{battleRuleLabel(problem.battle_rule)}</Badge>
        ) : (
          <Badge tone="neutral">解题</Badge>
        ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (problem) =>
        readOnly ? (
          <Badge tone="neutral">只读</Badge>
        ) : (
          <Button variant="ghost" size="sm" onClick={() => setModal({ problem })}>
            编辑
          </Button>
        ),
    },
  ]

  return (
    <PageSection
      title="赛题"
      description={
        isBattle
          ? '对抗赛的每道赛题都要指定对抗规则与执行环境,学生提交的是参战程序。'
          : '解题赛的赛题按分值累计排名。动态分可以让先解出的队伍得分更高。'
      }
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="neutral">合计 {totalScore} 分</Badge>
          {!readOnly ? (
            <Button variant="primary" leftIcon={Plus} onClick={() => setModal({})}>
              添加赛题
            </Button>
          ) : null}
        </div>
      }
    >
      <DataPanel label="赛题">
        <ResourceState
          resource={problems}
          emptyIcon={Swords}
          emptyTitle="还没有赛题"
          emptyDescription="发布赛事前至少需要一道赛题,否则学生报名后无题可做。"
          emptyAction={
            readOnly ? undefined : (
              <Button variant="primary" leftIcon={Plus} onClick={() => setModal({})}>
                添加赛题
              </Button>
            )
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
              onRowClick={readOnly ? undefined : (item) => setModal({ problem: item })}
              mobileCard={(item) => ({
                title: item.title || `第 ${item.seq} 题`,
                meta: `第 ${item.seq} 题 · ${item.score} 分 · ${item.item_code}`,
                badge: item.battle_rule ? (
                  <Badge tone="cinnabar">{battleRuleLabel(item.battle_rule)}</Badge>
                ) : (
                  <Badge tone="neutral">解题</Badge>
                ),
                action: readOnly ? undefined : (
                  <Button variant="ghost" size="sm" onClick={() => setModal({ problem: item })}>
                    编辑
                  </Button>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>

      {modal && !readOnly ? (
        <ProblemFormModal
          contest={contest}
          problem={modal.problem}
          nextSeq={(problems.data ?? []).length + 1}
          onClose={() => setModal(undefined)}
          onSaved={() => {
            setModal(undefined)
            problems.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface ProblemFormModalProps {
  contest: Contest
  problem?: ContestProblem
  nextSeq: number
  onClose: () => void
  onSaved: () => void
}

function ProblemFormModal({ contest, problem, nextSeq, onClose, onSaved }: ProblemFormModalProps) {
  const editing = problem !== undefined
  const isBattle = contest.mode === ContestMode.BATTLE

  const [itemRef, setItemRef] = useState(
    problem ? `${problem.item_code}|${problem.item_version}` : ''
  )
  const [score, setScore] = useState(String(problem?.score ?? 100))
  const [seq, setSeq] = useState(String(problem?.seq ?? nextSeq))
  const [dynamicEnabled, setDynamicEnabled] = useState((problem?.dynamic_score?.decay_per_solve ?? 0) > 0)
  const [minScore, setMinScore] = useState(String(problem?.dynamic_score?.min_score ?? 60))
  const [decay, setDecay] = useState(String(problem?.dynamic_score?.decay_per_solve ?? 5))
  const [battleRule, setBattleRule] = useState(String(problem?.battle_rule ?? BATTLE_RULES[0]))
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const useDynamicScore = !isBattle && dynamicEnabled
  const { errors, validate } = useFormValidation(() => ({
    item: itemRef === '' ? '请选择这道赛题引用的题目' : null,
    score: Number.isInteger(Number(score)) && Number(score) > 0
      ? null : '分值需要是大于 0 的整数',
    seq: Number.isInteger(Number(seq)) && Number(seq) > 0
      ? null : '序号需要是大于 0 的整数',
    minScore: useDynamicScore && (
      !Number.isInteger(Number(minScore)) || Number(minScore) <= 0 || Number(minScore) > Number(score)
    ) ? '最低分需要是正整数且不超过题目分值' : null,
    decay: useDynamicScore && (!Number.isInteger(Number(decay)) || Number(decay) <= 0)
      ? '每队衰减需要是大于 0 的整数' : null,
  }))

  // 引用题目:对抗赛只能引用竞赛题,其他赛制可再引用理论题(ContentItemSelect 只列已发布的题)
  const itemTypes = isBattle
    ? [ContentType.CONTEST_PROBLEM]
    : [ContentType.CONTEST_PROBLEM, ContentType.THEORY_QUESTION]

  const entryRoles = battleEntryRoles(Number(battleRule) as BattleRule)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (submitting || !validate()) return
      const scoreValue = Number(score)
      const seqValue = Number(seq)

      const [itemCode, itemVersion] = itemRef.split('|')
      const rule = Number(battleRule) as BattleRule
      const payload: ContestProblemRequest = {
        item_code: itemCode,
        item_version: itemVersion,
        score: scoreValue,
        seq: seqValue,
        dynamic_score: useDynamicScore
          ? { min_score: Number(minScore), decay_per_solve: Number(decay) }
          : undefined,
        battle_rule: isBattle ? rule : undefined,
      }

      setFormError(undefined)
      setSubmitting(true)
      try {
        await api.contest.addProblem(contest.id, payload)
        toast.success(editing ? '赛题已更新' : '赛题已添加')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '赛题保存没有成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [
      battleRule,
      contest.id,
      decay,
      useDynamicScore,
      editing,
      isBattle,
      itemRef,
      minScore,
      onSaved,
      score,
      seq,
      submitting,
      validate,
    ]
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑赛题' : '添加赛题'}</ModalTitle>
          <ModalDescription>
            {isBattle
              ? '对抗题的题目提供背景与判定标准,执行环境决定参战程序在什么链与工具链上运行。'
              : '题目从题库里选并锁定版本,之后题库改动不影响已开赛的赛题。'}
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="引用题目"
              htmlFor="problem-item"
              required
              error={errors.item}
              helper="题目按当前版本锁定;换题请新增赛题,已有提交仍绑定原题版本"
            >
              <ContentItemSelect
                id="problem-item"
                label="引用题目"
                value={itemRef === '' ? undefined : { code: itemRef.split('|')[0], version: itemRef.split('|')[1] }}
                types={itemTypes}
                invalid={Boolean(errors.item)}
                disabled={editing}
                onChange={(item) => setItemRef(`${item.code}|${item.version}`)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="分值" htmlFor="problem-score" required error={errors.score}>
                <Input
                  id="problem-score"
                  type="number"
                  min="1"
                  step="1"
                  value={score}
                  invalid={Boolean(errors.score)}
                  onBlur={() => validate(['score'])}
                  onChange={(event) => setScore(event.target.value)}
                />
              </FormField>
              <FormField
                label="序号"
                htmlFor="problem-seq"
                error={errors.seq}
                helper="决定赛题在学生答题界面的排列顺序"
              >
                <Input
                  id="problem-seq"
                  type="number"
                  min="1"
                  step="1"
                  value={seq}
                  invalid={Boolean(errors.seq)}
                  onBlur={() => validate(['seq'])}
                  onChange={(event) => setSeq(event.target.value)}
                />
              </FormField>
            </div>

            {!isBattle ? (
              <div className="flex flex-col gap-3 rounded-md bg-surface-hover p-4">
                <Checkbox
                  checked={dynamicEnabled}
                  label="按通过队数递减分值(动态分)"
                  onCheckedChange={(checked) => setDynamicEnabled(checked === true)}
                />
                {dynamicEnabled ? (
                  <div className="grid gap-4 sm:grid-cols-2">
                    <FormField
                      label="每队衰减"
                      htmlFor="problem-decay"
                      required
                      error={errors.decay}
                      helper="每有一支队伍通过,后续通过者的得分下降这么多"
                    >
                      <Input
                        id="problem-decay"
                        type="number"
                        min="1"
                        step="1"
                        value={decay}
                        invalid={Boolean(errors.decay)}
                        onBlur={() => validate(['decay'])}
                        onChange={(event) => setDecay(event.target.value)}
                      />
                    </FormField>
                    <FormField
                      label="最低分"
                      htmlFor="problem-min-score"
                      required
                      error={errors.minScore}
                      helper="衰减到这个分数后不再下降"
                    >
                      <Input
                        id="problem-min-score"
                        type="number"
                        min="1"
                        step="1"
                        value={minScore}
                        invalid={Boolean(errors.minScore)}
                        onBlur={() => validate(['minScore'])}
                        onChange={(event) => setMinScore(event.target.value)}
                      />
                    </FormField>
                  </div>
                ) : (
                  <p className="text-sm text-ink-sub">不开启则所有通过的队伍都得满分。</p>
                )}
              </div>
            ) : null}

            {isBattle ? (
              <div className="flex flex-col gap-4 rounded-md bg-surface-hover p-4">
                <FormField label="对抗规则" required helper="决定这道题怎么判胜负">
                  <SegmentedControl
                    aria-label="对抗规则"
                    options={BATTLE_RULES.map((rule) => ({
                      value: String(rule),
                      label: battleRuleLabel(rule),
                    }))}
                    value={battleRule}
                    onValueChange={setBattleRule}
                  />
                </FormField>

                <DescriptionList
                  dense
                  items={[
                    {
                      term: '可参战角色',
                      description: entryRoles.map(battleRoleLabel).join('、'),
                    },
                    { term: '对局回放', description: '完整记录链上交易,赛后可回放复盘' },
                    {
                      term: '执行环境',
                      description: '取自题目锁定版本里的环境声明,赛题不另外指定',
                    },
                  ]}
                />
                <Callout tone="info">
                  对抗题的链与工具在题库里随题目版本一起锁定。要换环境请改题目版本,不要在赛题里另配一套。
                </Callout>
              </div>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存赛题' : '添加赛题'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
