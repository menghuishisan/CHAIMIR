// 账号管理页(校管首屏侧栏,/school-admin/users)。
//
// 学校管理员的第一件事:开通与维护教师/学生账号。一页承载列表、状态流转、
// 管理员授权与批量操作;新增/编辑/导入是本页的流程页,从这里进入(对齐清单 §3.3)。
//
// 账号状态是有向流转:待激活 → 正常 ⇄ 已停用 → 已归档 ⇄ 恢复,注销不可逆。
// 每个动作单独确认并说明后果,避免误点改变学生登录能力。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import {
  CircleSlash,
  Download,
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
import {
  AccountStatus,
  BaseIdentity,
  UserRole,
  type Account,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Checkbox,
  FormField,
  IconButton,
  Input,
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
  Select,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDate } from '../../../../utils/formatters'
import {
  accountStatusLabel,
  accountStatusTone,
  baseIdentityLabel,
  userRoleLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { AccountFormModal } from './account-form'
import { AccountImportModal } from './account-import'
import { AccountImportBatches } from './account-batches'
import { ResetPasswordModal } from './account-reset-password'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(AccountStatus.PENDING), label: '待激活' },
  { value: String(AccountStatus.ACTIVE), label: '正常' },
  { value: String(AccountStatus.DISABLED), label: '已停用' },
  { value: String(AccountStatus.ARCHIVED), label: '已归档' },
] as const

/** 身份筛选项:值为空串表示不过滤。 */
const ROLE_FILTERS = [
  { value: '', label: '全部' },
  { value: String(UserRole.TEACHER), label: '教师' },
  { value: String(UserRole.STUDENT), label: '学生' },
] as const

/** 单账号可执行的状态动作。 */
type AccountAction =
  | 'disable'
  | 'enable'
  | 'archive'
  | 'restore'
  | 'cancel'
  | 'force-logout'
  | 'grant-admin'
  | 'revoke-admin'

const ACTION_COPY: Record<AccountAction, { title: string; description: string; confirm: string; danger?: boolean }> = {
  disable: {
    title: '确认停用账号',
    description: '停用后这个人无法登录,已有的课程、作业与成绩记录保留。可以随时重新启用。',
    confirm: '确认停用',
  },
  enable: {
    title: '确认启用账号',
    description: '启用后这个人可以正常登录。',
    confirm: '确认启用',
  },
  archive: {
    title: '确认归档账号',
    description: '归档用于毕业或离职:账号不再出现在常规名单里,历史数据保留。可以恢复。',
    confirm: '确认归档',
  },
  restore: {
    title: '确认恢复账号',
    description: '恢复后账号回到正常状态,可以重新登录。',
    confirm: '确认恢复',
  },
  cancel: {
    title: '确认注销账号',
    description: '注销不可撤销。注销后手机号与学工号被释放,这个人无法再登录,历史数据以匿名形式保留。',
    confirm: '确认注销',
    danger: true,
  },
  'force-logout': {
    title: '确认强制下线',
    description: '这个人当前所有登录会话会立即失效,需要重新登录。适用于设备丢失或账号异常。',
    confirm: '确认下线',
  },
  'grant-admin': {
    title: '确认授予学校管理员',
    description: '授予后这个人可以管理全校账号、组织架构、成绩审核与学校配置。请谨慎授予。',
    confirm: '确认授予',
  },
  'revoke-admin': {
    title: '确认撤销学校管理员',
    description: '撤销后这个人回到原本的教师或学生身份,不能再进入管理端。',
    confirm: '确认撤销',
  },
}

/**
 * SchoolAdminUsersPage 承载账号列表、状态流转与批量操作。
 */
export default function SchoolAdminUsersPage() {
  const navigate = useNavigate()
  const [roleFilter, setRoleFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [classFilter, setClassFilter] = useState<string>('')
  const [keyword, setKeyword] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [formTarget, setFormTarget] = useState<{ account?: Account } | undefined>()
  const [importOpen, setImportOpen] = useState(false)
  const [resetTarget, setResetTarget] = useState<Account>()
  const [confirm, setConfirm] = useState<{ action: AccountAction; account: Account }>()
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [batchAction, setBatchAction] = useState<'disable' | 'restore'>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const accounts = usePagedResource<Account>(
    (params) =>
      api.identity.getAccounts({
        role: roleFilter ? (Number(roleFilter) as UserRole) : undefined,
        status: statusFilter ? (Number(statusFilter) as AccountStatus) : undefined,
        class_id: classFilter || undefined,
        keyword: searchTerm || undefined,
        ...params,
      }),
    [roleFilter, statusFilter, classFilter, searchTerm],
  )

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)

  /** runAction 执行单账号动作。每个动作后清空选择,避免对已变更的账号继续批量操作。 */
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
      toast.success('操作已完成')
      setConfirm(undefined)
      setSelected(new Set())
      accounts.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [accounts, confirm])

  /** runBatch 批量停用或恢复。后端另有按入学年份的批量归档,归属组织架构的升届流程。 */
  const runBatch = useCallback(async () => {
    if (!batchAction || selected.size === 0) return
    setWorking(true)
    setActionError(undefined)
    try {
      const accountIds = Array.from(selected)
      if (batchAction === 'disable') await api.identity.batchDisableAccounts({ account_ids: accountIds })
      if (batchAction === 'restore') await api.identity.batchRestoreAccounts({ account_ids: accountIds })
      toast.success(`已处理 ${accountIds.length} 个账号`)
      setBatchAction(undefined)
      setSelected(new Set())
      accounts.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '批量操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [accounts, batchAction, selected])

  /** downloadTemplate 下载导入模板:文件名取自响应头,页面不自造。 */
  const downloadTemplate = useCallback(async (type: 'teacher' | 'student') => {
    setActionError(undefined)
    try {
      const file = await api.identity.downloadAccountImportTemplate({ type })
      const url = URL.createObjectURL(file.blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = file.fileName
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '模板下载没有成功,请稍后重试。'))
    }
  }, [])

  const toggleSelect = useCallback((accountId: string) => {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(accountId)) next.delete(accountId)
      else next.add(accountId)
      return next
    })
  }, [])

  const stats = useMemo(() => {
    const list = accounts.data ? accounts.data.list : []
    return {
      pending: list.filter((item) => item.status === AccountStatus.PENDING).length,
      active: list.filter((item) => item.status === AccountStatus.ACTIVE).length,
      disabled: list.filter((item) => item.status === AccountStatus.DISABLED).length,
    }
  }, [accounts.data])

  const classOptions = useMemo(
    () => [
      { value: '', label: '全部班级' },
      ...(classes.data ?? []).map((item) => ({ value: item.id, label: item.name })),
    ],
    [classes.data],
  )

  const columns: TableColumn<Account>[] = [
    {
      key: 'select',
      header: '',
      render: (account) => (
        <Checkbox
          checked={selected.has(account.id)}
          aria-label={`选择 ${account.name}`}
          onCheckedChange={() => toggleSelect(account.id)}
        />
      ),
    },
    {
      key: 'name',
      header: '姓名',
      render: (account) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{account.name}</div>
          <div className="truncate font-mono text-xs text-ink-sub">
            {account.no ?? '未设学工号'}
            {account.phone_masked ? ` · ${account.phone_masked}` : ''}
          </div>
        </div>
      ),
    },
    {
      key: 'base_identity',
      header: '身份',
      render: (account) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="neutral">{baseIdentityLabel(account.base_identity)}</Badge>
          {account.roles.includes(UserRole.SCHOOL_ADMIN) ? (
            <Badge tone="jade">{userRoleLabel(UserRole.SCHOOL_ADMIN)}</Badge>
          ) : null}
          {account.title ? <span className="text-xs text-ink-sub">{account.title}</span> : null}
        </div>
      ),
    },
    {
      key: 'created_at',
      header: '开通时间',
      render: (account) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {account.created_at ? formatDate(account.created_at) : '—'}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (account) => (
        <StatusIndicator tone={accountStatusTone(account.status)} label={accountStatusLabel(account.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (account) => (
        <div className="flex items-center justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => setFormTarget({ account })}>
            编辑
          </Button>
          <AccountActionMenu
            account={account}
            onRequest={(action) => setConfirm({ action, account })}
            onResetPassword={() => setResetTarget(account)}
          />
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '用户与组织' }, { label: '账号管理' }]} />}
        title="账号管理"
        description="开通教师与学生账号、维护状态、授予管理员权限。批量开通请用导入。"
        icon={Users}
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

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="账号总数" value={accounts.total} icon={Users} />
          <Stat label="本页待激活" value={stats.pending} icon={UserPlus} hint="尚未设置密码" />
          <Stat label="本页正常" value={stats.active} icon={UserCheck} />
          <Stat label="本页已停用" value={stats.disabled} icon={CircleSlash} />
        </div>
      </PageSection>

      <PageSection
        title="账号列表"
        description={`共 ${accounts.total} 个账号`}
        actions={
          <div className="flex flex-wrap items-end gap-2">
            <SegmentedControl
              aria-label="按身份筛选"
              size="sm"
              options={ROLE_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={roleFilter}
              onValueChange={(value) => {
                setRoleFilter(value)
                setSelected(new Set())
              }}
            />
            <SegmentedControl
              aria-label="按账号状态筛选"
              size="sm"
              options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={statusFilter}
              onValueChange={(value) => {
                setStatusFilter(value)
                setSelected(new Set())
              }}
            />
            <FormField label="班级" htmlFor="users-class" className="mb-0">
              <Select
                id="users-class"
                options={classOptions}
                value={classFilter}
                placeholder="全部班级"
                onValueChange={setClassFilter}
              />
            </FormField>
            <form
              className="flex items-end gap-2"
              onSubmit={(event) => {
                event.preventDefault()
                setSearchTerm(keyword.trim())
              }}
            >
              <FormField label="搜索" htmlFor="users-keyword" className="mb-0">
                <Input
                  id="users-keyword"
                  value={keyword}
                  placeholder="姓名或学工号"
                  onChange={(event) => setKeyword(event.target.value)}
                />
              </FormField>
              <Button type="submit" variant="outline" size="sm">
                搜索
              </Button>
            </form>
          </div>
        }
      >
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          {selected.size > 0 ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-line bg-surface-sunken p-3">
              <span className="text-sm text-ink">已选择 {selected.size} 个账号</span>
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="outline" size="sm" onClick={() => setBatchAction('disable')}>
                  批量停用
                </Button>
                <Button variant="outline" size="sm" onClick={() => setBatchAction('restore')}>
                  批量恢复
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
                  取消选择
                </Button>
              </div>
            </div>
          ) : null}

          <ResourceState
            resource={accounts}
            emptyIcon={Users}
            emptyTitle={
              statusFilter || roleFilter || classFilter || searchTerm ? '没有匹配的账号' : '还没有账号'
            }
            emptyDescription={
              statusFilter || roleFilter || classFilter || searchTerm
                ? '换个条件再试,或清空筛选查看全部账号。'
                : '先建立组织架构,再单个新增或批量导入教师与学生账号。'
            }
            emptyAction={
              statusFilter || roleFilter || classFilter || searchTerm ? undefined : (
                <div className="flex flex-wrap items-center gap-2">
                  <Button variant="primary" leftIcon={UserPlus} onClick={() => setFormTarget({})}>
                    新增账号
                  </Button>
                  <Button variant="outline" onClick={() => navigate('/school-admin/organization')}>
                    先去建组织架构
                  </Button>
                </div>
              )
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={accounts.page}
                  pageSize={accounts.pageSize}
                  total={accounts.total}
                  onPageChange={accounts.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>

      <PageSection
        title="导入模板"
        description="按模板填写后上传,系统会先预览校验再提交。"
      >
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" leftIcon={Download} onClick={() => void downloadTemplate('teacher')}>
            教师导入模板
          </Button>
          <Button variant="outline" leftIcon={Download} onClick={() => void downloadTemplate('student')}>
            学生导入模板
          </Button>
          <Button variant="ghost" leftIcon={Upload} onClick={() => setImportOpen(true)}>
            开始导入
          </Button>
        </div>
      </PageSection>

      <AccountImportBatches />

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
        <ResetPasswordModal
          account={resetTarget}
          onClose={() => setResetTarget(undefined)}
          onDone={() => setResetTarget(undefined)}
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
                <p className="text-base text-ink">
                  {confirm.account.name}
                  {confirm.account.no ? ` · ${confirm.account.no}` : ''}
                </p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button
                  variant={ACTION_COPY[confirm.action].danger ? 'danger' : 'seal'}
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

      <Modal open={batchAction !== undefined} onOpenChange={(open) => !open && setBatchAction(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>{batchAction === 'disable' ? '批量停用账号' : '批量恢复账号'}</ModalTitle>
            <ModalDescription>
              {batchAction === 'disable'
                ? '被选中的账号将无法登录,历史数据保留。可以随时恢复。'
                : '被选中的账号回到正常状态,可以重新登录。'}
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">共 {selected.size} 个账号</p>
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setBatchAction(undefined)}>
              取消
            </Button>
            <Button variant="seal" loading={working} onClick={() => void runBatch()}>
              确认处理
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
 * AccountActionMenu 按当前状态给出可执行动作。
 * 注销不可逆,与常规项用分隔线隔开;管理员授权只对教师开放
 * (后端 grantSchoolAdmin 要求账号是本租户教师)。
 */
function AccountActionMenu({ account, onRequest, onResetPassword }: AccountActionMenuProps) {
  const isActive = account.status === AccountStatus.ACTIVE
  const isDisabled = account.status === AccountStatus.DISABLED
  const isArchived = account.status === AccountStatus.ARCHIVED
  const isCancelled = account.status === AccountStatus.CANCELLED
  const isTeacher = account.base_identity === BaseIdentity.TEACHER
  const isSchoolAdmin = account.roles.includes(UserRole.SCHOOL_ADMIN)

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton
          variant="ghost"
          size="sm"
          icon={MoreVertical}
          aria-label={`${account.name} 的更多操作`}
        />
      </MenuTrigger>
      <MenuContent align="end">
        {isActive ? (
          <MenuItem icon={CircleSlash} onSelect={() => onRequest('disable')}>
            停用账号
          </MenuItem>
        ) : null}
        {isDisabled ? (
          <MenuItem icon={UserCheck} onSelect={() => onRequest('enable')}>
            启用账号
          </MenuItem>
        ) : null}
        {isActive || isDisabled ? (
          <MenuItem onSelect={() => onRequest('archive')}>归档账号</MenuItem>
        ) : null}
        {isArchived ? <MenuItem onSelect={() => onRequest('restore')}>恢复账号</MenuItem> : null}

        {isCancelled ? null : (
          <>
            <MenuSeparator />
            <MenuItem icon={KeyRound} onSelect={onResetPassword}>
              重置密码
            </MenuItem>
            <MenuItem icon={LogOut} onSelect={() => onRequest('force-logout')}>
              强制下线
            </MenuItem>
          </>
        )}

        {isTeacher && !isCancelled ? (
          <>
            <MenuSeparator />
            {isSchoolAdmin ? (
              <MenuItem icon={ShieldCheck} onSelect={() => onRequest('revoke-admin')}>
                撤销管理员
              </MenuItem>
            ) : (
              <MenuItem icon={ShieldCheck} onSelect={() => onRequest('grant-admin')}>
                授予管理员
              </MenuItem>
            )}
          </>
        ) : null}

        {isCancelled ? (
          <MenuItem disabled>账号已注销,无可执行操作</MenuItem>
        ) : (
          <>
            <MenuSeparator />
            <MenuItem danger icon={Trash2} onSelect={() => onRequest('cancel')}>
              注销账号
            </MenuItem>
          </>
        )}
      </MenuContent>
    </Menu>
  )
}
