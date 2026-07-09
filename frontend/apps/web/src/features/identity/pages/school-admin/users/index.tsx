// UsersPage 展示当前租户账号列表，并调用 identity 后端执行账号状态操作。

import React, { useCallback, useMemo, useState } from 'react'
import type { Account, ApiError } from '@chaimir/api-client'
import { AccountStatus, UserRole } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table } from '@chaimir/ui'
import { RefreshCw, Upload, UserPlus, Users } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { accountRoleFilterOptions, accountRoleLabel, accountStatusFilterOptions, accountStatusLabel } from '../../../../../utils/index'

const PAGE_SIZE = 20



const UsersPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [role, setRole] = useState('')
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const resource = useAsyncResource(() => api.identity.getAccounts({
    keyword: keyword || undefined,
    role: role ? Number(role) as UserRole : undefined,
    status: status ? Number(status) as AccountStatus : undefined,
    page: 1,
    size: PAGE_SIZE,
  }), [keyword, role, status])

  /**
   * runAccountAction 调用账号状态接口并刷新列表。
   */
  const runAccountAction = useCallback(async (action: () => Promise<void>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      resource.reload()
    } catch (actionError) {
      setError((actionError as ApiError).message || '账号操作失败，请稍后重试。')
    }
  }, [resource])

  const columns = useMemo<TableColumn<Account>[]>(() => [
    { key: 'no', title: '学号/工号', render: (row) => row.no || '未设置', priority: 'primary' },
    { key: 'name', title: '姓名', dataIndex: 'name', priority: 'primary' },
    { key: 'role', title: '角色', render: (row) => accountRoleLabel(row.roles), priority: 'secondary' },
    { key: 'phone', title: '手机号', render: (row) => row.phone_masked || '未绑定' },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{accountStatusLabel(row.status)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="ghost" size="sm" onClick={() => navigate(`/school-admin/users/edit?id=${row.id}`)}>
            编辑
          </Button>
          {row.status === AccountStatus.DISABLED ? (
            <Button variant="outline" size="sm" onClick={() => runAccountAction(() => api.identity.enableAccount(row.id), '账号已启用。')}>
              启用
            </Button>
          ) : (
            <Button variant="outline" size="sm" onClick={() => runAccountAction(() => api.identity.disableAccount(row.id), '账号已停用。')}>
              停用
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => runAccountAction(() => api.identity.forceLogoutAccount(row.id), '账号会话已强制下线。')}>
            强制下线
          </Button>
        </div>
      ),
    },
  ], [navigate, runAccountAction])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Users size={28} />
            账号管理
          </h1>
          <p className={styles.subtitle}>维护当前学校租户内的教师、学生和学校管理员账号。</p>
        </div>
        <div className={styles.toolbar}>
          <Button variant="outline" icon={<Upload size={16} />} onClick={() => navigate('/school-admin/users/import')}>
            批量导入
          </Button>
          <Button icon={<UserPlus size={16} />} onClick={() => navigate('/school-admin/users/edit')}>
            新增账号
          </Button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="操作成功">
          {message}
        </Callout>
      )}

      <div className={styles.toolbar}>
        <Input placeholder="搜索姓名、学号或工号" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Select value={role} options={accountRoleFilterOptions} onChange={setRole} />
        <Select value={status} options={accountStatusFilterOptions} onChange={setStatus} />
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取账号列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无账号"
            emptyDescription="当前筛选条件下没有账号记录。"
            ariaLabel="账号列表"
          />
        </div>
      )}
    </div>
  )
}

export default UsersPage
