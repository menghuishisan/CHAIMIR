// 账号管理页(校管首屏侧栏,/school-admin/users)。设计 §7.5:
// 页头 +「新增账号」,次要「批量导入」;工具条:状态分段(待激活 / 正常 / 已停用 / 已归档 / 全部)、身份、班级、搜索,
// 都写进地址;表格(勾选、姓名与编号、身份、所属组织、角色、状态、开通时间),整行打开账号抽屉(详情与编辑同一处);
// 行内更多菜单按状态给:停用或启用 / 重置密码 / 强制下线 / 授予或撤销管理员 / 归档或恢复 / 注销。
// 勾选后表格上方出现批量条(已选 n 个、批量停用、批量恢复、取消选择);筛选或翻页后选择自动清空。
// 导入在右侧大抽屉里分三步进行,导入记录也在那里。
//
// 账号状态是有向流转:待激活 → 正常 ⇄ 已停用 → 已归档 ⇄ 恢复,注销不可逆;每个动作单独确认并说明后果。

import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router'
import {
  CircleSlash,
  KeyRound,
  LogOut,
  MoreVertical,
  ShieldCheck,
  Trash2,
  Upload,
  UserCheck,
  UserPlus,
  Users,
} from 'lucide-react'
import { AccountStatus, BaseIdentity, UserRole, type Account } from '@chaimir/api-client'
import {
  Badge,
  Button,
  buttonVariants,
  Callout,
  Checkbox,
  Icon,
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
  Pagination,
  SegmentedControl,
  Select,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { segmentCounts } from '../../../../utils/facets'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { accountStatusLabel, baseIdentityLabel, userRoleLabel } from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { accountStatusTone } from '../../statusPresentation'
import { AccountFormModal } from './account-form'
import { AccountImportModal } from './account-import'
import { ResetPasswordModal } from './account-reset-password'

/** 地址参数:status 状态分段,role 身份,class 班级(空为全部),q 搜索;页码在 page */
const USERS_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'pending', 'active', 'disabled', 'archived'] },
  role: { defaultValue: 'all', allowed: ['all', 'teacher', 'student'] },
  class: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'pending', label: '待激活', status: AccountStatus.PENDING },
  { value: 'active', label: '正常', status: AccountStatus.ACTIVE },
  { value: 'disabled', label: '已停用', status: AccountStatus.DISABLED },
  { value: 'archived', label: '已归档', status: AccountStatus.ARCHIVED },
  { value: 'all', label: '全部' },
] as const

/** 班级筛选里「全部班级」的取值(下拉不接受空串作选项值) */
const ALL_CLASSES = 'all'

/** 单账号可执行的状态动作。 */
type AccountAction = 'disable' | 'enable' | 'archive' | 'restore' | 'cancel' | 'force-logout' | 'grant-admin' | 'revoke-admin'

const ACTION_COPY: Record<AccountAction, { title: string; description: string; confirm: string; danger?: boolean }> = {
  disable: {
    title: '停用这个账号?',
    description: '停用后这个人无法登录,已有的课程、作业与成绩记录保留。可以随时重新启用。',
    confirm: '确认停用',
  },
  enable: { title: '启用这个账号?', description: '启用后这个人可以正常登录。', confirm: '确认启用' },
  archive: {
    title: '归档这个账号?',
    description: '归档用于毕业或离职:账号不再出现在常规名单里,历史数据保留。可以恢复。',
    confirm: '确认归档',
  },
  restore: { title: '恢复这个账号?', description: '恢复后账号回到正常状态,可以重新登录。', confirm: '确认恢复' },
  cancel: {
    title: '注销这个账号?',
    description: '注销不能撤销。注销后手机号与学工号被释放,这个人无法再登录,历史数据以匿名形式保留。',
    confirm: '确认注销',
    danger: true,
  },
  'force-logout': {
    title: '强制下线?',
    description: '这个人当前所有登录会立即失效,需要重新登录。适用于设备丢失或账号异常。',
    confirm: '确认下线',
  },
  'grant-admin': {
    title: '授予学校管理员?',
    description: '授予后这个人可以管理全校账号、组织架构、成绩审核与学校配置。请谨慎授予。',
    confirm: '确认授予',
  },
  'revoke-admin': {
    title: '撤销学校管理员?',
    description: '撤销后这个人回到原本的教师身份,不能再进入管理端。',
    confirm: '确认撤销',
  },
}

/**
 * SchoolAdminUsersPage 承载账号列表、状态流转与批量操作。
 */
export default function SchoolAdminUsersPage() {
  const [query, setQuery] = useUrlState(USERS_QUERY)
  const [formTarget, setFormTarget] = useState<{ account?: Account } | undefined>()
  const [importOpen, setImportOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<Account>()
  const [confirm, setConfirm] = useState<{ action: AccountAction; account: Account }>()
  const [batchAction, setBatchAction] = useState<'disable' | 'restore'>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const statusFilter = STATUS_SEGMENTS.find((segment) => segment.value === query.status)
  const accounts = usePagedResource<Account>(
    (params) =>
      api.identity.getAccounts({
        role: query.role === 'teacher' ? BaseIdentity.TEACHER : query.role === 'student' ? BaseIdentity.STUDENT : undefined,
        status: statusFilter && 'status' in statusFilter ? statusFilter.status : undefined,
        class_id: query.class || undefined,
        keyword: query.q || undefined,
        ...params,
      }),
    [query.class, query.q, query.role, query.status],
    { urlPageKey: 'page' },
  )

  // 勾选只属于当前这一页、这组筛选:换筛选或翻页就作废,不对看不见的行做批量操作
  const selectionScope = `${query.status}|${query.role}|${query.class}|${query.q}|${accounts.page}|${accounts.refreshVersion}`
  const [selection, setSelection] = useState<{ scope: string; ids: Set<string> }>({ scope: '', ids: new Set() })
  const selected = useMemo(
    () => (selection.scope === selectionScope ? selection.ids : new Set<string>()),
    [selection, selectionScope],
  )
  const setSelected = (ids: Set<string>) => setSelection({ scope: selectionScope, ids })

  // 组织名称:教师挂院系、学生挂班级(后端 validateAccountOrgForProfile 同一口径)
  const org = useAsyncResource(
    () => Promise.all([api.identity.listDepartments(), api.identity.listClasses()]).then(([departments, classes]) => ({ departments, classes })),
    [],
    () => false,
  )
  const orgName = useMemo(() => {
    const names = new Map<string, string>()
    for (const department of org.data?.departments ?? []) names.set(department.id, department.name)
    for (const entity of org.data?.classes ?? []) names.set(entity.id, entity.name)
    return names
  }, [org.data])

  // 分段计数取列表接口的 facets:套用身份、班级与搜索,不套用状态筛选
  const facets = accounts.status === 'success' || accounts.status === 'empty' ? accounts.data?.facets : undefined
  const counts = segmentCounts(facets, 'status', STATUS_SEGMENTS, (segment) => ('status' in segment ? segment.status : undefined))

  /** runAction 执行单账号动作;列表重读后勾选随之作废 */
  const runAction = useCallback(async () => {
    if (!confirm) return
    setWorking(true)
    setActionError(undefined)
    try {
      const { action, account } = confirm
      if (action === 'disable') await api.identity.disableAccount(account.id)
      if (action === 'enable') await api.identity.enableAccount(account.id)
      if (action === 'archive') await api.identity.archiveAccount(account.id)
      if (action === 'restore') await api.identity.restoreAccount(account.id)
      if (action === 'cancel') await api.identity.cancelAccount(account.id)
      if (action === 'force-logout') await api.identity.forceLogoutAccount(account.id)
      if (action === 'grant-admin') await api.identity.grantSchoolAdmin(account.id)
      if (action === 'revoke-admin') await api.identity.revokeSchoolAdmin(account.id)
      toast.success(`${ACTION_COPY[action].confirm.replace('确认', '已')}:${account.name}`)
      setConfirm(undefined)
      accounts.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [accounts, confirm])

  /** runBatch 批量停用或恢复。按入学年份的整届归档归组织架构的升届流程。 */
  const runBatch = useCallback(async () => {
    if (!batchAction || selected.size === 0) return
    setWorking(true)
    setActionError(undefined)
    try {
      const accountIds = Array.from(selected)
      if (batchAction === 'disable') await api.identity.batchDisableAccounts({ account_ids: accountIds })
      if (batchAction === 'restore') await api.identity.batchRestoreAccounts({ account_ids: accountIds })
      toast.success(`已${batchAction === 'disable' ? '停用' : '恢复'} ${accountIds.length} 个账号`)
      setBatchAction(undefined)
      accounts.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '批量操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [accounts, batchAction, selected])

  const list = accounts.data?.list ?? []
  const allSelected = list.length > 0 && list.every((account) => selected.has(account.id))
  const toggle = (accountId: string) => {
    const next = new Set(selected)
    if (next.has(accountId)) next.delete(accountId)
    else next.add(accountId)
    setSelected(next)
  }

  const menuFor = (account: Account) => (
    <AccountActionMenu
      account={account}
      onRequest={(action) => setConfirm({ action, account })}
      onResetPassword={() => setResetTarget(account)}
    />
  )

  const columns: TableColumn<Account>[] = [
    {
      key: 'select',
      header: (
        <Checkbox
          checked={allSelected}
          aria-label="选择本页全部账号"
          disabled={list.length === 0}
          onCheckedChange={(checked) => setSelected(checked === true ? new Set(list.map((account) => account.id)) : new Set())}
        />
      ),
      render: (account) => <Checkbox checked={selected.has(account.id)} aria-label={`选择 ${account.name}`} onCheckedChange={() => toggle(account.id)} />,
    },
    {
      key: 'name',
      header: '姓名',
      primary: true,
      render: (account) => (
        <span className="block min-w-0">
          <span className="block truncate font-medium text-ink">{account.name}</span>
          <span className="block truncate font-mono text-xs text-ink-sub">{account.no ?? '未设学工号'}</span>
        </span>
      ),
    },
    {
      key: 'identity',
      header: '身份',
      render: (account) => (
        <span className="whitespace-nowrap text-ink-sub">
          {baseIdentityLabel(account.base_identity)}
          {account.title ? ` · ${account.title}` : ''}
        </span>
      ),
    },
    {
      key: 'org',
      header: '所属组织',
      render: (account) => (
        <span className="text-ink-sub">{account.org_id ? (orgName.get(account.org_id) ?? (org.status === 'loading' ? '…' : '已撤销的组织')) : '—'}</span>
      ),
    },
    {
      key: 'roles',
      header: '角色',
      render: (account) =>
        account.roles.includes(UserRole.SCHOOL_ADMIN) ? <Badge tone="jade">{userRoleLabel(UserRole.SCHOOL_ADMIN)}</Badge> : <span className="text-ink-sub">—</span>,
    },
    {
      key: 'status',
      header: '状态',
      render: (account) => <StatusIndicator tone={accountStatusTone(account.status)} label={accountStatusLabel(account.status)} />,
    },
    {
      key: 'created_at',
      header: '开通时间',
      render: (account) =>
        account.created_at ? (
          <span title={formatDateTime(account.created_at)} className="whitespace-nowrap text-ink-sub">
            {formatRelativeOccurrence(account.created_at)}
          </span>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
    { key: 'actions', header: '操作', align: 'right', render: menuFor },
  ]

  const filtered = query.status !== 'all' || query.role !== 'all' || query.class !== '' || query.q !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="账号管理"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" leftIcon={Upload} onClick={() => setImportOpen(true)}>
              批量导入
            </Button>
            <Button variant="primary" leftIcon={UserPlus} onClick={() => setFormTarget({})}>
              新增账号
            </Button>
          </div>
        }
      />

      <div className="mb-4 flex flex-col gap-3">
        <SegmentedControl
          aria-label="按账号状态筛选"
          options={STATUS_SEGMENTS.map((segment) => ({ value: segment.value, label: segment.label, count: counts[segment.value] }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full self-start overflow-x-auto"
        />
        <div className="flex flex-wrap items-center gap-3">
          <Select
            aria-label="按身份筛选"
            value={query.role}
            onValueChange={(value) => setQuery({ role: value })}
            options={[
              { value: 'all', label: '全部身份' },
              { value: 'teacher', label: '教师' },
              { value: 'student', label: '学生' },
            ]}
            className="w-full sm:w-36"
          />
          <Select
            aria-label="按班级筛选"
            value={query.class || ALL_CLASSES}
            onValueChange={(value) => setQuery({ class: value === ALL_CLASSES ? '' : value })}
            options={[
              { value: ALL_CLASSES, label: '全部班级' },
              ...(org.data?.classes ?? []).map((entity) => ({ value: entity.id, label: entity.name })),
            ]}
            className="w-full sm:w-48"
          />
          <SearchBox
            label="搜索账号"
            placeholder="搜索姓名、学工号或完整手机号"
            value={query.q}
            onSearch={(value) => setQuery({ q: value })}
            className="sm:ml-auto"
          />
        </div>
      </div>

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {selected.size > 0 ? (
        <div
          role="region"
          aria-label="批量操作"
          className="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-primary-soft px-4 py-2.5"
        >
          <span className="text-sm font-medium text-ink">已选 {selected.size} 个账号</span>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" leftIcon={CircleSlash} onClick={() => setBatchAction('disable')}>
              批量停用
            </Button>
            <Button variant="outline" size="sm" leftIcon={UserCheck} onClick={() => setBatchAction('restore')}>
              批量恢复
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
              取消选择
            </Button>
          </div>
        </div>
      ) : null}

      {accounts.status === 'error' ? (
        <ResourceError
          message={accounts.error?.message ?? '账号暂时读取不到'}
          traceId={accounts.error?.traceId}
          onRetry={accounts.reload}
        />
      ) : accounts.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Users} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有匹配的账号' : '还没有账号'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个条件再试,或清空筛选查看全部账号。' : '先建立组织架构,再单个新增或批量导入教师与学生账号。'}
          </p>
          <div className="mt-5 flex flex-wrap justify-center gap-2">
            {filtered ? (
              <Button variant="outline" onClick={() => setQuery({ status: 'all', role: 'all', class: '', q: '' })}>
                清空筛选
              </Button>
            ) : (
              <>
                <Button variant="primary" leftIcon={UserPlus} onClick={() => setFormTarget({})}>
                  新增账号
                </Button>
                <Link to="/school-admin/organization" className={buttonVariants({ variant: 'outline' })}>
                  先去建组织架构
                </Link>
              </>
            )}
          </div>
        </div>
      ) : (
        <>
          <Table
            columns={columns}
            data={list}
            rowKey={(account) => account.id}
            loading={accounts.status === 'loading' && !accounts.data}
            onRowClick={(account) => setFormTarget({ account })}
            // <md 换行卡(§6.4.1 规则 3):姓名一行、学工号与身份一行,状态在右,更多菜单在行尾
            mobileCard={(account) => ({
              title: account.name,
              meta: `${account.no ?? '—'} · ${baseIdentityLabel(account.base_identity)}${account.org_id && orgName.get(account.org_id) ? ` · ${orgName.get(account.org_id)}` : ''}`,
              badge: <StatusIndicator tone={accountStatusTone(account.status)} label={accountStatusLabel(account.status)} />,
              action: menuFor(account),
            })}
          />
          <div className="mt-4">
            <Pagination page={accounts.page} pageSize={accounts.pageSize} total={accounts.total} onPageChange={accounts.setPage} />
          </div>
        </>
      )}

      {formTarget ? (
        <AccountFormModal
          account={formTarget.account}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            accounts.reload()
          }}
        />
      ) : null}

      {importOpen ? (
        <AccountImportModal
          onClose={() => setImportOpen(false)}
          onCommitted={() => {
            setImportOpen(false)
            accounts.reload()
          }}
        />
      ) : null}

      {resetTarget ? (
        <ResetPasswordModal account={resetTarget} onClose={() => setResetTarget(undefined)} onDone={() => setResetTarget(undefined)} />
      ) : null}

      <Modal open={confirm !== undefined} onOpenChange={(open) => !open && !working && setConfirm(undefined)}>
        <ModalContent size="sm">
          {confirm ? (
            <>
              <ModalHeader>
                <ModalTitle>{ACTION_COPY[confirm.action].title}</ModalTitle>
                <ModalDescription>{ACTION_COPY[confirm.action].description}</ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base font-medium text-ink">
                  {confirm.account.name}
                  {confirm.account.no ? <span className="ml-2 font-mono text-sm text-ink-sub">{confirm.account.no}</span> : null}
                </p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" disabled={working} onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button variant={ACTION_COPY[confirm.action].danger ? 'danger' : 'primary'} loading={working} onClick={() => void runAction()}>
                  {ACTION_COPY[confirm.action].confirm}
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>

      <Modal open={batchAction !== undefined} onOpenChange={(open) => !open && !working && setBatchAction(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>{batchAction === 'disable' ? `停用选中的 ${selected.size} 个账号?` : `恢复选中的 ${selected.size} 个账号?`}</ModalTitle>
            <ModalDescription>
              {batchAction === 'disable'
                ? '被选中的账号将无法登录,历史数据保留。可以随时恢复。'
                : '被选中的账号回到正常状态,可以重新登录。'}
            </ModalDescription>
          </ModalHeader>
          <ModalFooter>
            <Button variant="outline" disabled={working} onClick={() => setBatchAction(undefined)}>
              取消
            </Button>
            <Button variant="primary" loading={working} onClick={() => void runBatch()}>
              {batchAction === 'disable' ? '确认停用' : '确认恢复'}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface AccountActionMenuProps {
  account: Account
  onRequest: (action: AccountAction) => void
  onResetPassword: () => void
}

/**
 * AccountActionMenu 按当前状态只给可执行的动作。注销不可逆,与常规项用分隔线隔开;
 * 管理员授权只对教师开放(后端 grantSchoolAdmin 要求账号是本租户教师);已注销的账号没有菜单。
 */
function AccountActionMenu({ account, onRequest, onResetPassword }: AccountActionMenuProps) {
  const isActive = account.status === AccountStatus.ACTIVE
  const isDisabled = account.status === AccountStatus.DISABLED
  const isArchived = account.status === AccountStatus.ARCHIVED
  const isTeacher = account.base_identity === BaseIdentity.TEACHER
  const isSchoolAdmin = account.roles.includes(UserRole.SCHOOL_ADMIN)

  if (account.status === AccountStatus.CANCELLED) return <span className="text-sm text-ink-sub">已注销</span>

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${account.name} 的更多操作`} />
      </MenuTrigger>
      <MenuContent align="end">
        {isActive ? (
          <MenuItem icon={CircleSlash} onSelect={() => onRequest('disable')}>
            停用
          </MenuItem>
        ) : null}
        {isDisabled ? (
          <MenuItem icon={UserCheck} onSelect={() => onRequest('enable')}>
            启用
          </MenuItem>
        ) : null}
        <MenuItem icon={KeyRound} onSelect={onResetPassword}>
          重置密码
        </MenuItem>
        <MenuItem icon={LogOut} onSelect={() => onRequest('force-logout')}>
          强制下线
        </MenuItem>
        {isTeacher ? (
          <MenuItem icon={ShieldCheck} onSelect={() => onRequest(isSchoolAdmin ? 'revoke-admin' : 'grant-admin')}>
            {isSchoolAdmin ? '撤销管理员' : '授予管理员'}
          </MenuItem>
        ) : null}
        {isActive || isDisabled ? <MenuItem onSelect={() => onRequest('archive')}>归档</MenuItem> : null}
        {isArchived ? <MenuItem onSelect={() => onRequest('restore')}>恢复</MenuItem> : null}
        <MenuSeparator />
        <MenuItem danger icon={Trash2} onSelect={() => onRequest('cancel')}>
          注销
        </MenuItem>
      </MenuContent>
    </Menu>
  )
}
