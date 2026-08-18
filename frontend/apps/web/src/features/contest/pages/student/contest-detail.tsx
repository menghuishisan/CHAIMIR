// 竞赛详情页(深页,/student/contests/:contestId)。
// 这是竞赛答题与对局回放两条沉浸路由的退出回落页,故必须支持深链与刷新:
// 赛事本体走学生专用单读 GET /contest/student/contests/{id}。
//
// 报名与组队是本页动作区的操作,不单独建页 —— 报名没有跨步骤中间态,
// 独立成页只会多一次跳转(对齐清单 §3.1「竞赛报名」)。
// 结果快照不对学生开放(teacher 组),最终名次经天梯榜与竞赛战绩呈现。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ListOrdered, Play, Swords, Trophy, UserPlus, Users } from 'lucide-react'
import {
  ContestStatus,
  TeamMode,
  type Contest,
  type ContestProblem,
  type ContestRecord,
  type LadderRank,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  Empty,
  FormField,
  Input,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Stat,
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import {
  contestModeLabel,
  contestStatusLabel,
  matchModeLabel,
  teamModeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { isContestLeaderboardFrozen } from '../../rules'
import { contestStatusTone } from '../../statusPresentation'
import { ContestTeamCard } from './contest-team'

/** 允许进入答题的赛事状态:进行中与封榜期(封榜只停榜单更新,不停作答)。 */
const ANSWERABLE_STATUSES: ReadonlySet<ContestStatus> = new Set([
  ContestStatus.RUNNING,
  ContestStatus.FROZEN,
])

/**
 * StudentContestDetailPage 读取赛事与本人参赛记录。
 */
export default function StudentContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

  // 赛事本体与本人战绩一起读:战绩里的 team_id 是学生侧唯一的队伍来源
  const view = useAsyncResource(
    () =>
      Promise.all([
        api.contest.getStudentContest(contestId),
        api.contest.getMyContestRecords(),
      ]).then(([contest, records]) => ({
        contest,
        record: records.find((item: ContestRecord) => item.contest_id === contestId),
      })),
    [contestId],
    () => false,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={Swords}
        emptyTitle="赛事暂不可用"
        emptyDescription="这场赛事可能尚未发布,请回到竞赛参赛查看其他赛事。"
      >
        {(data) => (
          <ContestDetailContent
            contest={data.contest}
            record={data.record}
            onRefresh={view.reload}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

interface ContestDetailContentProps {
  contest: Contest
  record: ContestRecord | undefined
  onRefresh: () => void
}

/**
 * ContestDetailContent 渲染赛事档案、赛题、榜单与参赛动作。
 */
function ContestDetailContent({ contest, record, onRefresh }: ContestDetailContentProps) {
  const navigate = useNavigate()
  const joined = record !== undefined
  const canAnswer = joined && ANSWERABLE_STATUSES.has(contest.status)

  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[{ label: '竞赛参赛', href: '/student/contests' }, { label: contest.name }]}
          />
        }
        title={contest.name}
        description={`${contestModeLabel(contest.mode)} · ${teamModeLabel(contest.team_mode)}`}
        icon={Swords}
        actions={
          <div className="flex items-center gap-2">
            {joined ? <Badge tone="jade">已报名</Badge> : null}
            <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
          </div>
        }
      />

      {/* 指标带只放我的成绩;赛制、对抗形式与封榜时长是赛事静态属性,列在右侧「赛程」卡里 */}
      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2">
          <Stat
            label="我的得分"
            value={record ? formatScore(record.score) : '—'}
            icon={Trophy}
            hint={joined ? '以榜单实时结算为准' : '报名后开始计分'}
          />
          <Stat
            label="我的名次"
            value={record && record.rank > 0 ? record.rank : '—'}
            icon={ListOrdered}
            hint={isContestLeaderboardFrozen(contest.status) ? '封榜中,名次暂停更新' : undefined}
          />
        </div>
      </PageSection>

      <PageBody
        rail={
          <div className="flex flex-col gap-4">
            <ContestActionCard
              contest={contest}
              joined={joined}
              canAnswer={canAnswer}
              onJoined={onRefresh}
              onEnterWorkspace={() => navigate(`/student/contests/${contest.id}/workspace`)}
              onViewRecords={() => navigate('/student/records')}
            />
            {record ? <ContestTeamCard teamId={record.team_id} onChanged={onRefresh} /> : null}
            <ContestScheduleCard contest={contest} />
          </div>
        }
      >
        <Tabs defaultValue="problems">
          <TabsList>
            <TabsTrigger value="problems" icon={Swords}>
              赛题
            </TabsTrigger>
            <TabsTrigger value="ladder" icon={ListOrdered}>
              天梯榜
            </TabsTrigger>
          </TabsList>

          <TabsContent value="problems">
            <ContestProblems contestId={contest.id} canAnswer={canAnswer} />
          </TabsContent>

          <TabsContent value="ladder">
            <ContestLadder contestId={contest.id} frozen={isContestLeaderboardFrozen(contest.status)} />
          </TabsContent>
        </Tabs>
      </PageBody>
    </>
  )
}

interface ContestActionCardProps {
  contest: Contest
  joined: boolean
  canAnswer: boolean
  onJoined: () => void
  onEnterWorkspace: () => void
  onViewRecords: () => void
}

/**
 * ContestActionCard 承载报名、加入队伍与进入答题。
 * 报名窗口关闭后不给可点的报名按钮,而是说明原因 —— 给一个必然失败的按钮更差。
 */
function ContestActionCard({
  contest,
  joined,
  canAnswer,
  onJoined,
  onEnterWorkspace,
  onViewRecords,
}: ContestActionCardProps) {
  const [teamName, setTeamName] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [signupError, setSignupError] = useState<string>()
  const [joinError, setJoinError] = useState<string>()
  const [signingUp, setSigningUp] = useState(false)
  const [joining, setJoining] = useState(false)

  const signupOpen = contest.status === ContestStatus.SIGNUP
  const isSolo = contest.team_mode === TeamMode.SOLO

  /** signup 报名并创建队伍(个人赛由后端统一命名,不要求填队名)。 */
  const signup = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!isSolo && teamName.trim() === '') {
        setSignupError('请输入队伍名称')
        return
      }
      setSignupError(undefined)
      setSigningUp(true)
      try {
        await api.contest.signup(contest.id, { team_name: teamName.trim() })
        toast.success('报名成功')
        onJoined()
      } catch (error) {
        setSignupError(userFacingErrorMessage(error, '报名没有成功,请稍后重试。'))
      } finally {
        setSigningUp(false)
      }
    },
    [contest.id, isSolo, onJoined, teamName],
  )

  /**
   * joinTeam 用邀请码加入已有队伍。
   * 邀请码由队长分享,后端按邀请码定位队伍并校验人数上限。
   */
  const joinTeam = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const code = inviteCode.trim()
      if (!code) {
        setJoinError('请输入队伍邀请码')
        return
      }
      setJoinError(undefined)
      setJoining(true)
      try {
        await api.contest.joinTeam(contest.id, { invite_code: code })
        toast.success('已加入队伍')
        onJoined()
      } catch (error) {
        setJoinError(userFacingErrorMessage(error, '加入队伍没有成功,请确认邀请码后重试。'))
      } finally {
        setJoining(false)
      }
    },
    [contest.id, inviteCode, onJoined],
  )

  if (joined) {
    return (
      <Card>
        <CardHeader title="参赛" description="你已报名这场赛事。" />
        <CardBody className="flex flex-col gap-3">
          {canAnswer ? (
            <Button variant="primary" leftIcon={Play} onClick={onEnterWorkspace}>
              进入答题
            </Button>
          ) : (
            <Callout tone="info">
              {contest.status === ContestStatus.SIGNUP
                ? '比赛还没开始,开赛后可以进入答题。'
                : '比赛已结束,可以在竞赛战绩里回顾成绩。'}
            </Callout>
          )}
          <Button variant="ghost" leftIcon={Trophy} onClick={onViewRecords}>
            查看我的战绩
          </Button>
        </CardBody>
      </Card>
    )
  }

  if (!signupOpen) {
    return (
      <Card>
        <CardHeader title="报名" />
        <CardBody>
          <Callout tone="info">
            {contest.status === ContestStatus.RUNNING || contest.status === ContestStatus.FROZEN
              ? '这场赛事已开赛,报名已关闭。'
              : '报名窗口尚未开放或已结束。'}
          </Callout>
        </CardBody>
      </Card>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader
          title="报名参赛"
          description={isSolo ? '这场赛事个人参赛,报名即完成。' : '创建队伍后把邀请码发给队友。'}
        />
        <CardBody>
          <form onSubmit={signup} noValidate>
            {isSolo ? null : (
              <FormField label="队伍名称" required error={signupError}>
                <Input
                  value={teamName}
                  placeholder="给队伍起个名字"
                  invalid={Boolean(signupError)}
                  onChange={(event) => setTeamName(event.target.value)}
                />
              </FormField>
            )}
            {isSolo && signupError ? <Callout tone="danger">{signupError}</Callout> : null}
            <Button
              type="submit"
              variant="primary"
              leftIcon={UserPlus}
              loading={signingUp}
              className={isSolo ? 'w-full' : 'mt-4 w-full'}
            >
              报名参赛
            </Button>
          </form>
        </CardBody>
      </Card>

      {isSolo ? null : (
        <Card>
          <CardHeader title="加入已有队伍" description="向队长索取邀请码后填入即可加入。" />
          <CardBody>
            <form onSubmit={joinTeam} noValidate>
              <FormField label="队伍邀请码" required error={joinError}>
                <Input
                  value={inviteCode}
                  autoComplete="off"
                  placeholder="请输入邀请码"
                  invalid={Boolean(joinError)}
                  onChange={(event) => setInviteCode(event.target.value)}
                />
              </FormField>
              <Button type="submit" variant="outline" leftIcon={Users} loading={joining} className="mt-4 w-full">
                加入队伍
              </Button>
            </form>
          </CardBody>
        </Card>
      )}
    </div>
  )
}

/**
 * ContestScheduleCard 展示赛程时间与规则要点。
 * 赛制、参赛形式与封榜时长都是赛事的静态属性,统一在这里列出,不占指标位(规范 §6.5)。
 */
function ContestScheduleCard({ contest }: { contest: Contest }) {
  const items = useMemo(
    () => [
      { term: '报名开始', description: formatDateTime(contest.signup_start), mono: true },
      { term: '报名截止', description: formatDateTime(contest.signup_end), mono: true },
      { term: '比赛开始', description: formatDateTime(contest.start_at), mono: true },
      { term: '比赛结束', description: formatDateTime(contest.end_at), mono: true },
      { term: '赛制', description: contestModeLabel(contest.mode) },
      ...(contest.match_mode
        ? [{ term: '对抗形式', description: matchModeLabel(contest.match_mode) }]
        : []),
      { term: '参赛形式', description: teamModeLabel(contest.team_mode) },
      {
        term: '封榜时长',
        description: contest.freeze_minutes > 0 ? `${contest.freeze_minutes} 分钟` : '不封榜',
      },
    ],
    [contest],
  )

  return (
    <Card>
      <CardHeader title="赛程" />
      <CardBody>
        <DescriptionList dense items={items} />
      </CardBody>
    </Card>
  )
}

interface ContestProblemsProps {
  contestId: string
  canAnswer: boolean
}

/**
 * ContestProblems 列出赛题。
 * 题面正文不在列表里展开:答案黑盒要求题面只在答题工作台按需取用,
 * 列表只给分值与序号(后端 face 字段已剥离答案与判题配置)。
 */
function ContestProblems({ contestId, canAnswer }: ContestProblemsProps) {
  const navigate = useNavigate()
  const problems = useAsyncResource(() => api.contest.getProblems(contestId), [contestId])

  const columns: TableColumn<ContestProblem>[] = [
    { key: 'seq', header: '序号', align: 'right', mono: true },
    {
      key: 'title',
      header: '赛题',
      render: (problem) => (
        <span className="font-medium text-ink">
          {typeof problem.face?.title === 'string' ? problem.face.title : `第 ${problem.seq} 题`}
        </span>
      ),
    },
    { key: 'score', header: '分值', align: 'right', mono: true },
    {
      key: 'battle_rule',
      header: '题目类型',
      render: (problem) => (
        <Badge tone="neutral">{problem.battle_rule ? '对抗题' : '解题'}</Badge>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: () => (
        <Button
          variant="ghost"
          size="sm"
          disabled={!canAnswer}
          onClick={() => navigate(`/student/contests/${contestId}/workspace`)}
        >
          去作答
        </Button>
      ),
    },
  ]

  return (
    <PageSection title="赛题" description="进入答题后才能看到完整题面。">
      <ResourceState
        resource={problems}
        emptyIcon={Swords}
        emptyTitle="赛题尚未公布"
        emptyDescription="开赛后赛题会显示在这里。"
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
      >
        {(list) => <Table columns={columns} data={list} rowKey={(item) => item.id} />}
      </ResourceState>
    </PageSection>
  )
}

interface ContestLadderProps {
  contestId: string
  frozen: boolean
}

/**
 * ContestLadder 展示天梯榜。
 * 封榜期明确说明榜单暂停更新,避免用户把冻结的名次当成最终结果。
 */
function ContestLadder({ contestId, frozen }: ContestLadderProps) {
  const ladder = usePagedResource<LadderRank>(
    (params) => api.contest.getLadder(contestId, params),
    [contestId],
  )

  const columns: TableColumn<LadderRank>[] = [
    { key: 'rank', header: '名次', align: 'right', mono: true },
    {
      key: 'team_id',
      header: '队伍',
      // 队伍名不在天梯响应里(只回 team_id),按名次呈现,不把内部编号当队名显示
      render: (rank) => <span className="text-ink">第 {rank.rank} 名队伍</span>,
    },
    {
      key: 'score',
      header: '得分',
      align: 'right',
      mono: true,
      render: (rank) => formatScore(rank.score),
    },
    { key: 'solved_count', header: '通过题数', align: 'right', mono: true },
    {
      key: 'last_solve_at',
      header: '最近通过',
      render: (rank) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {rank.last_solve_at ? formatDateTime(rank.last_solve_at) : '—'}
        </span>
      ),
    },
  ]

  return (
    <PageSection title="天梯榜" description={`共 ${ladder.total} 支队伍上榜`}>
      <div className="flex flex-col gap-4">
        {frozen ? (
          <Callout tone="warning" title="封榜中">
            比赛进入封榜期,榜单暂停更新,当前名次不代表最终结果。
          </Callout>
        ) : null}
        <ResourceState
          resource={ladder}
          emptyIcon={ListOrdered}
          emptyTitle="榜单还是空的"
          emptyDescription="有队伍通过赛题后榜单就会出现。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <>
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.team_id}
                empty={<Empty icon={ListOrdered} title="榜单还是空的" />}
              />
              <Pagination
                page={ladder.page}
                pageSize={ladder.pageSize}
                total={ladder.total}
                onPageChange={ladder.setPage}
              />
            </>
          )}
        </ResourceState>
      </div>
    </PageSection>
  )
}
