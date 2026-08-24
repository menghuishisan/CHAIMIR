// 竞赛出题(赛事详情页内区块)。
//
// 赛题 = M5 题目引用 + 赛内分值 + 动态分规则 + (对抗赛)对抗配置。
// 题目从题库已发布内容里选并锁版本,不让教师手填题目编号与版本号。
//
// 动态分与对抗配置使用前后端共用的固定结构,不给裸 JSON 文本域。
//
// 答案黑盒:题面正文与判题配置不在本页展开,只呈现标题、分值与类型 ——
// 教师编排需要的是"选哪道题、值多少分",不需要在这里读答案。

import { useCallback, useMemo, useState } from 'react'
import { Plus, Swords, Target } from 'lucide-react'
import {
  ContestMode,
  ContentStatus,
  PAGINATION_MAX_SIZE,
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
  Select,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { toContentItemVersionOptions } from '../../../content/contentItemOptions'
import { battleRoleLabel, battleRuleLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { BATTLE_RULES } from '../../options'
import { battleEntryRoles, battleRuntimeConfig } from '../../rules'

export interface ContestProblemsProps {
  contest: Contest
}

/**
 * ContestProblems 管理赛题清单。
 * 后端按 (赛事, 题目, 版本) 唯一约束做 upsert,故"编辑"与"新增"是同一个提交动作。
 */
export function ContestProblems({ contest }: ContestProblemsProps) {
  const [modal, setModal] = useState<{ problem?: ContestProblem } | undefined>()

  const problems = useAsyncResource(
    () => api.contest.getProblems(contest.id),
    [contest.id],
    (value) => value.length === 0
  )

  const isBattle = contest.mode === ContestMode.BATTLE
  const totalScore = (problems.data ?? []).reduce((sum, item) => sum + item.score, 0)

  const columns: TableColumn<ContestProblem>[] = [
    { key: 'seq', header: '序号', align: 'right', mono: true },
    {
      key: 'title',
      header: '赛题',
      render: (problem) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">
            {typeof problem.face?.title === 'string' ? problem.face.title : `第 ${problem.seq} 题`}
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
      render: (problem) => (
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
          <Button variant="primary" leftIcon={Plus} onClick={() => setModal({})}>
            添加赛题
          </Button>
        </div>
      }
    >
      {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):与同父页的防作弊标签同构;
          赛题一次回齐,不分页也不筛选,故只用片本身 */}
      <DataPanel label="赛题">
        <ResourceState
          resource={problems}
          emptyIcon={Swords}
          emptyTitle="还没有赛题"
          emptyDescription="发布赛事前至少需要一道赛题,否则学生报名后无题可做。"
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setModal({})}>
              添加赛题
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(list) => (
            <Table
              columns={columns}
              data={list}
              rowKey={(item) => item.id}
              elevated={false}
              onRowClick={(item) => setModal({ problem: item })}
              // <md 换行卡(§6.4.1 规则 3):题名一行、题源与分值一行,题目类型在右
              mobileCard={(item) => ({
                title:
                  typeof item.face?.title === 'string' ? item.face.title : `第 ${item.seq} 题`,
                meta: `第 ${item.seq} 题 · ${item.score} 分 · ${item.item_code}`,
                badge: item.battle_rule ? (
                  <Badge tone="cinnabar">{battleRuleLabel(item.battle_rule)}</Badge>
                ) : (
                  <Badge tone="neutral">解题</Badge>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>

      {modal ? (
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

/**
 * ProblemFormModal 配置单道赛题。
 * 对抗题的执行环境来自题库锁定版本的组合声明,竞赛侧只决定赛制:
 * 规则、入场角色与回放方式 —— 环境参数不在这里重复一遍,也就不会与题目版本对不上。
 */
function ProblemFormModal({ contest, problem, nextSeq, onClose, onSaved }: ProblemFormModalProps) {
  const editing = problem !== undefined
  const isBattle = contest.mode === ContestMode.BATTLE

  const [itemRef, setItemRef] = useState(
    problem ? `${problem.item_code}|${problem.item_version}` : ''
  )
  const [score, setScore] = useState(String(problem?.score ?? 100))
  const [seq, setSeq] = useState(String(problem?.seq ?? nextSeq))
  const [dynamicEnabled, setDynamicEnabled] = useState(
    (problem?.dynamic_score?.decay_per_solve ?? 0) > 0
  )
  const [minScore, setMinScore] = useState(String(problem?.dynamic_score?.min_score ?? 60))
  const [decay, setDecay] = useState(String(problem?.dynamic_score?.decay_per_solve ?? 5))
  const [battleRule, setBattleRule] = useState(String(problem?.battle_rule ?? BATTLE_RULES[0]))
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  // 题目只列已发布的:草稿题目在学生答题时取不到题面
  const items = useAsyncResource(
    () =>
      api.content.getItems({
        status: ContentStatus.PUBLISHED,
        page: 1,
        size: PAGINATION_MAX_SIZE,
      }),
    [],
    () => false
  )

  const itemOptions = useMemo(
    () => toContentItemVersionOptions(items.data?.list ?? []),
    [items.data]
  )

  const entryRoles = useMemo(
    () => battleEntryRoles(Number(battleRule) as BattleRule),
    [battleRule]
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (itemRef === '') {
        setFormError('请选择这道赛题引用的题目')
        return
      }
      const scoreValue = Number(score)
      if (!Number.isInteger(scoreValue) || scoreValue <= 0) {
        setFormError('分值需要是大于 0 的整数')
        return
      }
      const seqValue = Number(seq)
      if (!Number.isInteger(seqValue) || seqValue <= 0) {
        setFormError('序号需要是大于 0 的整数')
        return
      }
      if (dynamicEnabled) {
        const minValue = Number(minScore)
        const decayValue = Number(decay)
        if (!Number.isInteger(minValue) || minValue <= 0 || minValue > scoreValue) {
          setFormError('最低分需要是正整数且不超过题目分值')
          return
        }
        if (!Number.isInteger(decayValue) || decayValue <= 0) {
          setFormError('每队衰减需要是大于 0 的整数')
          return
        }
      }

      const [itemCode, itemVersion] = itemRef.split('|')
      const rule = Number(battleRule) as BattleRule
      const payload: ContestProblemRequest = {
        item_code: itemCode,
        item_version: itemVersion,
        score: scoreValue,
        seq: seqValue,
        // 动态分与对抗配置按结构化字段组装,不接受用户手写 JSON
        dynamic_score: dynamicEnabled
          ? { min_score: Number(minScore), decay_per_solve: Number(decay) }
          : undefined,
        battle_rule: isBattle ? rule : undefined,
        battle_config: isBattle ? battleRuntimeConfig(rule) : undefined,
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
      dynamicEnabled,
      editing,
      isBattle,
      itemRef,
      minScore,
      onSaved,
      score,
      seq,
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
            <ResourceState
              resource={items}
              emptyIcon={Target}
              emptyTitle="题库里还没有已发布的题目"
              emptyDescription="先在题库内容里创建并发布题目,再回来编排赛题。"
              skeleton={<Skeleton variant="line" lines={2} />}
            >
              {() => (
                <FormField
                  label="引用题目"
                  htmlFor="problem-item"
                  required
                  helper="题目按当前版本锁定;换题请新增赛题,已有提交仍绑定原题版本"
                >
                  <Select
                    id="problem-item"
                    options={itemOptions}
                    value={itemRef}
                    placeholder={itemOptions.length > 0 ? '选择题目' : '暂无已发布题目'}
                    disabled={itemOptions.length === 0 || editing}
                    onValueChange={setItemRef}
                  />
                </FormField>
              )}
            </ResourceState>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="分值" htmlFor="problem-score" required>
                <Input
                  id="problem-score"
                  type="number"
                  min="1"
                  step="1"
                  value={score}
                  onChange={(event) => setScore(event.target.value)}
                />
              </FormField>
              <FormField
                label="序号"
                htmlFor="problem-seq"
                helper="决定赛题在学生答题界面的排列顺序"
              >
                <Input
                  id="problem-seq"
                  type="number"
                  min="1"
                  step="1"
                  value={seq}
                  onChange={(event) => setSeq(event.target.value)}
                />
              </FormField>
            </div>

            <div className="flex flex-col gap-3 well p-4">
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
                    helper="每有一支队伍通过,后续通过者的得分下降这么多"
                  >
                    <Input
                      id="problem-decay"
                      type="number"
                      min="1"
                      step="1"
                      value={decay}
                      onChange={(event) => setDecay(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="最低分"
                    htmlFor="problem-min-score"
                    required
                    helper="衰减到这个分数后不再下降"
                  >
                    <Input
                      id="problem-min-score"
                      type="number"
                      min="1"
                      step="1"
                      value={minScore}
                      onChange={(event) => setMinScore(event.target.value)}
                    />
                  </FormField>
                </div>
              ) : (
                <p className="text-sm text-ink-sub">不开启则所有通过的队伍都得满分。</p>
              )}
            </div>

            {isBattle ? (
              <div className="flex flex-col gap-4 well p-4">
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

                {/* 入场角色与回放方式由规则决定,故只呈现结果不给重复输入(改规则即改这两项) */}
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
