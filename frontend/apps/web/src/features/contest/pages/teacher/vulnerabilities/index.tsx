// 教师漏洞题列表页：读取漏洞题草稿并执行预验证和固化。

import React, { useState } from 'react'
import type { VulnProblem } from '@chaimir/api-client'
import { Button, Input, Table, ResourceState } from '@chaimir/ui'
import { CheckCircle, ShieldAlert } from 'lucide-react'
import { api } from '../../../../../app/api'
import { usePendingAction } from '../../../../../hooks'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { vulnLevelLabel, vulnPrevalidateStatusLabel, vulnProblemStatusLabel, vulnRuntimeModeLabel } from '../../../../../utils'

const TeacherVulnerabilitiesPage: React.FC = () => {
  const [runtimeCode, setRuntimeCode] = useState('')
  const [runtimeVersion, setRuntimeVersion] = useState('')
  const [toolCodes, setToolCodes] = useState('')
  const [message, setMessage] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const resource = useAsyncResource(() => api.contest.listVulnProblems({ page: 1, size: 30 }), [])

  const prevalidate = async (problemId: string) => {
    if (!runtimeCode.trim() || !runtimeVersion.trim()) {
      setMessage('请先填写预验证运行时和镜像版本。')
      return
    }
    setMessage('')
    try {
      await api.contest.prevalidateVulnProblem(problemId, {
        runtime_code: runtimeCode.trim(),
        runtime_image_version: runtimeVersion.trim(),
        tool_codes: toolCodes.split(',').map((item) => item.trim()).filter(Boolean),
      })
      setMessage('预验证已提交。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法提交预验证。'))
    }
  }

  const finalize = async (problemId: string) => {
    setMessage('')
    try {
      await api.contest.finalizeVulnProblem(problemId)
      setMessage('漏洞题已固化到题库。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法固化漏洞题。'))
    }
  }

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取漏洞题" description="系统正在同步漏洞题草稿。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 漏洞题工坊</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <ShieldAlert className={styles.titleIcon} size={28} />
          漏洞题工坊
        </h1>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <section className={`${styles.panel} ${styles.section}`}>
        <h2 className={styles.sectionTitle}>预验证配置</h2>
        <div className={styles.grid}>
          <div className={styles.field}><label className={styles.label} htmlFor="runtime-code">运行时编号</label><Input id="runtime-code" value={runtimeCode} onChange={(event) => setRuntimeCode(event.target.value)} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="runtime-version">镜像版本</label><Input id="runtime-version" value={runtimeVersion} onChange={(event) => setRuntimeVersion(event.target.value)} fullWidth /></div>
          <div className={styles.field}><label className={styles.label} htmlFor="tool-codes">工具编号</label><Input id="tool-codes" value={toolCodes} onChange={(event) => setToolCodes(event.target.value)} placeholder="多个用逗号分隔" fullWidth /></div>
        </div>
      </section>

      <Table<VulnProblem>
        rows={resource.data?.list ?? []}
        rowKey="id"
        ariaLabel="漏洞题草稿"
        emptyTitle="暂无漏洞题"
        emptyDescription="导入漏洞题后会显示在这里。"
        columns={[
          { key: 'title', title: '标题', dataIndex: 'title', priority: 'primary' },
          { key: 'level', title: '等级', render: (row) => vulnLevelLabel(row.level) },
          { key: 'runtime', title: '运行方式', render: (row) => vulnRuntimeModeLabel(row.runtime_mode) },
          { key: 'prevalidate', title: '预验证', render: (row) => vulnPrevalidateStatusLabel(row.prevalidate_status) },
          { key: 'status', title: '状态', render: (row) => vulnProblemStatusLabel(row.status) },
          {
            key: 'actions',
            title: '操作',
            render: (row) => (
              <div className={styles.actions}>
                <Button size="sm" variant="outline" loading={pendingAction === `validate-${row.id}`} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction(`validate-${row.id}`, () => prevalidate(row.id))}>预验证</Button>
                <Button size="sm" icon={<CheckCircle size={15} />} loading={pendingAction === `finalize-${row.id}`} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction(`finalize-${row.id}`, () => finalize(row.id))}>固化</Button>
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}

export default TeacherVulnerabilitiesPage
