// 赛事详情页(教师深页,/teacher/contests/:contestId)。
//
// 一页承载赛事配置、赛题编排、防作弊审查与归档快照四件事 ——
// 它们都以同一场赛事为上下文,拆成四个侧栏项会让教师在页面间来回跳
// (对齐清单 §3.2:竞赛配置是流程编辑页、竞赛出题与防作弊审查是深页)。
//
// 教师侧没有单场赛事的读取接口(学生单读接口有非草稿门槛,草稿态赛事正是教师要编排的),
// 故从 teacher 组的列表里定位 —— 与实验编排向导同一做法。

import { useMemo, useState } from 'react'
import { useParams } from 'react-router'
import {
  Archive,
  Pencil,
  ShieldAlert,
  Swords,
  Trophy,
} from 'lucide-react'
import {
  ContestStatus,
  type Contest,
  type ResultSnapshot,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  DescriptionList,
  Empty,
  ObjectIdentity,
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
import { useAsyncResource } from '../../../../hooks'
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
import { ContestProblems } from './contest-problems'
import { ContestCheat } from './contest-cheat'

/**
 * TeacherContestDetailPage 读取赛事本体并按任务分区呈现管理能力。
 */
export default function TeacherContestDetailPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()

  // 单读走 teacher 组:它没有非草稿门槛,而草稿态赛事正是教师要编排的那些
  const contest = useAsyncResource(
    () => api.contest.getContest(contestId),
    [contestId],
    () => false,
  )

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
 * ContestDetailContent 渲染赛事头部、指标带与四个管理分区。
 */
function ContestDetailContent({ contest, onRefresh }: ContestDetailContentProps) {
  const [editOpen, setEditOpen] = useState(false)

  const archived = contest.status === ContestStatus.ARCHIVED
  const isDraft = contest.status === ContestStatus.DRAFT

  // 赛制、参赛形式、封榜时长与赛程都是赛事静态属性,统一进这张属性表,不占指标位(规范 §6.5)
  const scheduleItems = useMemo(
    () => [
      { term: '赛制', description: contestModeLabel(contest.mode) },
      { term: '参赛形式', description: teamModeLabel(contest.team_mode) },
      {
        term: '封榜时长',
        description: contest.freeze_minutes > 0 ? `${contest.freeze_minutes} 分钟` : '不封榜',
      },
      { term: '报名开始', description: formatDateTime(contest.signup_start), mono: true },
      { term: '报名截止', description: formatDateTime(contest.signup_end), mono: true },
      { term: '比赛开始', description: formatDateTime(contest.start_at), mono: true },
      { term: '比赛结束', description: formatDateTime(contest.end_at), mono: true },
    ],
    [contest],
  )

  return (
    <>
      {/*
        归族:详情族(§6.5.3 第 ④)。h1 由 ObjectIdentity 的赛事名承担,
        故页面头只出面包屑,末节到「赛事组织」为止(§6.5.0 通则 1)。
      */}
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实践' },
              { label: '赛事组织', href: '/teacher/contests' },
            ]}
          />
        }
      />

      {/*
        对象身份区:赛事名 + 状态 + 关键属性横排 + 对象级动作。
        赛制、参赛形式、封榜时长都是赛事静态属性,横排在这里;完整赛程留在下方属性表 ——
        七个时间点摊进身份区会把「一眼看清这是什么赛事」变成读时刻表(§6.5.3 第 ④:超过 6 项应当下沉)。
      */}
      <ObjectIdentity
        name={contest.name}
        status={
          <StatusIndicator
            tone={contestStatusTone(contest.status)}
            label={contestStatusLabel(contest.status)}
          />
        }
        subtitle={`${formatDateTime(contest.start_at)} — ${formatDateTime(contest.end_at)}`}
        actions={
          <Button variant="outline" leftIcon={Pencil} onClick={() => setEditOpen(true)}>
            编辑赛事
          </Button>
        }
        properties={[
          { label: '赛制', value: contestModeLabel(contest.mode) },
          { label: '参赛形式', value: teamModeLabel(contest.team_mode) },
          ...(contest.match_mode
            ? [{ label: '对局方式', value: matchModeLabel(contest.match_mode) }]
            : []),
          {
            label: '封榜时长',
            value: contest.freeze_minutes > 0 ? `${contest.freeze_minutes} 分钟` : '不封榜',
          },
          { label: '报名截止', value: formatDateTime(contest.signup_end) },
        ]}
      />

      {isDraft ? (
        <Callout tone="info" title="这场赛事还是草稿" className="mt-4">
          编排好赛题后回到赛事组织发布,发布后学生才能看到并报名。
        </Callout>
      ) : null}
      {isContestLeaderboardFrozen(contest.status) ? (
        <Callout tone="warning" className="mt-4">
          当前处于封榜期,榜单暂停更新。封榜结束后名次会一次性揭晓。
        </Callout>
      ) : null}

      <PageSection title="完整赛程" description="报名与比赛的四个时间点,以及赛制细节。" className="mt-6">
        <div className="rounded-lg bg-surface p-5 shadow-xs">
          <DescriptionList dense columns={2} items={scheduleItems} />
        </div>
      </PageSection>

      <Tabs defaultValue="problems">
        <TabsList>
          <TabsTrigger value="problems" icon={Swords}>
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
          <ContestProblems contest={contest} />
        </TabsContent>

        <TabsContent value="cheat">
          <ContestCheat contest={contest} />
        </TabsContent>

        <TabsContent value="snapshot">
          {archived ? (
            <ContestSnapshot contestId={contest.id} />
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
  rank: number
  score: number
  solvedCount: number
  lastSolveAt: string
}

/** ContestSnapshot 展示归档后的最终榜单。 */
function ContestSnapshot({ contestId }: { contestId: string }) {
  const snapshot = useAsyncResource(
    () => api.contest.getResultSnapshot(contestId),
    [contestId],
    (value: ResultSnapshot) => value.final_ranking.length === 0,
  )

  const columns: TableColumn<SnapshotRow>[] = [
    { key: 'rank', header: '名次', align: 'right', mono: true },
    {
      key: 'team',
      header: '队伍',
      // 快照只存 team_id,按名次呈现,不把内部编号当队名显示
      render: (row) => <span className="text-ink">第 {row.rank} 名队伍</span>,
    },
    {
      key: 'score',
      header: '最终得分',
      align: 'right',
      mono: true,
      render: (row) => formatScore(row.score),
    },
    { key: 'solvedCount', header: '通过题数', align: 'right', mono: true },
    {
      key: 'lastSolveAt',
      header: '最近通过',
      render: (row) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {row.lastSolveAt ? formatDateTime(row.lastSolveAt) : '—'}
        </span>
      ),
    },
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
                rowKey={(row) => String(row.rank)}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):名次一行、得分与通过题数一行
                mobileCard={(row) => ({
                  title: `第 ${row.rank} 名`,
                  meta: `得分 ${formatScore(row.score)} · 通过 ${row.solvedCount} 题`,
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
      rank: entry.rank,
      score: entry.score,
      solvedCount: entry.solved_count,
      lastSolveAt: entry.last_solve_at ?? '',
    }))
    .sort((a, b) => a.rank - b.rank)
}
