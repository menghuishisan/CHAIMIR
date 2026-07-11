// 平台漏洞源管理页：维护后端漏洞源配置并触发同步。

import React, { useState } from 'react'
import type { VulnProblem, VulnSource } from '@chaimir/api-client'
import { VulnLevel } from '@chaimir/api-client'
import { Button, Checkbox, Input, Select, Table, Textarea } from '@chaimir/ui'
import { DatabaseZap, RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { parseJsonObject, vulnLevelOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const PlatformVulnerabilitiesPage: React.FC = () => {
  const [sourceId, setSourceId] = useState('')
  const [type, setType] = useState(1)
  const [name, setName] = useState('')
  const [configText, setConfigText] = useState('{}')
  const [defaultLevel, setDefaultLevel] = useState(String(VulnLevel.B))
  const [enabled, setEnabled] = useState(true)
  const [synced, setSynced] = useState<VulnProblem[]>([])
  const [message, setMessage] = useState('')
  const resource = useAsyncResource(() => api.contest.listVulnSources(), [])

  const save = async () => {
    if (!name.trim()) return
    setMessage('')
    try {
      await api.contest.upsertVulnSource({
        id: sourceId ? Number(sourceId) : undefined,
        type,
        name: name.trim(),
        config: parseJsonObject(configText),
        default_level: Number(defaultLevel) as VulnLevel,
        enabled,
      })
      setMessage('漏洞源配置已保存。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法保存漏洞源，请检查配置格式。'))
    }
  }

  const sync = async (id: string) => {
    setMessage('')
    try {
      const problems = await api.contest.syncVulnSource(id)
      setSynced(problems)
      setMessage(`同步完成，返回 ${problems.length} 条漏洞题草稿。`)
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法同步漏洞源。'))
    }
  }

  const edit = (source: VulnSource) => {
    setSourceId(source.id)
    setType(source.type)
    setName(source.name)
    setConfigText(JSON.stringify(source.config, null, 2))
    setDefaultLevel(String(source.default_level))
    setEnabled(source.enabled)
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在读取漏洞源" description="系统正在同步平台漏洞源配置。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>平台端 / 漏洞题源管理</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <DatabaseZap className={styles.titleIcon} size={28} />
          漏洞题源管理
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.split}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>漏洞源</h2>
          <Table<VulnSource>
            rows={resource.data ?? []}
            rowKey="id"
            ariaLabel="漏洞源"
            emptyTitle="暂无漏洞源"
            emptyDescription="新增漏洞源后会显示在这里。"
            columns={[
              { key: 'name', title: '名称', dataIndex: 'name', priority: 'primary' },
              { key: 'type', title: '类型', dataIndex: 'type' },
              { key: 'level', title: '默认等级', dataIndex: 'default_level' },
              { key: 'enabled', title: '状态', render: (row) => row.enabled ? '启用' : '停用' },
              {
                key: 'actions',
                title: '操作',
                render: (row) => (
                  <div className={styles.actions}>
                    <Button size="sm" variant="outline" onClick={() => edit(row)}>编辑</Button>
                    <Button size="sm" icon={<RefreshCw size={15} />} onClick={() => sync(row.id)}>同步</Button>
                  </div>
                ),
              },
            ]}
          />
        </section>

        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>源配置</h2>
          <div className={styles.field}><label className={styles.label} htmlFor="source-id">源编号</label><Input id="source-id" value={sourceId} onChange={(event) => setSourceId(event.target.value)} placeholder="新建时留空" fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="source-name">名称</label><Input id="source-name" value={name} onChange={(event) => setName(event.target.value)} fullWidth /></div>
          <div className={styles.grid}>
            <div className={styles.field}><label className={styles.label} htmlFor="source-type">类型</label><Input id="source-type" type="number" value={type} onChange={(event) => setType(Number(event.target.value))} fullWidth /></div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="default-level">默认等级</label>
              <Select id="default-level" value={defaultLevel} options={vulnLevelOptions} onChange={setDefaultLevel} />
            </div>
          </div>
          <div className={styles.field}><label className={styles.label} htmlFor="source-config">配置</label><Textarea id="source-config" className={styles.jsonEditor} value={configText} onChange={(event) => setConfigText(event.target.value)} fullWidth /></div>
          <Checkbox checked={enabled} onChange={(event) => setEnabled(event.target.checked)}>启用漏洞源</Checkbox>
          <Button icon={<Save size={16} />} onClick={save}>保存漏洞源</Button>
        </aside>
      </div>

      {synced.length > 0 && (
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>最近同步结果</h2>
          <Table<VulnProblem>
            rows={synced}
            rowKey="id"
            ariaLabel="同步漏洞题"
            columns={[
              { key: 'title', title: '标题', dataIndex: 'title', priority: 'primary' },
              { key: 'level', title: '等级', dataIndex: 'level' },
              { key: 'status', title: '状态', dataIndex: 'status' },
            ]}
          />
        </section>
      )}
    </div>
  )
}

export default PlatformVulnerabilitiesPage
