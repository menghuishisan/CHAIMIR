// SimulationsPage 展示已发布仿真包，并进入 sim-sdk 工作台。

import React, { useMemo, useState } from 'react'
import { SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Input, Table } from '@chaimir/ui'
import { Network, Play, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../sim.module.css'

const SimulationsPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const resource = useAsyncResource(() => api.sim.getPackages({
    keyword: keyword || undefined,
    status: SIM_PACKAGE_STATUS.PUBLISHED,
    page: 1,
    size: 20,
  }), [keyword])

  const columns = useMemo<TableColumn<SimPackageMeta>[]>(() => [
    { key: 'name', title: '仿真名称', dataIndex: 'name', priority: 'primary' },
    { key: 'category', title: '分类', dataIndex: 'category' },
    { key: 'version', title: '版本', dataIndex: 'version' },
    { key: 'compute', title: '运行方式', dataIndex: 'compute' },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <Button size="sm" icon={<Play size={14} />} onClick={() => navigate(`/student/simulations/${row.code}/workspace?version=${encodeURIComponent(row.version)}`)}>启动工作台</Button>,
    },
  ], [navigate])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Network size={28} />仿真实验室</h1>
          <p className={styles.subtitle}>选择已发布仿真包进入沉浸式工作台。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      <div className={styles.toolbar}>
        <Input placeholder="搜索仿真包" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
      </div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取仿真包" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无仿真包" emptyDescription="当前没有已发布的仿真包。" ariaLabel="学生仿真包列表" />
        </div>
      )}
    </div>
  )
}

export default SimulationsPage
