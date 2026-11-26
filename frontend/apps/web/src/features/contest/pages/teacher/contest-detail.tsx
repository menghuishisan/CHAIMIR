import { useState } from 'react'
import { useParams } from 'react-router'
import { Archive, MoreVertical, Pencil, ShieldAlert, Swords, Trophy } from 'lucide-react'
import { ContestMode, ContestStatus, type Contest, type ResultSnapshot } from '@chaimir/api-client'
import {
  Badge,
  Callout,
  DataPanel,
  Empty,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageHeader,
  PageScaffold,
  PageSection,
  StatusIndicator,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePageLocation, locationLabel, useUrlState } from '../../../../hooks'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import {
  contestModeLabel,
  contestStatusLabel,
  matchModeLabel,
  teamModeLabel,
} from '../../../../utils/labels/contest'
import { isContestLeaderboardFrozen } from '../../rules'
import { contestStatusTone } from '../../statusPresentation'
import { ContestFormModal } from '../../components/ContestFormModal'
import { ContestNextStepButton } from '../../components/ContestNextStep'
import { ContestTimeline } from '../../components/ContestTimeline'
import { ContestProblems } from './contest-problems'
import { ContestCheat } from './contest-cheat'

const DETAIL_QUERY = {
  tab: { defaultValue: 'problems', allowed: ['problems', 'cheat', 'snapshot'] },
} as const

export default function TeacherContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

  const contest = useAsyncResource(
    () => api.contest.getContest(contestId),
    [contestId],
    () => false
  )

  usePageLocation([{ label: locationLabel(contest, (data) => data.name, '赛事详情') }])

  return (
    <PageScaffold>
      <ResourceState
        resource={contest}
        emptyIcon={Trophy}
        emptyTitle="赛事不存在"
        emptyDescription="这场赛事可能已被删除,请回到赛事组织查看。"
      >
        {(data) => <ContestDetailContent contest={data} onRefresh={contest.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface ContestDetailContentProps {
  contest: Contest
  onRefresh: () => void
}

function ContestDetailContent({ contest, onRefresh }: ContestDetailContentProps) {
  const [query, setQuery] = useUrlState(DETAIL_QUERY)
  const [editOpen, setEditOpen] = useState(false)
  const problems = useAsyncResource(() => api.contest.getProblems(contest.id), [contest.id])
  const problemCount = problems.status === 'success' || problems.status === 'empty'
    ? problems.data?.length
    : undefined

  const archived = contest.status === ContestStatus.ARCHIVED
  const isDraft = contest.status === ContestStatus.DRAFT
  const contestEditable = isDraft || contest.status === ContestStatus.SIGNUP
  const freezeText = contest.freeze_minutes > 0 ? `封榜 ${contest.freeze_minutes} 分钟` : '不封榜'

  return (
    <>
      <PageHeader
        title={contest.name}
        status={<StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />}
        meta={[
          { value: contestModeLabel(contest.mode) },
          { value: teamModeLabel(contest.team_mode) },
          ...(contest.match_mode ? [{ label: '对局方式', value: matchModeLabel(contest.match_mode) }] : []),
          { value: freezeText },
        ]}
        actions={
          <div className="flex items-center gap-2">
            <ContestNextStepButton
              contest={contest}
              placement="header"
              onDone={onRefresh}
              publishBlocker={problemCount === 0 ? '先添加至少一道赛题才能发布' : undefined}
            />
            <Menu>
              <MenuTrigger asChild>
                <IconButton variant="outline" icon={MoreVertical} aria-label="更多操作" />
              </MenuTrigger>
              <MenuContent align="end">
                <MenuItem icon={Pencil} disabled={!contestEditable} onSelect={() => setEditOpen(true)}>
                  {contestEditable ? '编辑赛事' : archived ? '编辑赛事(归档后只读)' : '编辑赛事(开赛后不能修改)'}
                </MenuItem>
              </MenuContent>
            </Menu>
          </div>
        }
      />

      <ContestTimeline contest={contest} className="mb-6" />

      {isDraft ? (
        <Callout tone="info" className="mb-6">
          {problemCount === 0
            ? '这场赛事还是草稿。发布前至少添加一道赛题,发布后学生才能看到并报名。'
            : '这场赛事还是草稿。编排好赛题后点右上角「发布」,发布后学生才能看到并报名。'}
        </Callout>
      ) : null}
      {isContestLeaderboardFrozen(contest.status) ? (
        <Callout tone="warning" className="mb-6">
          当前处于封榜期,榜单对学生暂停更新,比赛结束后名次一次性揭晓。
        </Callout>
      ) : null}

      <Tabs value={query.tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="赛事管理">
          <TabsTrigger value="problems" icon={Swords} count={problemCount}>
            赛题编排
          </TabsTrigger>
          <TabsTrigger value="cheat" icon={ShieldAlert}>
            防作弊审查
          </TabsTrigger>
          <TabsTrigger value="snapshot" icon={Archive}>
            归档榜单
          </TabsTrigger>
        </TabsList>

        <TabsContent value="problems">
          <ContestProblems contest={contest} problems={problems} readOnly={!isDraft} />
        </TabsContent>

        <TabsContent value="cheat">
          <ContestCheat contest={contest} problems={problems} readOnly={archived} />
        </TabsContent>

        <TabsContent value="snapshot">
          {archived ? (
            <ContestSnapshot contest={contest} />
          ) : (
            <PageSection title="归档榜单">
              <Empty
                icon={Archive}
                title="赛事还没有归档"
                description="归档后会生成最终榜单快照,之后这里显示不再变动的名次。"
              />
            </PageSection>
          )}
        </TabsContent>
      </Tabs>

      {editOpen ? (
        <ContestFormModal
          contest={contest}
          onClose={() => setEditOpen(false)}
          onSaved={() => {
            setEditOpen(false)
            onRefresh()
          }}
        />
      ) : null}
    </>
  )
}

interface SnapshotRow {
  teamId: string
  teamName: string
  rank: number
  score: number
  solvedCount: number
  lastSolveAt: string
}

function ContestSnapshot({ contest }: { contest: Contest }) {
  const snapshot = useAsyncResource(
    () => api.contest.getResultSnapshot(contest.id),
    [contest.id],
    (value: ResultSnapshot) => value.final_ranking.length === 0
  )

  const columns: TableColumn<SnapshotRow>[] = [
    { key: 'rank', header: '名次', align: 'right', mono: true, width: '88px' },
    {
      key: 'team',
      header: '队伍',
      render: (row) => <span className="text-ink">{row.teamName}</span>,
    },
    {
      key: 'score',
      header: '最终得分',
      align: 'right',
      mono: true,
      render: (row) => formatScore(row.score),
    },
    ...(contest.mode === ContestMode.SOLVE
      ? [
          { key: 'solvedCount', header: '通过题数', align: 'right' as const, mono: true },
          {
            key: 'lastSolveAt',
            header: '最近通过',
            render: (row: SnapshotRow) => (
              <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
                {row.lastSolveAt ? formatDateTime(row.lastSolveAt) : '—'}
              </span>
            ),
          },
        ]
      : []),
  ]

  return (
    <PageSection title="归档榜单" description="赛事归档时生成的最终名次,不再随后续操作变动。">
      <ResourceState
        resource={snapshot}
        emptyIcon={Archive}
        emptyTitle="快照里没有名次"
        emptyDescription="这场赛事归档时还没有队伍产生成绩。"
        skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
      >
        {(data) => (
          <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone="neutral">生成于 {formatDateTime(data.generated_at)}</Badge>
              <Badge tone="jade">{data.final_ranking.length} 支队伍上榜</Badge>
            </div>
            <DataPanel label="最终名次">
              <Table
                columns={columns}
                data={snapshotRows(data)}
                rowKey={(row) => row.teamId}
                elevated={false}
                mobileCard={(row) => ({
                  title: `第 ${row.rank} 名 · ${row.teamName}`,
                  meta:
                    contest.mode === ContestMode.SOLVE
                      ? `得分 ${formatScore(row.score)} · 通过 ${row.solvedCount} 题`
                      : `对抗积分 ${formatScore(row.score)}`,
                })}
              />
            </DataPanel>
          </div>
        )}
      </ResourceState>
    </PageSection>
  )
}

function snapshotRows(snapshot: ResultSnapshot): SnapshotRow[] {
  return snapshot.final_ranking
    .map((entry) => ({
      teamId: entry.team_id,
      teamName: entry.team_name,
      rank: entry.rank,
      score: entry.score,
      solvedCount: entry.solved_count,
      lastSolveAt: entry.last_solve_at ?? '',
    }))
    .sort((a, b) => a.rank - b.rank)
}
