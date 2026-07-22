// ProfilePage 展示当前登录账号资料、会话列表，并提供密码修改入口。

import React, { useEffect, useMemo } from 'react'
import type { Session } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { DescriptionList, Table } from '@chaimir/ui'
import { User } from 'lucide-react'
import { api } from '../../../../../app/api'
import { subscribeAppResource } from '../../../../../app/resourceInvalidation'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { accountStatusLabel, formatDateTime, sessionStatusLabel } from '../../../../../utils/index'
import { SecuritySettings } from './SecuritySettings'


const ProfilePage: React.FC = () => {
  const me = useAsyncResource(() => api.identity.getMe(), [])
  const sessions = useAsyncResource(() => api.identity.getSessions(), [])
  useEffect(() => subscribeAppResource('profile', me.reload), [me.reload])
  const columns = useMemo<TableColumn<Session>[]>(() => [
    { key: 'device', title: '设备', render: (row) => row.device_info || '未知设备', priority: 'primary' },
    { key: 'ip', title: 'IP 地址', render: (row) => row.ip || '暂无', priority: 'secondary' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{sessionStatusLabel(row.status)}</span> },
    { key: 'expire', title: '过期时间', render: (row) => <span className={styles.muted}>{formatDateTime(row.expire_at)}</span> },
  ], [])

  if (me.status === 'loading') {
    return <LoadingState title="正在获取账号资料" />
  }
  if (me.status === 'error') {
    return <ErrorState error={me.error} onRetry={me.reload} />
  }

  const account = me.data?.account

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <User size={28} />
            个人中心
          </h1>
          <p className={styles.subtitle}>查看当前账号资料，管理密码和登录会话。</p>
        </div>
      </div>

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>账号资料</h2>
          <DescriptionList
            items={[
              { key: 'name', label: '姓名', value: account?.name || '暂无' },
              { key: 'no', label: '学号工号', value: account?.no || '暂无' },
              { key: 'phone', label: '手机号', value: account?.phone_masked || '未绑定' },
              { key: 'status', label: '账号状态', value: account ? accountStatusLabel(account.status) : '暂无' },
            ]}
          />
        </section>

        <section className={`${styles.panel} ${styles.wide}`}>
          <h2>登录会话</h2>
          {sessions.status === 'error' && <ErrorState error={sessions.error} onRetry={sessions.reload} />}
          {sessions.status === 'loading' && <LoadingState title="正在获取会话列表" />}
          {(sessions.status === 'success' || sessions.status === 'empty') && (
            <Table
              columns={columns}
              rows={sessions.data || []}
              rowKey="id"
              emptyTitle="暂无会话"
              emptyDescription="当前没有可展示的登录会话。"
              ariaLabel="登录会话列表"
            />
          )}
        </section>
      </div>
      <SecuritySettings />
    </div>
  )
}

export default ProfilePage
