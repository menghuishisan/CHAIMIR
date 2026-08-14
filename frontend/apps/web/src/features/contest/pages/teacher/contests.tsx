// 赛事组织页(教师侧栏,/teacher/contests)。
// 赛事状态是有向流转:草稿→报名→进行中→封榜→结束→归档。
// 归档会生成结果快照且不可逆,故走确认;其余流转也各自确认,避免误点改变学生可见性。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bug, Flag, Lock, MoreVertical, Play, Plus, Snowflake, Trophy } from 'lucide-react'
import { ContestStatus, type Contest } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  Callout,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  contestModeLabel,
  contestStatusLabel,
  contestStatusTone,
  teamModeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContestFormModal } from './contest-form'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(ContestStatus.DRAFT), label: '草稿' },
  { value: String(ContestStatus.SIGNUP), label: '报名中' },
  { value: String(ContestStatus.RUNNING), label: '进行中' },
  { value: String(ContestStatus.ENDED), label: '已结束' },
] as const

/** 可执行的状态流转动作。 */
type LifecycleAction = 'publish' | 'start' | 'freeze' | 'end' | 'archive'

const ACTION_COPY: Record<LifecycleAction, { title: string; description: string; confirm: string }> = {
  publish: {
    title: '确认发布赛事',
    description: '发布后学生可以看到赛事并在报名窗口内报名。赛制与时间安排将不宜再改。',
    confirm: '确认发布',
  },
  start: {
    title: '确认开始比赛',
    description: '开赛后报名关闭、队伍自动锁定,学生可以进入答题。',
    confirm: '确认开赛',
  },
  freeze: {
    title: '确认封榜',
    description: '封榜后天梯榜停止对学生更新,学生仍可继续提交。',
    confirm: '确认封榜',
  },
  end: {
    title: '确认结束比赛',
    description: '结束后学生不能再提交,榜单恢复显示最终名次。',
    confirm: '确认结束',
  },
  archive: {
    title: '确认归档赛事',
    description: '归档会生成最终成绩快照,之后赛事转为只读。归档不可撤销。',
    confirm: '确认归档',
  },
}

/**
 * TeacherContestsPage 列出赛事并承载状态流转。
 */
export default function TeacherContestsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [createOpen, setCreateOpen] = useState(false)
  const [confirm, setConfirm] = useState<{ action: LifecycleAction; contest: Contest }>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const contests = usePagedResource<Contest>(
    (params) =>
      api.contest.getContests({
        status: statusFilter ? (Number(statusFilter) as ContestStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  /** runAction 执行状态流转。归档返回结果快照,其余返回赛事本体。 */
  const runAction = useCallback(async () => {
    if (!confirm) return
    setWorking(true)
    setActionError(undefined)
    try {
      const { action, contest } = confirm
      if (action === 'publish') await api.contest.publishContest(contest.id)
      if (action === 'start') await api.contest.startContest(contest.id)
      if (action === 'freeze') await api.contest.freezeContest(contest.id)
      if (action === 'end') await api.contest.endContest(contest.id)
      if (action === 'archive') await api.contest.archiveContest(contest.id)
      toast.success('赛事状态已更新')
      setConfirm(undefined)
      contests.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [confirm, contests])

  const stats = useMemo(() => {
    const list = contests.data ? contests.data.list : []
    return {
      signup: list.filter((item) => item.status === ContestStatus.SIGNUP).length,
      running: list.filter(
        (item) => item.status === ContestStatus.RUNNING || item.status === ContestStatus.FROZEN,
      ).length,
      draft: list.filter((item) => item.status === ContestStatus.DRAFT).length,
    }
  }, [contests.data])

  const columns: TableColumn<Contest>[] = [
    {
      key: 'name',
      header: '赛事',
      render: (contest) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{contest.name}</div>
          <div className="truncate text-xs text-ink-sub">
            {contestModeLabel(contest.mode)} · {teamModeLabel(contest.team_mode)}
          </div>
        </div>
      ),
    },
    {
      key: 'signup',
      header: '报名窗口',
      render: (contest) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(contest.signup_start)} — {formatDateTime(contest.signup_end)}
        </span>
      ),
    },
    {
      key: 'schedule',
      header: '比赛时间',
      render: (contest) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(contest.start_at)} — {formatDateTime(contest.end_at)}
        </span>
      ),
    },
    {
      key: 'freeze_minutes',
      header: '封榜时长',
      align: 'right',
      mono: true,
      render: (contest) => `${contest.freeze_minutes} 分钟`,
    },
    {
      key: 'status',
      header: '状态',
      render: (contest) => (
        <StatusIndicator tone={contestStatusTone(contest.status)} label={contestStatusLabel(contest.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (contest) => (
        <div className="flex items-center justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => navigate(`/teacher/contests/${contest.id}`)}>
            管理赛事
          </Button>
          <ContestActionMenu contest={contest} onRequest={(action) => setConfirm({ action, contest })} />
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '实践' }, { label: '赛事组织' }]} />}
        title="赛事组织"
        description="创建赛事、编排赛题,并按赛程推进报名、开赛、封榜与结束。"
        icon={Trophy}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" leftIcon={Bug} onClick={() => navigate('/teacher/vuln-workshop')}>
              漏洞题工坊
            </Button>
            <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              新建赛事
            </Button>
          </div>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="赛事总数" value={contests.total} icon={Trophy} />
          <Stat label="报名中" value={stats.signup} icon={Flag} />
          <Stat label="进行中" value={stats.running} icon={Play} />
          <Stat label="草稿" value={stats.draft} icon={Plus} hint="发布后学生可见" />
        </div>
      </PageSection>

      <PageSection
        title="赛事列表"
        description={`共 ${contests.total} 场赛事`}
        actions={
          <SegmentedControl
            aria-label="按赛事状态筛选"
            size="sm"
            options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
            value={statusFilter}
            onValueChange={setStatusFilter}
          />
        }
      >
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={contests}
            emptyIcon={Trophy}
            emptyTitle={statusFilter ? '这个状态下没有赛事' : '还没有赛事'}
            emptyDescription={
              statusFilter ? '换个状态看看。' : '新建赛事后编排赛题,发布即可开放报名。'
            }
            emptyAction={
              statusFilter ? undefined : (
                <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
                  新建赛事
                </Button>
              )
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={contests.page}
                  pageSize={contests.pageSize}
                  total={contests.total}
                  onPageChange={contests.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>

      {createOpen ? (
        <ContestFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            contests.reload()
          }}
        />
      ) : null}

      <Modal open={confirm !== undefined} onOpenChange={(open) => !open && setConfirm(undefined)}>
        <ModalContent size="sm">
          {confirm ? (
            <>
              <ModalHeader>
                <ModalTitle>{ACTION_COPY[confirm.action].title}</ModalTitle>
                <ModalDescription>{ACTION_COPY[confirm.action].description}</ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base text-ink">{confirm.contest.name}</p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button
                  variant={confirm.action === 'archive' ? 'danger' : 'seal'}
                  loading={working}
                  onClick={() => void runAction()}
                >
                  {ACTION_COPY[confirm.action].confirm}
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface ContestActionMenuProps {
  contest: Contest
  onRequest: (action: LifecycleAction) => void
}

/**
 * ContestActionMenu 按当前状态给出可执行的流转动作。
 * 归档是不可逆动作,与常规项用分隔线隔开。
 */
function ContestActionMenu({ contest, onRequest }: ContestActionMenuProps) {
  const now = Date.now()
  const startAt = Date.parse(contest.start_at)
  const endAt = Date.parse(contest.end_at)
  const freezeAt = endAt - contest.freeze_minutes * 60_000
  const canPublish = contest.status === ContestStatus.DRAFT
  const canStart = contest.status === ContestStatus.SIGNUP && now >= startAt && now < endAt
  const canFreeze =
    contest.status === ContestStatus.RUNNING && contest.freeze_minutes > 0 && now >= freezeAt && now < endAt
  const canEnd =
    (contest.status === ContestStatus.RUNNING || contest.status === ContestStatus.FROZEN) && now >= endAt
  const canArchive = contest.status === ContestStatus.ENDED

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton
          variant="ghost"
          size="sm"
          icon={MoreVertical}
          aria-label={`${contest.name} 的更多操作`}
        />
      </MenuTrigger>
      <MenuContent align="end">
        {canPublish ? <MenuItem onSelect={() => onRequest('publish')}>发布赛事</MenuItem> : null}
        {canStart ? (
          <MenuItem icon={Play} onSelect={() => onRequest('start')}>
            开始比赛
          </MenuItem>
        ) : null}
        {canFreeze ? (
          <MenuItem icon={Snowflake} onSelect={() => onRequest('freeze')}>
            封榜
          </MenuItem>
        ) : null}
        {canEnd ? (
          <MenuItem icon={Lock} onSelect={() => onRequest('end')}>
            结束比赛
          </MenuItem>
        ) : null}
        {canArchive ? (
          <>
            <MenuSeparator />
            <MenuItem danger onSelect={() => onRequest('archive')}>
              归档赛事
            </MenuItem>
          </>
        ) : null}
        {!canPublish && !canStart && !canFreeze && !canEnd && !canArchive ? (
          <MenuItem disabled>当前状态没有可执行的操作</MenuItem>
        ) : null}
      </MenuContent>
    </Menu>
  )
}
