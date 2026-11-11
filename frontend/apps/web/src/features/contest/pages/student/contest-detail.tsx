// 竞赛详情页(深页,/student/contests/:contestId)。设计 §7.2:
// 页头为赛事名 + 状态 + 一行属性(赛制 · 参赛形式 · 当前阶段的关键时间),右侧主按钮;下方赛程条。
// 主体左栏「赛题 / 天梯榜」标签(当前标签写进地址;开赛前赛题显示「开赛后公布」,开赛后列题目与分值,
// 点击进入答题);右栏「我的队伍」(未报名时是报名 / 用邀请码加入)与「我的成绩」(报名后显示)。
// 主操作按状态:报名中且未报名 →「报名参赛」;进行中或封榜且已报名 →「进入答题」;已结束 →
// 「查看我的战绩」;对抗赛增加「看对局回放」。不能操作时写原因(如「报名已截止」)。
//
// 这是竞赛答题与对局回放两条沉浸路由的退出回落页,故必须支持深链与刷新:
// 赛事本体走学生专用单读 GET /contest/student/contests/{id}。报名与组队在本页完成,不单独建页。
// 结果快照不对学生开放(teacher 组),最终名次经天梯榜与竞赛战绩呈现。

import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { History, ListOrdered, Play, Swords, Trophy, UserPlus, Users } from 'lucide-react'
import {
  ContestMode,
  ContestStatus,
  TeamMode,
  type Contest,
  type ContestRecord,
  type LadderRank,
} from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  Callout,
  FormField,
  Icon,
  Input,
  ListRow,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  Skeleton,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, usePageLocation, locationLabel, useUrlState } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import { contestModeLabel, contestStatusLabel, matchModeLabel, teamModeLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContestTimeline } from '../../components/ContestTimeline'
import { isContestAnswerable, isContestLeaderboardFrozen } from '../../rules'
import { contestStatusTone } from '../../statusPresentation'
import { ContestTeamCard } from './contest-team'

/** 地址参数:tab 是左栏标签 */
const DETAIL_QUERY = {
  tab: { defaultValue: 'problems', allowed: ['problems', 'ladder'] },
} as const

/**
 * StudentContestDetailPage 读取赛事与本人参赛记录。
 */
export default function StudentContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

  // 赛事本体与本人战绩一起读:战绩里的 team_id 是学生侧唯一的队伍来源
  const view = useAsyncResource(
    () =>
      Promise.all([api.contest.getStudentContest(contestId), api.contest.getMyContestRecords()]).then(
        ([contest, records]) => ({
          contest,
          record: records.find((item: ContestRecord) => item.contest_id === contestId),
        }),
      ),
    [contestId],
    () => false,
  )

  usePageLocation([{ label: locationLabel(view, (data) => data.contest.name, '赛事详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={Swords}
        emptyTitle="赛事暂不可用"
        emptyDescription="这场赛事可能尚未发布,请回到竞赛参赛查看其他赛事。"
        skeleton={
          <div aria-busy="true" className="flex flex-col gap-4">
            <Skeleton variant="title" width="half" />
            <Skeleton variant="line" width="third" />
            <Skeleton variant="block" />
          </div>
        }
      >
        {(data) => <ContestDetailContent contest={data.contest} record={data.record} onRefresh={view.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

/** keyTimeMeta 页头属性里「当前阶段的关键时间」 */
function keyTimeMeta(contest: Contest): { label: string; value: string } {
  switch (contest.status) {
    case ContestStatus.SIGNUP:
      return { label: '报名截止', value: formatDateTime(contest.signup_end) }
    case ContestStatus.RUNNING:
    case ContestStatus.FROZEN:
      return { label: '结束', value: formatDateTime(contest.end_at) }
    default:
      return { label: '已于', value: `${formatDateTime(contest.end_at)} 结束` }
  }
}

interface ContestDetailContentProps {
  contest: Contest
  record: ContestRecord | undefined
  onRefresh: () => void
}

/**
 * ContestDetailContent 渲染页头主操作、赛程条、赛题与天梯榜、队伍与成绩。
 */
function ContestDetailContent({ contest, record, onRefresh }: ContestDetailContentProps) {
  const [query, setQuery] = useUrlState(DETAIL_QUERY)
  const [signupOpen, setSignupOpen] = useState(false)
  const joined = record !== undefined
  const canAnswer = joined && isContestAnswerable(contest.status)
  const frozen = isContestLeaderboardFrozen(contest.status)
  const battle = contest.mode === ContestMode.BATTLE
  const ended = contest.status === ContestStatus.ENDED || contest.status === ContestStatus.ARCHIVED
  const keyTime = keyTimeMeta(contest)

  return (
    <>
      <PageHeader
        title={contest.name}
        status={<StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />}
        meta={[
          { value: contestModeLabel(contest.mode) + (contest.match_mode ? `(${matchModeLabel(contest.match_mode)})` : '') },
          { value: teamModeLabel(contest.team_mode) },
          { label: keyTime.label, value: keyTime.value },
        ]}
        actions={
          <PrimaryContestAction
            contest={contest}
            joined={joined}
            canAnswer={canAnswer}
            ended={ended}
            battle={battle}
            onSignup={() => setSignupOpen(true)}
          />
        }
      />

      <ContestTimeline contest={contest} className="mb-6" />

      {frozen ? (
        <Callout tone="warning" className="mb-6">
          现在处于封榜期,得分与名次暂停更新,结束后一次性揭晓。
        </Callout>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="min-w-0 lg:col-span-2">
          <Tabs value={query.tab} onValueChange={(value) => setQuery({ tab: value })}>
            <TabsList aria-label="赛题与榜单">
              <TabsTrigger value="problems" icon={Swords}>
                赛题
              </TabsTrigger>
              <TabsTrigger value="ladder" icon={ListOrdered}>
                天梯榜
              </TabsTrigger>
            </TabsList>
            <TabsContent value="problems">
              <ContestProblems contest={contest} canAnswer={canAnswer} joined={joined} />
            </TabsContent>
            <TabsContent value="ladder">
              <ContestLadder contestId={contest.id} mode={contest.mode} myTeamId={record?.team_id} />
            </TabsContent>
          </Tabs>
        </div>

        <aside aria-label="我的参赛" className="flex min-w-0 flex-col gap-4">
          {record ? (
            <>
              <MyScoreCard record={record} frozen={frozen} />
              <ContestTeamCard teamId={record.team_id} onChanged={onRefresh} />
            </>
          ) : (
            <NotJoinedCard contest={contest} onSignup={() => setSignupOpen(true)} onJoined={onRefresh} />
          )}
        </aside>
      </div>

      <SignupModal
        open={signupOpen}
        onOpenChange={setSignupOpen}
        contest={contest}
        onJoined={() => {
          setSignupOpen(false)
          onRefresh()
        }}
      />
    </>
  )
}

/** PrimaryContestAction 页头主操作,按赛事状态与本人是否报名决定;不能操作时写原因 */
function PrimaryContestAction({
  contest,
  joined,
  canAnswer,
  ended,
  battle,
  onSignup,
}: {
  contest: Contest
  joined: boolean
  canAnswer: boolean
  ended: boolean
  battle: boolean
  onSignup: () => void
}) {
  const replay = battle && (canAnswer || ended) ? (
    <Link to={`/student/contests/${contest.id}/replay`} className={buttonVariants({ variant: 'outline' })}>
      <Icon icon={History} size="sm" />
      看对局回放
    </Link>
  ) : null

  if (canAnswer) {
    return (
      <div className="flex flex-wrap gap-2">
        {replay}
        <Link to={`/student/contests/${contest.id}/workspace`} className={buttonVariants({ variant: 'primary' })}>
          <Icon icon={Play} size="sm" />
          进入答题
        </Link>
      </div>
    )
  }
  if (ended) {
    return (
      <div className="flex flex-wrap gap-2">
        {replay}
        {joined ? (
          <Link to="/student/records" className={buttonVariants({ variant: 'primary' })}>
            <Icon icon={Trophy} size="sm" />
            查看我的战绩
          </Link>
        ) : null}
      </div>
    )
  }
  if (contest.status === ContestStatus.SIGNUP && !joined) {
    const closed = new Date(contest.signup_end).getTime() < Date.now()
    return closed ? (
      <span className="text-sm text-ink-sub">报名已截止</span>
    ) : (
      <Button variant="primary" leftIcon={UserPlus} onClick={onSignup}>
        报名参赛
      </Button>
    )
  }
  if (contest.status === ContestStatus.SIGNUP && joined) {
    return <span className="text-sm text-ink-sub">已报名,开赛后可以进入答题</span>
  }
  // 进行中但没报名
  return <span className="text-sm text-ink-sub">已开赛,报名已关闭</span>
}

/** MyScoreCard 我的成绩:得分与名次(封榜期注明暂停更新) */
function MyScoreCard({ record, frozen }: { record: ContestRecord; frozen: boolean }) {
  return (
    <section aria-labelledby="contest-score-title" className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="contest-score-title" className="text-sm font-medium text-ink-sub">
        我的成绩
      </h2>
      <dl className="mt-3 grid grid-cols-3 gap-3">
        <div>
          <dt className="text-xs text-ink-sub">得分</dt>
          <dd className="text-xl font-bold tabular-nums text-ink">{formatScore(record.score)}</dd>
        </div>
        <div>
          <dt className="text-xs text-ink-sub">名次</dt>
          <dd className="text-xl font-bold tabular-nums text-ink">{record.rank > 0 ? record.rank : '—'}</dd>
        </div>
        <div>
          <dt className="text-xs text-ink-sub">通过</dt>
          <dd className="text-xl font-bold tabular-nums text-ink">{record.solved_count}</dd>
        </div>
      </dl>
      {frozen ? <p className="mt-2 text-xs text-ink-sub">封榜期间名次暂停更新</p> : null}
    </section>
  )
}

/** NotJoinedCard 未报名时的右栏:报名中给报名与邀请码加入,否则说明报名已关闭 */
function NotJoinedCard({ contest, onSignup, onJoined }: { contest: Contest; onSignup: () => void; onJoined: () => void }) {
  const signupOpen = contest.status === ContestStatus.SIGNUP && new Date(contest.signup_end).getTime() >= Date.now()
  const group = contest.team_mode === TeamMode.GROUP
  return (
    <section aria-labelledby="contest-join-title" className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="contest-join-title" className="text-sm font-medium text-ink-sub">
        我的队伍
      </h2>
      {signupOpen ? (
        <div className="mt-3 flex flex-col gap-4">
          <p className="text-sm text-ink">
            {group ? '还没有队伍。创建队伍后把邀请码发给队友,或用队长给的邀请码加入。' : '这场赛事个人参赛,报名即可。'}
          </p>
          <Button variant="secondary" leftIcon={UserPlus} onClick={onSignup}>
            {group ? '创建队伍并报名' : '报名参赛'}
          </Button>
          {group ? <JoinTeamForm contestId={contest.id} onJoined={onJoined} /> : null}
        </div>
      ) : (
        <p className="mt-3 text-sm text-ink-sub">
          {contest.status === ContestStatus.SIGNUP ? '报名已截止,' : '报名已关闭,'}可以看看赛题与榜单。
        </p>
      )}
    </section>
  )
}

/** JoinTeamForm 用队长分享的邀请码加入队伍(后端按邀请码定位队伍并校验人数上限) */
function JoinTeamForm({ contestId, onJoined }: { contestId: string; onJoined: () => void }) {
  const [inviteCode, setInviteCode] = useState('')
  const [joining, setJoining] = useState(false)
  const [error, setError] = useState<string>()
  const validation = useFormValidation(() => ({ inviteCode: !inviteCode.trim() ? '请输入队伍邀请码' : null }))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validation.validate()) return
      setJoining(true)
      setError(undefined)
      try {
        await api.contest.joinTeam(contestId, { invite_code: inviteCode.trim() })
        toast.success('已加入队伍')
        onJoined()
      } catch (cause) {
        setError(userFacingErrorMessage(cause, '没能加入队伍,请确认邀请码后重试。'))
      } finally {
        setJoining(false)
      }
    },
    [contestId, inviteCode, onJoined, validation],
  )

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-3 border-t border-line pt-4">
      <FormField label="用邀请码加入" htmlFor="contest-team-invite" error={validation.errors.inviteCode}>
        <Input
          id="contest-team-invite"
          value={inviteCode}
          autoComplete="off"
          placeholder="队长给你的邀请码"
          invalid={Boolean(validation.errors.inviteCode)}
          onChange={(event) => setInviteCode(event.target.value)}
          onBlur={() => validation.validate(['inviteCode'])}
        />
      </FormField>
      {error ? <Callout tone="danger">{error}</Callout> : null}
      <Button type="submit" variant="outline" leftIcon={Users} loading={joining}>
        加入队伍
      </Button>
    </form>
  )
}

/** SignupModal 报名弹窗:组队赛填队名(同时创建队伍),个人赛直接确认 */
function SignupModal({
  open,
  onOpenChange,
  contest,
  onJoined,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  contest: Contest
  onJoined: () => void
}) {
  const solo = contest.team_mode === TeamMode.SOLO
  const [teamName, setTeamName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string>()
  const validation = useFormValidation(() => ({ teamName: !solo && !teamName.trim() ? '请输入队伍名称' : null }))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validation.validate()) return
      setSubmitting(true)
      setError(undefined)
      try {
        // 个人赛的队名由后端统一生成,不要求填写
        await api.contest.signup(contest.id, { team_name: teamName.trim() })
        toast.success('报名成功')
        onJoined()
      } catch (cause) {
        setError(userFacingErrorMessage(cause, '报名没有成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [contest.id, onJoined, teamName, validation],
  )

  return (
    <Modal open={open} onOpenChange={(next) => (submitting ? undefined : onOpenChange(next))}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>报名「{contest.name}」</ModalTitle>
          <ModalDescription>
            {solo ? '这场赛事个人参赛,确认后即完成报名。' : '报名会同时创建你的队伍,之后把邀请码发给队友。'}
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          <form id="contest-signup-form" onSubmit={submit} noValidate className="flex flex-col gap-4">
            {solo ? null : (
              <FormField label="队伍名称" htmlFor="contest-team-name" required error={validation.errors.teamName}>
                <Input
                  id="contest-team-name"
                  value={teamName}
                  placeholder="给队伍起个名字"
                  invalid={Boolean(validation.errors.teamName)}
                  onChange={(event) => setTeamName(event.target.value)}
                  onBlur={() => validation.validate(['teamName'])}
                />
              </FormField>
            )}
            {error ? <Callout tone="danger">{error}</Callout> : null}
            <div className="flex justify-end gap-2">
              <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit" variant="primary" loading={submitting}>
                确认报名
              </Button>
            </div>
          </form>
        </ModalBody>
      </ModalContent>
    </Modal>
  )
}

/**
 * ContestProblems 赛题列表:开赛前「开赛后公布」;开赛后每题一行(题号、题名、题型、分值),
 * 已报名且可作答时整行链到答题工作台里的这一题。题面正文只在工作台里按需取用(答案黑盒)。
 */
function ContestProblems({ contest, canAnswer, joined }: { contest: Contest; canAnswer: boolean; joined: boolean }) {
  const started = contest.status !== ContestStatus.SIGNUP && contest.status !== ContestStatus.DRAFT
  const problems = useAsyncResource(() => (started ? api.contest.getProblems(contest.id) : Promise.resolve([])), [contest.id, started])

  if (!started) {
    return (
      <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-12 text-center shadow-xs">
        <Icon icon={Swords} size="lg" className="text-ink-faint" />
        <p className="mt-3 text-md font-semibold text-ink">赛题开赛后公布</p>
        <p className="mt-1 text-sm text-ink-sub">{formatDateTime(contest.start_at)} 开赛</p>
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-lg bg-surface shadow-xs">
      <ResourceState
        resource={problems}
        emptyIcon={Swords}
        emptyTitle="这场赛事还没有赛题"
        emptyDescription="老师添加赛题后会显示在这里。"
        skeleton={
          <div aria-busy="true" className="flex flex-col gap-3 p-4">
            <Skeleton variant="line" lines={3} />
          </div>
        }
      >
        {(list) => (
          <>
            <ul className="divide-y divide-line">
              {[...list]
                .sort((a, b) => a.seq - b.seq)
                .map((problem) => {
                  const title = problem.title || `第 ${problem.seq} 题`
                  const leading = (
                    <span className="grid size-8 place-items-center rounded-md bg-surface-hover text-sm font-semibold tabular-nums text-ink-sub">
                      {problem.seq}
                    </span>
                  )
                  const trailing = <span className="text-base font-semibold tabular-nums text-ink">{problem.score} 分</span>
                  const meta = problem.battle_rule ? '对抗题' : '解题'
                  return (
                    <li key={problem.id}>
                      {canAnswer ? (
                        <ListRowLink
                          href={`/student/contests/${contest.id}/workspace?problem=${problem.id}`}
                          title={title}
                          meta={meta}
                          leading={leading}
                          trailing={trailing}
                        />
                      ) : (
                        <ListRow title={title} meta={meta} leading={leading} trailing={trailing} />
                      )}
                    </li>
                  )
                })}
            </ul>
            {!canAnswer ? (
              <p className="border-t border-line px-4 py-3 text-sm text-ink-sub">
                {joined ? '比赛结束后不能再作答。' : '报名参赛的同学可以进入答题。'}
              </p>
            ) : null}
          </>
        )}
      </ResourceState>
    </div>
  )
}

/** ListRowLink 在路由内跳转的整行链接(保留中键与 Ctrl/⌘ 新标签页打开) */
function ListRowLink(props: { href: string; title: string; meta: string; leading: React.ReactNode; trailing: React.ReactNode }) {
  const navigate = useNavigate()
  return <ListRow {...props} onNavigate={(href: string) => navigate(href)} />
}

/**
 * ContestLadder 天梯榜:名次、得分(解题赛还有通过题数与最近通过时间);我的队伍高亮并标「我的队伍」。
 * 队伍名不在天梯响应里(只回 team_id),不把内部编号当队名显示。
 */
function ContestLadder({ contestId, mode, myTeamId }: { contestId: string; mode: ContestMode; myTeamId: string | undefined }) {
  const ladder = usePagedResource<LadderRank>((params) => api.contest.getLadder(contestId, params), [contestId])
  const solve = mode === ContestMode.SOLVE

  if (ladder.status === 'error') {
    return (
      <div className="rounded-lg bg-surface shadow-xs">
        <ResourceError
          message={ladder.error?.message ?? '榜单暂时读取不到'}
          traceId={ladder.error?.traceId}
          onRetry={ladder.reload}
        />
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-lg bg-surface shadow-xs">
      {ladder.status === 'loading' && !ladder.data ? (
        <div aria-busy="true" className="flex flex-col gap-3 p-4">
          <Skeleton variant="line" lines={5} />
        </div>
      ) : (ladder.data?.list.length ?? 0) === 0 ? (
        <div className="flex flex-col items-center px-6 py-12 text-center">
          <Icon icon={ListOrdered} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">榜单还是空的</p>
          <p className="mt-1 text-sm text-ink-sub">{solve ? '有队伍通过赛题后榜单就会出现。' : '有队伍完成对局后榜单就会出现。'}</p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line text-left text-sm text-ink-sub">
                <th scope="col" className="px-4 py-3 font-normal">名次</th>
                <th scope="col" className="px-4 py-3 font-normal">队伍</th>
                <th scope="col" className="px-4 py-3 text-right font-normal">{solve ? '得分' : '对抗积分'}</th>
                {solve ? <th scope="col" className="px-4 py-3 text-right font-normal">通过题数</th> : null}
                {solve ? <th scope="col" className="hidden px-4 py-3 font-normal sm:table-cell">最近通过</th> : null}
              </tr>
            </thead>
            <tbody>
              {ladder.data?.list.map((rank) => {
                const mine = rank.team_id === myTeamId
                return (
                  <tr key={rank.team_id} className={mine ? 'bg-primary-soft' : 'border-b border-line last:border-b-0'}>
                    <td className="px-4 py-3 font-semibold tabular-nums text-ink">{rank.rank}</td>
                    <td className="px-4 py-3 text-ink">{mine ? <span className="font-semibold text-primary">我的队伍</span> : `第 ${rank.rank} 名`}</td>
                    <td className="px-4 py-3 text-right font-semibold tabular-nums text-ink">{formatScore(rank.score)}</td>
                    {solve ? <td className="px-4 py-3 text-right tabular-nums text-ink">{rank.solved_count}</td> : null}
                    {solve ? (
                      <td className="hidden px-4 py-3 font-mono text-xs tabular-nums text-ink-sub sm:table-cell">
                        {rank.last_solve_at ? formatDateTime(rank.last_solve_at) : '—'}
                      </td>
                    ) : null}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      <div className="border-t border-line px-4 py-3 empty:hidden">
        <Pagination page={ladder.page} pageSize={ladder.pageSize} total={ladder.total} onPageChange={ladder.setPage} />
      </div>
    </div>
  )
}
