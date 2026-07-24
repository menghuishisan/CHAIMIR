// SimulationsPage 展示已发布仿真包，并进入 sim-sdk 工作台。

import React, { useCallback, useMemo, useState } from 'react'
import { SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, FormField, Input, Modal, Select, Table, ResourceState } from '@chaimir/ui'
import { History, Network, Play, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../sim.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const SimulationsPage: React.FC = () => {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [shareCode, setShareCode] = useState('')
  const [selectedPackage, setSelectedPackage] = useState<SimPackageMeta | null>(null)
  const [versions, setVersions] = useState<SimPackageMeta[]>([])
  const [selectedVersion, setSelectedVersion] = useState('')
  const [loadingVersions, setLoadingVersions] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.sim.getPackages({
    keyword: keyword || undefined,
    status: SIM_PACKAGE_STATUS.PUBLISHED,
    page: 1,
    size: 20,
  }), [keyword])

  /**
   * openVersionSelector 读取所选仿真包的已发布版本，再让用户确认启动版本。
   */
  const openVersionSelector = useCallback(async (item: SimPackageMeta) => {
    setSelectedPackage(item)
    setLoadingVersions(true)
    setError(null)
    try {
      const availableVersions = await api.sim.getPackageVersions(item.code)
      setVersions(availableVersions)
      setSelectedVersion(availableVersions.find((version) => version.version === item.version)?.version || availableVersions[0]?.version || '')
    } catch (versionError) {
      setError(userFacingErrorMessage(versionError, '暂时无法获取仿真版本，请稍后重试。'))
      setVersions([])
      setSelectedVersion('')
    } finally {
      setLoadingVersions(false)
    }
  }, [])

  /**
   * closeVersionSelector 关闭版本选择并清理当前临时选择。
   */
  const closeVersionSelector = useCallback(() => {
    setSelectedPackage(null)
    setVersions([])
    setSelectedVersion('')
  }, [])

  const columns = useMemo<TableColumn<SimPackageMeta>[]>(() => [
    { key: 'name', title: '仿真名称', dataIndex: 'name', priority: 'primary' },
    { key: 'category', title: '分类', dataIndex: 'category' },
    { key: 'version', title: '版本', dataIndex: 'version' },
    { key: 'compute', title: '运行方式', dataIndex: 'compute' },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <Button size="sm" icon={<Play size={14} />} onClick={() => void openVersionSelector(row)}>选择版本</Button>,
    },
  ], [openVersionSelector])

  const rows = resource.data?.list || []
  const versionOptions = useMemo(() => versions.map((version) => ({
    value: version.version,
    label: version.version,
  })), [versions])

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
        <FormField label="搜索仿真包" htmlFor="simulation-keyword">
          <Input id="simulation-keyword" placeholder="输入名称或分类" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        </FormField>
        <FormField label="打开分享回放" htmlFor="simulation-share-code">
          <Input id="simulation-share-code" placeholder="输入分享码" value={shareCode} onChange={(event) => setShareCode(event.target.value)} />
        </FormField>
        <Button
          variant="outline"
          icon={<History size={16} />}
          disabled={!shareCode.trim()}
          onClick={() => navigate(`/sim/shared/${encodeURIComponent(shareCode.trim())}`)}
        >
          打开回放
        </Button>
      </div>
      {error && <div className={styles.error} role="alert">{error}</div>}
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取仿真包" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无仿真包" emptyDescription="当前没有已发布的仿真包。" ariaLabel="学生仿真包列表" />
        </div>
      )}
      <Modal
        open={selectedPackage !== null}
        title={selectedPackage ? `选择 ${selectedPackage.name} 的版本` : '选择仿真版本'}
        size="sm"
        onClose={closeVersionSelector}
        footer={(
          <>
            <Button variant="ghost" onClick={closeVersionSelector}>取消</Button>
            <Button
              icon={<Play size={16} />}
              disabled={!selectedPackage || !selectedVersion || loadingVersions}
              onClick={() => {
                if (!selectedPackage || !selectedVersion) return
                navigate(`/student/simulations/${selectedPackage.code}/workspace?version=${encodeURIComponent(selectedVersion)}`)
              }}
            >
              启动工作台
            </Button>
          </>
        )}
      >
        {error && <div className={styles.error} role="alert">{error}</div>}
        <FormField label="仿真版本" htmlFor="simulation-version" helperText="工作台将加载所选版本的已审核内容" required>
          <Select
            id="simulation-version"
            fullWidth
            value={selectedVersion}
            options={versionOptions}
            placeholder={loadingVersions ? '正在获取版本' : '请选择版本'}
            disabled={loadingVersions || versionOptions.length === 0}
            onChange={setSelectedVersion}
          />
        </FormField>
      </Modal>
    </div>
  )
}

export default SimulationsPage
