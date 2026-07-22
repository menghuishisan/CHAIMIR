// AlertRulesPage 管理平台告警规则，读取并更新 admin 告警规则接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { AlertRule } from '@chaimir/api-client'
import { AdminScope } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Switch, Table, Textarea } from '@chaimir/ui'
import { ListChecks, RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import formStyles from './rules.module.css'
import { alertLevelOptions, parseJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const AlertRulesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.admin.listAlertRules({ scope: AdminScope.GLOBAL }), [])
  const [selectedRuleId, setSelectedRuleId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [metric, setMetric] = useState('')
  const [level, setLevel] = useState('1')
  const [enabled, setEnabled] = useState(true)
  const [condition, setCondition] = useState('{\n  "operator": "gt",\n  "threshold": 90,\n  "duration_minutes": 5\n}')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * fillForm 把后端规则填充到编辑表单。
   */
  const fillForm = useCallback((rule: AlertRule) => {
    setSelectedRuleId(rule.id)
    setName(rule.name)
    setMetric(rule.metric)
    setLevel(String(rule.level))
    setEnabled(rule.enabled)
    setCondition(JSON.stringify(rule.condition, null, 2))
    setError(null)
    setMessage(null)
  }, [])

  /**
   * handleSubmit 创建或更新告警规则。
   */
  const handleSubmit = useCallback(async () => {
    if (!name.trim() || !metric.trim()) {
      setError('请填写规则名称和指标名称。')
      return
    }

    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        scope: AdminScope.GLOBAL,
        name: name.trim(),
        metric: metric.trim(),
        condition: parseJsonObject(condition),
        level: Number(level),
        enabled,
      }
      if (selectedRuleId) {
        await api.admin.updateAlertRule(selectedRuleId, payload)
        setMessage('告警规则已更新。')
      } else {
        await api.admin.createAlertRule(payload)
        setMessage('告警规则已创建。')
      }
      resource.reload()
    } catch (submitError) {
      setError(userFacingErrorMessage(submitError, '告警规则保存失败，请检查配置。'))
    } finally {
      setSubmitting(false)
    }
  }, [condition, enabled, level, metric, name, resource, selectedRuleId])

  const columns = useMemo<TableColumn<AlertRule>[]>(() => [
    { key: 'name', title: '规则名称', dataIndex: 'name', priority: 'primary' },
    { key: 'metric', title: '指标', dataIndex: 'metric', priority: 'secondary' },
    {
      key: 'level',
      title: '级别',
      render: (row) => <span className={styles.status}>L{row.level}</span>,
    },
    {
      key: 'enabled',
      title: '状态',
      render: (row) => (row.enabled ? '已启用' : '已停用'),
    },
    {
      key: 'action',
      title: '操作',
      render: (row) => (
        <Button variant="ghost" size="sm" onClick={() => fillForm(row)}>
          编辑
        </Button>
      ),
    },
  ], [fillForm])

  const rows = resource.data || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <ListChecks className={styles.icon} size={28} />
            告警规则
          </h1>
          <p className={styles.subtitle}>维护平台级告警触发条件和通知级别。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {error && <div className={formStyles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="保存成功">
          {message}
        </Callout>
      )}

      <div className={formStyles.grid}>
        <section className={formStyles.panel}>
          <h2>{selectedRuleId ? '编辑规则' : '新建规则'}</h2>
          <label>
            规则名称
            <Input fullWidth value={name} placeholder="请输入规则名称" onChange={(event) => setName(event.target.value)} />
          </label>
          <label>
            指标名称
            <Input fullWidth value={metric} placeholder="例如 sandbox.cpu_usage" onChange={(event) => setMetric(event.target.value)} />
          </label>
          <label>
            告警级别
            <Select fullWidth value={level} options={alertLevelOptions} onChange={setLevel} />
          </label>
          <Switch checked={enabled} label={enabled ? '已启用' : '已停用'} onChange={(event) => setEnabled(event.target.checked)} />
          <label>
            触发条件
            <Textarea value={condition} onChange={(event) => setCondition(event.target.value)} />
          </label>
          <Button loading={submitting} icon={<Save size={16} />} onClick={handleSubmit}>
            保存规则
          </Button>
        </section>

        <section className={formStyles.panel}>
          <h2>规则列表</h2>
          {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
          {resource.status === 'loading' && <LoadingState title="正在获取告警规则" />}
          {(resource.status === 'success' || resource.status === 'empty') && (
            <Table
              columns={columns}
              rows={rows}
              rowKey="id"
              emptyTitle="暂无规则"
              emptyDescription="当前还没有平台级告警规则。"
              ariaLabel="告警规则列表"
            />
          )}
        </section>
      </div>
    </div>
  )
}

export default AlertRulesPage
