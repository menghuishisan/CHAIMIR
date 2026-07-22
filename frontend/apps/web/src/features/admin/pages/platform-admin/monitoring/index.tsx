// MonitoringPage 展示平台外接监控面板，数据来自 admin 监控面板接口。

import React, { useMemo, useState } from 'react'
import type { MonitoringPanel } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { Monitor, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from './monitoring.module.css'

/**
 * MonitoringPage 读取可嵌入监控面板并展示当前选中面板。
 */
const MonitoringPage: React.FC = () => {
  const resource = useAsyncResource(() => api.admin.monitoringPanels(), [])
  const panels = useMemo(() => resource.data || [], [resource.data])
  const [selectedName, setSelectedName] = useState<string | null>(null)
  const selectedPanel: MonitoringPanel | undefined = panels.find((panel) => panel.name === selectedName) || panels[0]

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Monitor className={styles.icon} size={28} />
            平台监控看板
          </h1>
          <p className={styles.subtitle}>查看已授权接入的平台监控面板。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'loading' && <LoadingState title="正在获取监控面板" />}
      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <EmptyState title="暂无监控面板" description="当前平台没有可嵌入的监控面板。" />
      )}
      {resource.status === 'success' && selectedPanel && (
        <>
          <div className={styles.panelTabs} aria-label="监控面板">
            {panels.map((panel) => (
              <button
                className={`${styles.panelTab} ${panel.name === selectedPanel.name ? styles.panelTabActive : ''}`}
                key={panel.name}
                type="button"
                onClick={() => setSelectedName(panel.name)}
              >
                {panel.name}
              </button>
            ))}
          </div>
          <iframe
            className={styles.frame}
            src={selectedPanel.url}
            title={selectedPanel.name}
            sandbox="allow-scripts allow-same-origin allow-forms"
          />
        </>
      )}
    </div>
  )
}

export default MonitoringPage
