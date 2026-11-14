// 赛事详情页(教师深页,/teacher/contests/:contestId)。设计 §7.4:
// 页头为赛事名 + 状态 + 一行属性;右侧主按钮是赛程的「下一步」(与列表页同一套判定与确认),
// 编辑赛事在更多菜单;页头下方赛程条(与学生端详情同一组件);标签「赛题编排 / 防作弊审查 / 归档榜单」
// 写进地址;草稿提示「发布前至少添加一道赛题」,没有赛题时「发布」不可用并写原因。
// 单读走 teacher 组接口:它没有非草稿门槛,草稿态赛事正是教师要编排的。

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

/** 地址参数:tab 是当前标签 */
const DETAIL_QUERY = {
  tab: { defaultValue: 'problems', allowed: ['problems', 'cheat', 'snapshot'] },
} as const

/**
 * TeacherContestDetailPage 读取赛事本体并按任务分区呈现管理能力。
 */
export default function TeacherContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

  // 单读走 teacher 组:它没有非草稿门槛,而草稿态赛事正是教师要编排的那些
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

/**
 * ContestDetailContent 渲染赛事页头(下一步 + 更多菜单)、赛程条与三个管理标签。
 * 赛题数量由赛题标签读取后回报:没有赛题时「发布」不可用(后端也会拒绝),标签上带数量。
 */
function ContestDetailContent({ contest, onRefresh }: ContestDetailContentProps) {
  const [query, setQuery] = useUrlState(DETAIL_QUERY)
  const [editOpen, setEditOpen] = useState(false)
  const [problemCount, setProblemCount] = useState<number>()

  const archived = contest.status === ContestStatus.ARCHIVED
  const isDraft = contest.status === ContestStatus.DRAFT
  // 报名开始后赛程与赛制已对学生公开,只在草稿与报名中允许修改(沿用现有门槛)
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

        {/* 赛题标签始终挂载(forceMount):页头「发布」需要它回报的赛题数量,切到其他标签时仅隐藏 */}
        <TabsContent value="problems" forceMount className="data-[state=inactive]:hidden">
          <ContestProblems contest={contest} readOnly={!isDraft} onCountChange={setProblemCount} />
        </TabsContent>

        <TabsContent value="cheat">
          <ContestCheat contest={contest} readOnly={archived} />
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

/** SnapshotRow 是快照榜单的一行。 */
interface SnapshotRow {
  teamId: string
  teamName: string
  rank: number
  score: number
  solvedCount: number
  lastSolveAt: string
}

/** ContestSnapshot 展示归档后的最终榜单。 */
function ContestSnapshot({ contest }: { contest: Contest }) {
  const snapshot = useAsyncResource(
    () => api.contest.getResultSnapshot(contest.id),
    [contest.id],
    (value: ResultSnapshot) => value.final_ranking.length === 0
  )

  const columns: TableColumn<SnapshotRow>[] = [
    // 名次是窄的数字列:定宽靠左,不让表格把它摊成一大片空白
    { key: 'rank', header: '名次', align: 'right', mono: true, width: '88px' },
    {
      key: 'team',
      header: '队伍',
      // 队名由接口按 team_id 读取后随快照条目给出
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
            {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):快照不分页也不筛选,只用片本身 */}
            <DataPanel label="最终名次">
              <Table
                columns={columns}
                data={snapshotRows(data)}
                rowKey={(row) => row.teamId}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):名次一行、得分与通过题数一行
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

/** snapshotRows 把公开快照条目转换为展示行并按名次升序。 */
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
