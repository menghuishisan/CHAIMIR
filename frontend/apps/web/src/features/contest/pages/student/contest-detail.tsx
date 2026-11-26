// 本人战绩提供队伍编号；完整题面只在答题工作台按需读取。

import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { History, ListOrdered, Play, Swords, Trophy, UserPlus } from 'lucide-react'
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
import { useAsyncResource, usePagedResource, usePageLocation, locationLabel, useUrlState, useVisiblePolling } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import { contestModeLabel, contestStatusLabel, matchModeLabel, teamModeLabel } from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContestTimeline } from '../../components/ContestTimeline'
import { JoinTeamForm } from '../../components/JoinTeamForm'
import { contestSignupState } from '../../contestTimeline'
import { isContestAnswerable, isContestLeaderboardFrozen } from '../../rules'
import { contestStatusTone } from '../../statusPresentation'
import { ContestTeamCard } from './contest-team'
import { useSession } from '../../../../components/RoleGuard/session'

const DETAIL_QUERY = {
  tab: { defaultValue: 'problems', allowed: ['problems', 'ladder'] },
} as const

export default function StudentContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

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

function keyTimeMeta(contest: Contest, upcomingSignup: boolean): { label: string; value: string } {
  switch (contest.status) {
    case ContestStatus.SIGNUP:
      return upcomingSignup
        ? { label: '报名开始', value: formatDateTime(contest.signup_start) }
        : { label: '报名截止', value: formatDateTime(contest.signup_end) }
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

function ContestDetailContent({ contest, record, onRefresh }: ContestDetailContentProps) {
  const [query, setQuery] = useUrlState(DETAIL_QUERY)
  const [signupOpen, setSignupOpen] = useState(false)
  const [now, setNow] = useState(Date.now)
  const signup = contestSignupState(contest, now)
  const signupHint = {
    open: undefined,
    upcoming: '报名尚未开始',
    closed: contest.status === ContestStatus.SIGNUP ? '报名已截止' : '报名已关闭',
  }[signup]
  useVisiblePolling(contest.status === ContestStatus.SIGNUP && signup !== 'closed', () => setNow(Date.now()), 1000)
  const joined = record !== undefined
  const canAnswer = joined && isContestAnswerable(contest.status)
  const frozen = isContestLeaderboardFrozen(contest.status)
  const battle = contest.mode === ContestMode.BATTLE
  const ended = contest.status === ContestStatus.ENDED || contest.status === ContestStatus.ARCHIVED
  const keyTime = keyTimeMeta(contest, signup === 'upcoming')

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
            signupHint={signupHint}
            onSignup={() => setSignupOpen(true)}
          />
        }
      />

      <ContestTimeline contest={contest} now={new Date(now)} className="mb-6" />

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
            <NotJoinedCard contest={contest} signupHint={signupHint} onSignup={() => setSignupOpen(true)} onJoined={onRefresh} />
          )}
        </aside>
      </div>

      <SignupModal
        open={signupOpen}
        onOpenChange={setSignupOpen}
        contest={contest}
        signupHint={signupHint}
        onJoined={() => {
          setSignupOpen(false)
          onRefresh()
        }}
      />
    </>
  )
}

function PrimaryContestAction({
  contest,
  joined,
  canAnswer,
  ended,
  battle,
  signupHint,
  onSignup,
}: {
  contest: Contest
  joined: boolean
  canAnswer: boolean
  ended: boolean
  battle: boolean
  signupHint: string | undefined
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
    return signupHint ? (
      <span className="text-sm text-ink-sub">{signupHint}</span>
    ) : (
      <Button variant="primary" leftIcon={UserPlus} onClick={onSignup}>
        报名参赛
      </Button>
    )
  }
  if (contest.status === ContestStatus.SIGNUP && joined) {
    return <span className="text-sm text-ink-sub">已报名,开赛后可以进入答题</span>
  }
  return <span className="text-sm text-ink-sub">已开赛,报名已关闭</span>
}

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

function NotJoinedCard({ contest, signupHint, onSignup, onJoined }: { contest: Contest; signupHint: string | undefined; onSignup: () => void; onJoined: () => void }) {
  const group = contest.team_mode === TeamMode.GROUP
  return (
    <section aria-labelledby="contest-join-title" className="rounded-lg bg-surface p-5 shadow-xs">
      <h2 id="contest-join-title" className="text-sm font-medium text-ink-sub">
        我的队伍
      </h2>
      {!signupHint ? (
        <div className="mt-3 flex flex-col gap-4">
          <p className="text-sm text-ink">
            {group ? '还没有队伍。创建队伍后把邀请码发给队友,或用队长给的邀请码加入。' : '这场赛事个人参赛,报名即可。'}
          </p>
          <Button variant="secondary" leftIcon={UserPlus} onClick={onSignup}>
            {group ? '创建队伍并报名' : '报名参赛'}
          </Button>
          {group ? (
            <div className="border-t border-line pt-4">
              <JoinTeamForm key={contest.id} contestId={contest.id} onJoined={onJoined} />
            </div>
          ) : null}
        </div>
      ) : (
        <p className="mt-3 text-sm text-ink-sub">
          {signupHint}，可以看看赛程与榜单。
        </p>
      )}
    </section>
  )
}

function SignupModal({
  open,
  onOpenChange,
  contest,
  signupHint,
  onJoined,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  contest: Contest
  signupHint: string | undefined
  onJoined: () => void
}) {
  const solo = contest.team_mode === TeamMode.SOLO
  const { me } = useSession()
  // 个人赛的参赛名称默认填本人姓名,可改成昵称;组队赛由队长起名
  const [teamName, setTeamName] = useState(solo ? me.account.name : '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string>()
  const validation = useFormValidation(() => ({
    teamName: !teamName.trim()
      ? solo
        ? '请输入参赛名称'
        : '请输入队伍名称'
      : [...teamName.trim()].length > 128
        ? '名称不能超过 128 个字'
        : null,
  }))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (signupHint || !validation.validate()) return
      setSubmitting(true)
      setError(undefined)
      try {
        await api.contest.signup(contest.id, { team_name: teamName.trim() })
        toast.success('报名成功')
        onJoined()
      } catch (cause) {
        setError(userFacingErrorMessage(cause, '报名没有成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [contest.id, onJoined, signupHint, teamName, validation],
  )

  return (
    <Modal open={open} onOpenChange={(next) => (submitting ? undefined : onOpenChange(next))}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>报名「{contest.name}」</ModalTitle>
          <ModalDescription>
            {solo ? '这场赛事个人参赛,排行榜上显示你填写的参赛名称。' : '报名会同时创建你的队伍,之后把邀请码发给队友。'}
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          <form id="contest-signup-form" onSubmit={submit} noValidate className="flex flex-col gap-4">
            <FormField
              label={solo ? '参赛名称' : '队伍名称'}
              htmlFor="contest-team-name"
              required
              helper={solo ? '排行榜上显示这个名称,可以用本人姓名或昵称' : undefined}
              error={validation.errors.teamName}
            >
              <Input
                id="contest-team-name"
                value={teamName}
                placeholder={solo ? '填写参赛名称' : '给队伍起个名字'}
                invalid={Boolean(validation.errors.teamName)}
                onChange={(event) => setTeamName(event.target.value)}
                onBlur={() => validation.validate(['teamName'])}
              />
            </FormField>
            {error ? <Callout tone="danger">{error}</Callout> : null}
            {signupHint ? <Callout tone="warning">{signupHint}</Callout> : null}
            <div className="flex justify-end gap-2">
              <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit" variant="primary" loading={submitting} disabled={Boolean(signupHint)}>
                确认报名
              </Button>
            </div>
          </form>
        </ModalBody>
      </ModalContent>
    </Modal>
  )
}

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
                    <td className="px-4 py-3 text-ink">
                      <span className={mine ? 'font-semibold text-primary' : undefined}>{rank.team_name}</span>
                      {mine ? <span className="ml-2 text-xs text-primary">我的队伍</span> : null}
                    </td>
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
