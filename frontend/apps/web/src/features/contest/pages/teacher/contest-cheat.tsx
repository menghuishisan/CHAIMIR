// 防作弊审查(赛事详情页内区块)。
//
// 两件事:按赛题查 M3 查重线索、对确认违规的队伍下处理记录。
// 查重线索需要一个"待比对的提交"作为输入 —— 后端 FindSimilarity 以 code_storage_key
// 为基准生成特征向量,故本页按 code_hash 或来源标识指定基准,不臆造扫描全表的能力。
//
// 处理动作有三种后果不同的档位(警告 / 扣分 / 取消资格),扣分与取消资格会改动榜单,
// 故各自明确说明后果,不做成一个笼统的"处理"按钮。

import { useCallback, useMemo, useState } from 'react'
import { Search, ShieldAlert, Trophy } from 'lucide-react'
import {
  CheatAction,
  CheatType,
  PAGINATION_MAX_SIZE,
  type CheatRecord,
  type CheatSuspect,
  type Contest,
  type ContestProblem,
  type LadderRank,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Empty,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  DataPanel,
  PageBody,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import {
  cheatActionLabel,
  cheatTypeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CHEAT_ACTIONS, CHEAT_TYPES } from '../../options'
import { readNumber, readString } from '../../jsonReaders'
import { cheatActionTone } from '../../statusPresentation'

/** 违规证据里的结构化键:后端在 action=扣分 时读取 penalty_score。 */
const EVIDENCE_PENALTY_SCORE = 'penalty_score'
const EVIDENCE_NOTE = 'note'
const EVIDENCE_SOURCE_REF = 'source_ref'

/** 处理动作的后果说明:扣分与取消资格会改动榜单,必须让教师知道。 */
const ACTION_HINTS: Record<CheatAction, string> = {
  [CheatAction.WARN]: '只留记录,不影响得分与名次。',
  [CheatAction.PENALTY]: '从该队总分里扣除指定分数,榜单随即重算。',
  [CheatAction.DISQUALIFY]: '该队退出排名,榜单随即重算。这个动作影响最大,请确认证据充分。',
}

export interface ContestCheatProps {
  contest: Contest
}

/**
 * ContestCheat 承载查重线索与违规处理。
 */
export function ContestCheat({ contest }: ContestCheatProps) {
  const [recordTarget, setRecordTarget] = useState<{ sourceRef?: string } | undefined>()

  const records = usePagedResource<CheatRecord>(
    (params) => api.contest.listCheatRecords(contest.id, params),
    [contest.id],
  )

  const columns: TableColumn<CheatRecord>[] = [
    {
      key: 'created_at',
      header: '处理时间',
      render: (record) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(record.created_at)}
        </span>
      ),
    },
    {
      key: 'type',
      header: '违规类型',
      render: (record) => <Badge tone="neutral">{cheatTypeLabel(record.type)}</Badge>,
    },
    {
      key: 'action',
      header: '处理方式',
      render: (record) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone={cheatActionTone(record.action)}>{cheatActionLabel(record.action)}</Badge>
          {record.action === CheatAction.PENALTY ? (
            <span className="font-mono text-xs tabular-nums text-ink-sub">
              -{formatScore(readNumber(record.evidence, EVIDENCE_PENALTY_SCORE))} 分
            </span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'evidence',
      header: '判定依据',
      render: (record) => {
        const note = readString(record.evidence, EVIDENCE_NOTE)
        return note ? (
          <span className="line-clamp-2 text-sm text-ink-sub">{note}</span>
        ) : (
          <span className="text-ink-sub">未填写说明</span>
        )
      },
    },
  ]

  return (
    <PageBody rail={<SuspectsCard contest={contest} onHandle={(sourceRef) => setRecordTarget({ sourceRef })} />}>
      <PageSection
        title="违规处理记录"
        description="扣分与取消资格会立即影响榜单。"
        actions={
          <Button variant="primary" leftIcon={ShieldAlert} onClick={() => setRecordTarget({})}>
            登记违规
          </Button>
        }
      >
        {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):数据表与分页同处一块抬起片 */}
        <DataPanel
          label="违规处理记录"
          footer={
            <Pagination
              page={records.page}
              pageSize={records.pageSize}
              total={records.total}
              onPageChange={records.setPage}
            />
          }
        >
          <ResourceState
            resource={records}
            emptyIcon={ShieldAlert}
            emptyTitle="还没有违规记录"
            emptyDescription="确认队伍违规后在这里登记处理。记录会留档并计入审计。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):处理时间一行、违规类型与依据一行,处理方式在右
                mobileCard={(item) => ({
                  title: formatDateTime(item.created_at),
                  meta: `${cheatTypeLabel(item.type)} · ${readString(item.evidence, EVIDENCE_NOTE) || '未填写说明'}`,
                  badge: (
                    <Badge tone={cheatActionTone(item.action)}>{cheatActionLabel(item.action)}</Badge>
                  ),
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
      </PageSection>

      {recordTarget ? (
        <CheatRecordModal
          contest={contest}
          sourceRef={recordTarget.sourceRef}
          onClose={() => setRecordTarget(undefined)}
          onSaved={() => {
            setRecordTarget(undefined)
            records.reload()
          }}
        />
      ) : null}
    </PageBody>
  )
}

interface SuspectsCardProps {
  contest: Contest
  onHandle: (sourceRef: string) => void
}

/**
 * SuspectsCard 按赛题查询代码相似线索。
 * 查重以某一份提交为基准比对同题其他提交,故需要先给出基准提交的代码指纹。
 */
function SuspectsCard({ contest, onHandle }: SuspectsCardProps) {
  const [problemId, setProblemId] = useState<string>('')
  const [codeHash, setCodeHash] = useState('')
  const [threshold, setThreshold] = useState('0.8')
  const [suspects, setSuspects] = useState<CheatSuspect[]>()
  const [searchError, setSearchError] = useState<string>()
  const [searching, setSearching] = useState(false)

  const problems = useAsyncResource(
    () => api.contest.getProblems(contest.id),
    [contest.id],
    () => false,
  )

  const problemOptions = useMemo(
    () =>
      (problems.data ?? []).map((problem: ContestProblem) => ({
        value: problem.id,
        label:
          typeof problem.face?.title === 'string'
            ? `第 ${problem.seq} 题 · ${problem.face.title}`
            : `第 ${problem.seq} 题`,
      })),
    [problems.data],
  )

  const search = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (problemId === '') {
        setSearchError('请选择要查重的赛题')
        return
      }
      const thresholdValue = Number(threshold)
      if (!Number.isFinite(thresholdValue) || thresholdValue <= 0 || thresholdValue >= 1) {
        setSearchError('相似度阈值需要在 0 与 1 之间,例如 0.8')
        return
      }
      setSearchError(undefined)
      setSearching(true)
      try {
        const result = await api.contest.listCheatSuspects(contest.id, {
          problem_id: problemId,
          code_hash: codeHash.trim() || undefined,
          threshold: thresholdValue,
        })
        setSuspects(result)
      } catch (error) {
        setSearchError(userFacingErrorMessage(error, '查重没有完成,请稍后重试。'))
      } finally {
        setSearching(false)
      }
    },
    [codeHash, contest.id, problemId, threshold],
  )

  return (
    <Card>
      <CardHeader
        title="代码查重"
        description="按赛题比对提交代码的相似度,超过阈值的会列出来供人工判断。"
      />
      <CardBody className="flex flex-col gap-4">
        <ResourceState
          resource={problems}
          emptyIcon={Search}
          emptyTitle="这场赛事还没有赛题"
          emptyDescription="先编排赛题,有提交后才能查重。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {() => (
            <form onSubmit={search} noValidate className="flex flex-col gap-4">
              <FormField label="赛题" htmlFor="cheat-problem" required>
                <Select
                  id="cheat-problem"
                  options={problemOptions}
                  value={problemId}
                  placeholder="选择赛题"
                  onValueChange={setProblemId}
                />
              </FormField>

              <FormField
                label="基准代码指纹"
                htmlFor="cheat-hash"
                helper="填入某次提交的代码指纹作为比对基准;留空则按赛题的默认基准比对"
              >
                <Input
                  id="cheat-hash"
                  value={codeHash}
                  autoComplete="off"
                  placeholder="提交详情里的代码指纹"
                  onChange={(event) => setCodeHash(event.target.value)}
                />
              </FormField>

              <FormField
                label="相似度阈值"
                htmlFor="cheat-threshold"
                required
                helper="0 到 1 之间。0.8 表示相似度达到 80% 才列出"
              >
                <Input
                  id="cheat-threshold"
                  type="number"
                  min="0.1"
                  max="0.99"
                  step="0.05"
                  value={threshold}
                  onChange={(event) => setThreshold(event.target.value)}
                />
              </FormField>

              {searchError ? <Callout tone="danger">{searchError}</Callout> : null}

              <Button type="submit" variant="outline" leftIcon={Search} loading={searching} className="w-full">
                开始查重
              </Button>
            </form>
          )}
        </ResourceState>

        {suspects === undefined ? null : suspects.length === 0 ? (
          <Empty
            icon={Search}
            title="没有超过阈值的相似提交"
            description="可以降低阈值再看,或换一道赛题。"
          />
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-sm text-ink-sub">相似度从高到低,人工确认后再登记违规。</p>
            {[...suspects]
              .sort((a, b) => b.score - a.score)
              .map((suspect) => (
                <div
                  key={suspect.source_ref}
                  className="flex flex-col gap-2 well p-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <Badge tone={suspect.score >= 0.95 ? 'danger' : 'warning'}>
                      相似度 {Math.round(suspect.score * 100)}%
                    </Badge>
                    {suspect.code_hash ? (
                      <span className="truncate font-mono text-xs text-ink-faint">
                        {suspect.code_hash.slice(0, 12)}
                      </span>
                    ) : null}
                  </div>
                  <Button variant="ghost" size="sm" onClick={() => onHandle(suspect.source_ref)}>
                    据此登记违规
                  </Button>
                </div>
              ))}
          </div>
        )}
      </CardBody>
    </Card>
  )
}

interface CheatRecordModalProps {
  contest: Contest
  sourceRef?: string
  onClose: () => void
  onSaved: () => void
}

/**
 * CheatRecordModal 登记一条违规处理。
 * 队伍从榜单里选(榜单是本赛事唯一对教师可见的队伍来源),不让教师手填队伍编号。
 */
function CheatRecordModal({ contest, sourceRef, onClose, onSaved }: CheatRecordModalProps) {
  const [teamId, setTeamId] = useState<string>('')
  const [type, setType] = useState(String(CHEAT_TYPES[0]))
  const [action, setAction] = useState(String(CHEAT_ACTIONS[0]))
  const [penaltyScore, setPenaltyScore] = useState('10')
  const [note, setNote] = useState('')
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const ladder = useAsyncResource(
    () => api.contest.getLadder(contest.id, { page: 1, size: PAGINATION_MAX_SIZE }),
    [contest.id],
    () => false,
  )

  const teamOptions = useMemo(
    () =>
      (ladder.data?.list ?? []).map((rank: LadderRank) => ({
        value: rank.team_id,
        // 榜单只回 team_id,按名次与得分标识队伍,不把内部编号当队名显示
        label: `第 ${rank.rank} 名 · ${formatScore(rank.score)} 分 · 通过 ${rank.solved_count} 题`,
      })),
    [ladder.data],
  )

  const actionValue = Number(action) as CheatAction

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (teamId === '') {
        setFormError('请选择要处理的队伍')
        return
      }
      if (note.trim() === '') {
        setFormError('请写下判定依据,处理记录会留档并计入审计')
        return
      }
      const penalty = Number(penaltyScore)
      if (actionValue === CheatAction.PENALTY && (!Number.isFinite(penalty) || penalty <= 0)) {
        setFormError('扣分需要是大于 0 的数字')
        return
      }
      setFormError(undefined)
      setSubmitting(true)
      try {
        await api.contest.createCheatRecord(contest.id, {
          team_id: teamId,
          type: Number(type) as CheatType,
          action: actionValue,
          // 证据按结构化键组装:后端在扣分时读 penalty_score
          evidence: {
            [EVIDENCE_NOTE]: note.trim(),
            ...(actionValue === CheatAction.PENALTY ? { [EVIDENCE_PENALTY_SCORE]: penalty } : {}),
            ...(sourceRef ? { [EVIDENCE_SOURCE_REF]: sourceRef } : {}),
          },
        })
        toast.success('违规处理已登记')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '登记没有成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [actionValue, contest.id, note, onSaved, penaltyScore, sourceRef, teamId, type],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>登记违规处理</ModalTitle>
          <ModalDescription>
            处理结果会记入审计并影响榜单。请先确认证据充分,再选择处理方式。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <ResourceState
              resource={ladder}
              emptyIcon={Trophy}
              emptyTitle="榜单里还没有队伍"
              emptyDescription="有队伍报名并产生成绩后才能登记违规。"
              skeleton={<Skeleton variant="line" lines={2} />}
            >
              {() => (
                <FormField
                  label="队伍"
                  htmlFor="cheat-team"
                  required
                  helper="按榜单名次识别队伍;队伍名不在榜单响应里"
                >
                  <Select
                    id="cheat-team"
                    options={teamOptions}
                    value={teamId}
                    placeholder={teamOptions.length > 0 ? '选择队伍' : '暂无上榜队伍'}
                    disabled={teamOptions.length === 0}
                    onValueChange={setTeamId}
                  />
                </FormField>
              )}
            </ResourceState>

            <FormField label="违规类型" required>
              <SegmentedControl
                aria-label="违规类型"
                options={CHEAT_TYPES.map((item) => ({
                  value: String(item),
                  label: cheatTypeLabel(item),
                }))}
                value={type}
                onValueChange={setType}
              />
            </FormField>

            <FormField label="处理方式" required helper={ACTION_HINTS[actionValue]}>
              <SegmentedControl
                aria-label="处理方式"
                options={CHEAT_ACTIONS.map((item) => ({
                  value: String(item),
                  label: cheatActionLabel(item),
                }))}
                value={action}
                onValueChange={setAction}
              />
            </FormField>

            {actionValue === CheatAction.PENALTY ? (
              <FormField label="扣除分数" htmlFor="cheat-penalty" required>
                <Input
                  id="cheat-penalty"
                  type="number"
                  min="1"
                  value={penaltyScore}
                  onChange={(event) => setPenaltyScore(event.target.value)}
                />
              </FormField>
            ) : null}

            <FormField
              label="判定依据"
              htmlFor="cheat-note"
              required
              helper="写清楚判定理由与证据来源,这条记录会留档"
            >
              <Textarea
                id="cheat-note"
                value={note}
                rows={4}
                onChange={(event) => setNote(event.target.value)}
              />
            </FormField>

            {sourceRef ? (
              <Callout tone="info">这条记录会关联你刚才查重命中的那次提交。</Callout>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button
              type="submit"
              variant={actionValue === CheatAction.DISQUALIFY ? 'danger' : 'seal'}
              loading={submitting}
            >
              确认登记
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** readNumber 从证据对象里读数字字段;非数字回 0(不把对象塞进界面)。 */
