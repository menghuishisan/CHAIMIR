// 教师实验编排页：把向导状态保存到后端实验定义，保证刷新和跨设备不丢失。

import React, { useEffect, useMemo, useState } from 'react'
import type { ComponentConfig, Experiment, ExperimentGroup, ExperimentRequest, GroupConfig } from '@chaimir/api-client'
import { ExperimentCollabMode } from '@chaimir/api-client'
import { Button, Checkbox, Input, Select, Textarea } from '@chaimir/ui'
import { Compass, Save, Send, Users } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import { emptyExperimentComponents, defaultExperimentGroup } from '../../../config/orchestration'
import styles from '../../experiment.module.css'
import { experimentCollabModeOptions, parseJsonObject, stringifyJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherExperimentOrchestrationPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams()
  const id = searchParams.get('id')
  const [form, setForm] = useState<ExperimentRequest>({
    course_id: '',
    template_ref: '',
    template_version: '',
    name: '',
    description: '',
    components: emptyExperimentComponents,
    collab_mode: ExperimentCollabMode.SOLO,
    group_config: defaultExperimentGroup,
    require_report: true,
    wizard_step: 1,
  })
  const [componentsText, setComponentsText] = useState(stringifyJsonObject(emptyExperimentComponents))
  const [groupText, setGroupText] = useState(stringifyJsonObject(defaultExperimentGroup))
  const [message, setMessage] = useState('')
  const [groupName, setGroupName] = useState('')
  const [groupId, setGroupId] = useState('')
  const [studentId, setStudentId] = useState('')
  const [memberRole, setMemberRole] = useState('member')
  const [group, setGroup] = useState<ExperimentGroup>()

  const resource = useAsyncResource(
    async () => {
      if (!id) return null
      const response = await api.experiment.getExperiments({ page: 1, size: 100 })
      return response.list.find((item) => item.id === id) ?? null
    },
    [id],
    () => false
  )

  useEffect(() => {
    if (!resource.data) return
    const experiment = resource.data
    setForm(toRequest(experiment))
    setComponentsText(stringifyJsonObject(experiment.components))
    setGroupText(stringifyJsonObject(experiment.group_config))
  }, [resource.data])

  const parsed = useMemo(() => {
    try {
      return {
        components: parseJsonObject<ComponentConfig>(componentsText),
        group: parseJsonObject<GroupConfig>(groupText),
        ok: true,
      }
    } catch {
      return { components: emptyExperimentComponents, group: defaultExperimentGroup, ok: false }
    }
  }, [componentsText, groupText])

  const save = async () => {
    if (!parsed.ok) {
      setMessage('组件配置或协作配置不是有效的结构，请检查后再保存。')
      return
    }
    setMessage('')
    const payload = { ...form, components: parsed.components, group_config: parsed.group }
    try {
      const saved = id ? await api.experiment.updateExperiment(id, payload) : await api.experiment.createExperiment(payload)
      setSearchParams({ id: saved.id }, { replace: true })
      setMessage('实验编排已保存到服务端。')
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法保存实验编排。'))
    }
  }

  const publish = async () => {
    if (!id) {
      setMessage('请先保存实验编排，再执行发布。')
      return
    }
    setMessage('')
    try {
      const result = await api.experiment.validateExperiment(id)
      if (!result.ok) {
        setMessage(result.issues.map((issue) => issue.message).join('；') || '实验配置未通过校验。')
        return
      }
      await api.experiment.publishExperiment(id)
      setMessage('实验已发布，学生端可进入。')
      resource.reload()
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '暂时无法发布实验。'))
    }
  }

  /** createGroup 为协作实验创建服务端小组并读取完整小组状态。 */
  const createGroup = async () => {
    if (!id || !groupName.trim()) return
    setMessage('')
    try {
      const created = await api.experiment.createGroup(id, { name: groupName.trim() })
      setGroupId(created.id)
      setGroup(await api.experiment.getGroup(created.id))
      setMessage('实验小组已创建。')
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '实验小组创建失败，请稍后重试。'))
    }
  }

  /** addGroupMember 把学生加入小组并刷新服务端权威成员列表。 */
  const addGroupMember = async () => {
    if (!groupId.trim() || !studentId.trim()) return
    setMessage('')
    try {
      await api.experiment.upsertGroupMember(groupId.trim(), { student_id: studentId.trim(), role: memberRole.trim() || 'member' })
      setGroup(await api.experiment.getGroup(groupId.trim()))
      setMessage('小组成员已保存。')
    } catch (error) {
      setMessage(userFacingErrorMessage(error, '小组成员保存失败，请检查学生编号后重试。'))
    }
  }

  if (id && resource.status === 'loading') {
    return <LoadingState title="正在读取实验编排" description="系统正在同步已保存的向导配置。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  if (id && resource.status === 'empty') {
    return <EmptyState title="未找到实验编排" description="该实验可能已被删除或你没有访问权限。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>教师端 / 实验实训编排 / 编排向导</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <Compass className={styles.titleIcon} size={28} />
          实验流编排
        </h1>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Save size={16} />} onClick={save}>保存编排</Button>
          <Button icon={<Send size={16} />} onClick={publish}>校验并发布</Button>
        </div>
      </div>
      {message && <p className={styles.message} role="status">{message}</p>}

      <div className={styles.orchestration}>
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>基础信息</h2>
          <div className={styles.formGrid}>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="experiment-name">实验名称</label>
              <Input id="experiment-name" value={form.name} onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} fullWidth />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="course-id">课程编号</label>
              <Input id="course-id" inputMode="numeric" value={form.course_id} onChange={(event) => setForm((current) => ({ ...current, course_id: event.target.value }))} fullWidth />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="template-ref">模板引用</label>
              <Input id="template-ref" value={form.template_ref} onChange={(event) => setForm((current) => ({ ...current, template_ref: event.target.value }))} fullWidth />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="template-version">模板版本</label>
              <Input id="template-version" value={form.template_version} onChange={(event) => setForm((current) => ({ ...current, template_version: event.target.value }))} fullWidth />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="collab-mode">协作模式</label>
              <Select
                id="collab-mode"
                value={String(form.collab_mode)}
                options={experimentCollabModeOptions}
                onChange={(value) => setForm((current) => ({ ...current, collab_mode: Number(value) as ExperimentCollabMode }))}
              />
            </div>
            <div className={styles.field}>
              <label className={styles.label} htmlFor="wizard-step">当前步骤</label>
              <Input id="wizard-step" type="number" min={1} value={form.wizard_step} onChange={(event) => setForm((current) => ({ ...current, wizard_step: Number(event.target.value) }))} fullWidth />
            </div>
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="description">实验说明</label>
            <Textarea id="description" rows={5} value={form.description} onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))} fullWidth />
          </div>
          <Checkbox label="学生需要提交实验报告" checked={form.require_report} onChange={(event) => setForm((current) => ({ ...current, require_report: event.target.checked }))} />
        </section>

        <aside className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}>组件与协作配置</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="components">实验组件配置</label>
            <Textarea id="components" className={styles.jsonEditor} value={componentsText} onChange={(event) => setComponentsText(event.target.value)} resize="vertical" fullWidth />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="group-config">协作配置</label>
            <Textarea id="group-config" className={styles.jsonEditor} value={groupText} onChange={(event) => setGroupText(event.target.value)} resize="vertical" fullWidth />
          </div>
          <p className={styles.muted}>组件配置将用于创建实验环境、解锁阶段、检查点判分和绑定仿真会话。</p>
        </aside>
      </div>
      {id && form.collab_mode !== ExperimentCollabMode.SOLO && (
        <section className={`${styles.panel} ${styles.section}`}>
          <h2 className={styles.sectionTitle}><Users size={18} />协作小组</h2>
          <div className={styles.formGrid}>
            <label className={styles.field}>小组名称<Input fullWidth value={groupName} onChange={(event) => setGroupName(event.target.value)} /></label>
            <label className={styles.field}>小组编号<Input fullWidth value={groupId} onChange={(event) => setGroupId(event.target.value)} /></label>
            <label className={styles.field}>学生编号<Input fullWidth value={studentId} onChange={(event) => setStudentId(event.target.value)} /></label>
            <label className={styles.field}>小组角色<Input fullWidth value={memberRole} onChange={(event) => setMemberRole(event.target.value)} /></label>
          </div>
          <div className={styles.actions}>
            <Button variant="outline" onClick={() => void createGroup()}>创建小组</Button>
            <Button onClick={() => void addGroupMember()}>保存成员</Button>
            <Button variant="ghost" disabled={!groupId.trim()} onClick={() => void api.experiment.getGroup(groupId.trim()).then(setGroup).catch((error) => setMessage(userFacingErrorMessage(error, '暂时无法读取小组。')))}>刷新小组</Button>
          </div>
          {group && <p className={styles.muted}>{group.name}，当前 {group.members.length} 名成员。</p>}
        </section>
      )}
    </div>
  )
}

/** toRequest 将服务端实验详情转换为编辑表单可提交的请求结构。 */
function toRequest(experiment: Experiment): ExperimentRequest {
  return {
    course_id: experiment.course_id ?? '',
    template_ref: experiment.template_ref ?? '',
    template_version: experiment.template_version ?? '',
    name: experiment.name,
    description: experiment.description,
    components: experiment.components,
    collab_mode: experiment.collab_mode,
    group_config: experiment.group_config,
    require_report: experiment.require_report,
    wizard_step: experiment.wizard_step,
  }
}

export default TeacherExperimentOrchestrationPage
