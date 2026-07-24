// GradeSettingsPage 管理成绩等级映射、学期和学业预警规则。

import React, { useCallback, useEffect, useState } from 'react'
import type { LevelRule } from '@chaimir/api-client'
import { TranscriptScope } from '@chaimir/api-client'
import { Button, Callout, Input, Switch, ResourceState, FormField } from '@chaimir/ui'
import { Plus, RefreshCw, Settings2, Trash2 } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const GradeSettingsPage: React.FC = () => {
  const levels = useAsyncResource(() => api.grade.listLevelConfigs(), [])
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [])
  const warningRules = useAsyncResource(() => api.grade.getWarningRules(), [])
  const [levelName, setLevelName] = useState('')
  const [mapping, setMapping] = useState<LevelRule[]>([
    { min: 90, grade: 'A', gpa: 4 },
    { min: 80, grade: 'B', gpa: 3 },
    { min: 70, grade: 'C', gpa: 2 },
    { min: 60, grade: 'D', gpa: 1 },
    { min: 0, grade: 'F', gpa: 0 },
  ])
  const [failCount, setFailCount] = useState('2')
  const [minGpa, setMinGpa] = useState('2.0')
  const [isDefault, setIsDefault] = useState(false)
  const [semesterName, setSemesterName] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [isCurrent, setIsCurrent] = useState(false)
  const [studentIds, setStudentIds] = useState('')
  const [maintenanceSemesterId, setMaintenanceSemesterId] = useState('')
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (warningRules.data) {
      setFailCount(String(warningRules.data.fail_count))
      setMinGpa(String(warningRules.data.min_gpa))
    }
  }, [warningRules.data])

  useEffect(() => {
    if (levels.status === 'success' || levels.status === 'empty') {
      setIsDefault(!(levels.data || []).some((level) => level.is_default))
    }
  }, [levels.data, levels.status])

  useEffect(() => {
    if (semesters.status === 'success' || semesters.status === 'empty') {
      setIsCurrent(!(semesters.data || []).some((semester) => semester.is_current))
    }
  }, [semesters.data, semesters.status])

  const reloadAll = useCallback(() => {
    levels.reload()
    semesters.reload()
    warningRules.reload()
  }, [levels, semesters, warningRules])

  const runAction = useCallback(async (key: string, action: () => Promise<unknown>, successMessage: string) => {
    setSubmitting(key)
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      reloadAll()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '配置保存失败，请检查内容。'))
    } finally {
      setSubmitting(null)
    }
  }, [reloadAll])

  /** updateLevelRule 更新指定等级行，保持编辑数据始终符合后端结构。 */
  const updateLevelRule = (index: number, patch: Partial<LevelRule>) => {
    setMapping((current) => current.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, ...patch } : rule))
  }

  /** createLevelRule 校验当前表单并保存新的等级规则。 */
  const createLevelRule = () => {
    const normalized = mapping
      .map((rule) => ({ min: Number(rule.min), grade: rule.grade.trim(), gpa: Number(rule.gpa) }))
      .sort((left, right) => right.min - left.min)
    if (!levelName.trim() || normalized.length === 0 || normalized.some((rule) => !rule.grade || rule.min < 0 || rule.min > 100 || rule.gpa < 0 || rule.gpa > 5)) {
      setError('请填写规则名称，并检查分数下限、等级名称和绩点范围。')
      return
    }
    void runAction('level', () => api.grade.createLevelConfig({
      name: levelName.trim(),
      mapping: normalized,
      warning_rules: { fail_count: Number(failCount), min_gpa: Number(minGpa) },
      is_default: isDefault,
    }), '等级映射已创建。')
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Settings2 size={28} />成绩规则定义</h1>
          <p className={styles.subtitle}>维护 GPA 映射、学期和预警规则。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={reloadAll}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      {(levels.error || semesters.error || warningRules.error) && (
        <ResourceState status="error" error={levels.error || semesters.error || warningRules.error} onRetry={reloadAll} />
      )}
      {(levels.status === 'loading' || semesters.status === 'loading' || warningRules.status === 'loading') && <ResourceState status="loading" title="正在获取成绩配置" />}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>新增等级映射</h2>
          <FormField className={styles.field} label="规则名称"><Input fullWidth value={levelName} onChange={(event) => setLevelName(event.target.value)} /></FormField>
          <div className={styles.ruleEditor} aria-label="等级映射">
            {mapping.map((rule, index) => (
              <div className={styles.ruleRow} key={`${index}-${rule.grade}`}>
                <FormField className={styles.field} label="最低分"><Input type="number" min={0} max={100} value={String(rule.min)} onChange={(event) => updateLevelRule(index, { min: Number(event.target.value) })} /></FormField>
                <FormField className={styles.field} label="等级"><Input value={rule.grade} onChange={(event) => updateLevelRule(index, { grade: event.target.value })} /></FormField>
                <FormField className={styles.field} label="绩点"><Input type="number" min={0} max={5} step={0.1} value={String(rule.gpa)} onChange={(event) => updateLevelRule(index, { gpa: Number(event.target.value) })} /></FormField>
                <Button variant="ghost" size="sm" icon={<Trash2 size={14} />} disabled={mapping.length === 1} onClick={() => setMapping((current) => current.filter((_, ruleIndex) => ruleIndex !== index))}>删除</Button>
              </div>
            ))}
            <Button variant="outline" size="sm" icon={<Plus size={14} />} onClick={() => setMapping((current) => [...current, { min: 0, grade: '', gpa: 0 }])}>添加等级</Button>
          </div>
          <Switch checked={isDefault} label="设为默认规则" onChange={(event) => setIsDefault(event.target.checked)} />
          <Button
            loading={submitting === 'level'}
            icon={<Plus size={16} />}
            onClick={createLevelRule}
          >
            保存等级规则
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>新增学期</h2>
          <FormField className={styles.field} label="学期名称"><Input fullWidth value={semesterName} onChange={(event) => setSemesterName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="开始日期"><Input fullWidth type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></FormField>
          <FormField className={styles.field} label="结束日期"><Input fullWidth type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} /></FormField>
          <Switch checked={isCurrent} label="设为当前学期" onChange={(event) => setIsCurrent(event.target.checked)} />
          <Button
            loading={submitting === 'semester'}
            icon={<Plus size={16} />}
            onClick={() => runAction('semester', () => api.grade.createSemester({ name: semesterName, start_date: startDate, end_date: endDate, is_current: isCurrent }), '学期已创建。')}
          >
            保存学期
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>预警规则</h2>
          <FormField className={styles.field} label="挂科门数"><Input fullWidth value={failCount} onChange={(event) => setFailCount(event.target.value)} /></FormField>
          <FormField className={styles.field} label="最低 GPA"><Input fullWidth value={minGpa} onChange={(event) => setMinGpa(event.target.value)} /></FormField>
          <Button
            loading={submitting === 'warning'}
            onClick={() => runAction('warning', () => api.grade.updateWarningRules({ fail_count: Number(failCount), min_gpa: Number(minGpa) }), '预警规则已保存。')}
          >
            保存预警规则
          </Button>
        </section>

        <section className={styles.panel}>
          <h2>当前配置</h2>
          {(levels.data || []).map((level) => (
            <div className={styles.actions} key={level.id}>
              <span className={styles.status}>{level.name}{level.is_default ? '（默认）' : ''}</span>
              {!level.is_default && (
                <Button variant="outline" size="sm" onClick={() => runAction(`level-${level.id}`, () => api.grade.updateLevelConfig(level.id, { name: level.name, mapping: level.mapping, warning_rules: level.warning_rules, is_default: true }), '默认等级规则已更新。')}>设为默认</Button>
              )}
            </div>
          ))}
          <span className={styles.status}>学期 {semesters.data?.length || 0} 个</span>
          <span className={styles.status}>预警: {warningRules.data ? `${warningRules.data.fail_count} 门 / GPA ${warningRules.data.min_gpa}` : '未配置'}</span>
        </section>

        <section className={styles.panel}>
          <h2>成绩维护</h2>
          <FormField className={styles.field} label="学生编号"><Input fullWidth value={studentIds} onChange={(event) => setStudentIds(event.target.value)} placeholder="多个编号用逗号分隔" /></FormField>
          <FormField className={styles.field} label="学期编号"><Input fullWidth value={maintenanceSemesterId} onChange={(event) => setMaintenanceSemesterId(event.target.value)} /></FormField>
          <div className={styles.actions}>
            <Button
              variant="outline"
              disabled={parseStudentIds(studentIds).length !== 1 || !maintenanceSemesterId}
              onClick={() => runAction('recompute', () => api.grade.recomputeStudentGrade(parseStudentIds(studentIds)[0]!, { semester_id: maintenanceSemesterId }), '学生成绩已重新计算。')}
            >重新计算</Button>
            <Button
              disabled={parseStudentIds(studentIds).length === 0}
              onClick={() => runAction('transcript-batch', () => api.grade.generateTranscriptBatch({ student_ids: parseStudentIds(studentIds), scope: maintenanceSemesterId ? TranscriptScope.SEMESTER : TranscriptScope.FULL, semester_id: maintenanceSemesterId || undefined }), '批量成绩单已生成。')}
            >批量生成成绩单</Button>
          </div>
        </section>
      </div>
    </div>
  )
}

export default GradeSettingsPage

/** parseStudentIds 解析并去重学校管理员输入的学生编号。 */
function parseStudentIds(value: string): string[] {
  return Array.from(new Set(value.split(',').map((item) => item.trim()).filter((item) => /^[1-9]\d*$/.test(item))))
}
