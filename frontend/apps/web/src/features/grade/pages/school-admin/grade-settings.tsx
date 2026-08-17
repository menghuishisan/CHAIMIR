// 成绩配置页(校管侧栏,/school-admin/grade-settings)。
//
// 三件配置同页分 Tab(对齐清单 §3.3:等级规则、学期、预警规则应做成配置页内 Tab):
// 等级映射决定百分制怎么换算成等级与绩点,学期决定成绩的归档区间,预警规则决定何时触发学业预警。
//
// 等级映射是有序区间:每条规则声明「不低于某分即为某等级」,后端按 min 降序匹配。
// 故表单要求按分数从高到低,并在保存前校验区间不重叠、覆盖到 0 分。

import { useCallback, useMemo, useState } from 'react'
import {
  CalendarDays,
  CircleCheck,
  GraduationCap,
  Plus,
  Settings2,
  Trash2,
  TriangleAlert,
} from 'lucide-react'
import type { LevelConfig, LevelRule, Semester, WarningRules } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Table,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDate, formatGpa } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/**
 * SchoolAdminGradeSettingsPage 承载等级规则、学期与预警规则三项配置。
 */
export default function SchoolAdminGradeSettingsPage() {
  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '教务与成绩' }, { label: '成绩配置' }]} />}
        title="成绩配置"
        description="等级换算、学期区间与学业预警阈值。这些配置影响全校成绩的计算与预警。"
        icon={Settings2}
      />

      <Tabs defaultValue="levels">
        <TabsList>
          <TabsTrigger value="levels" icon={GraduationCap}>
            等级规则
          </TabsTrigger>
          <TabsTrigger value="semesters" icon={CalendarDays}>
            学期
          </TabsTrigger>
          <TabsTrigger value="warnings" icon={TriangleAlert}>
            预警规则
          </TabsTrigger>
        </TabsList>

        <TabsContent value="levels">
          <LevelConfigsSection />
        </TabsContent>
        <TabsContent value="semesters">
          <SemestersSection />
        </TabsContent>
        <TabsContent value="warnings">
          <WarningRulesSection />
        </TabsContent>
      </Tabs>
    </PageScaffold>
  )
}

/**
 * LevelConfigsSection 维护等级映射配置。
 * 可以有多套配置,其中一套是默认 —— 默认那套用于新课程的成绩换算。
 */
function LevelConfigsSection() {
  const [formTarget, setFormTarget] = useState<{ config?: LevelConfig } | undefined>()

  const configs = useAsyncResource(
    () => api.grade.listLevelConfigs(),
    [],
    (value) => value.length === 0,
  )

  return (
    <PageSection
      title="等级规则"
      description="把百分制分数换算成等级与绩点。默认那套用于新课程。"
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
          新建规则
        </Button>
      }
    >
      <ResourceState
        resource={configs}
        emptyIcon={GraduationCap}
        emptyTitle="还没有等级规则"
        emptyDescription="建一套规则后成绩才能换算成等级与绩点,学生的学分绩点也依赖它。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            新建规则
          </Button>
        }
        skeleton={<Skeleton variant="line" lines={4} />}
      >
        {(list) => (
          <div className="grid gap-4 lg:grid-cols-2">
            {list.map((config) => (
              <LevelConfigCard
                key={config.id}
                config={config}
                onEdit={() => setFormTarget({ config })}
              />
            ))}
          </div>
        )}
      </ResourceState>

      {formTarget ? (
        <LevelConfigFormModal
          config={formTarget.config}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            configs.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface LevelConfigCardProps {
  config: LevelConfig
  onEdit: () => void
}

/**
 * LevelConfigCard 展示一套等级映射。
 */
function LevelConfigCard({ config, onEdit }: LevelConfigCardProps) {
  const sorted = useMemo(() => [...config.mapping].sort((a, b) => b.min - a.min), [config.mapping])

  const columns: TableColumn<LevelRule>[] = [
    { key: 'min', header: '不低于', align: 'right', mono: true },
    { key: 'grade', header: '等级' },
    {
      key: 'gpa',
      header: '绩点',
      align: 'right',
      mono: true,
      render: (rule) => formatGpa(rule.gpa),
    },
  ]

  return (
    <Card>
      <CardHeader
        title={config.name}
        description={`共 ${config.mapping.length} 档`}
        actions={
          <div className="flex items-center gap-2">
            {config.is_default ? <Badge tone="jade">默认</Badge> : null}
            <Button variant="ghost" size="sm" onClick={onEdit}>
              编辑
            </Button>
          </div>
        }
      />
      <CardBody className="flex flex-col gap-3">
        <Table columns={columns} data={sorted} rowKey={(rule) => rule.grade} />
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="neutral">不及格上限 {config.warning_rules.fail_count} 门</Badge>
          <Badge tone="neutral">绩点下限 {formatGpa(config.warning_rules.min_gpa)}</Badge>
        </div>
      </CardBody>
    </Card>
  )
}

interface LevelConfigFormModalProps {
  config?: LevelConfig
  onClose: () => void
  onSaved: () => void
}

/**
 * LevelConfigFormModal 维护一套等级映射。
 * 校验按后端匹配逻辑要求:至少一档、分数区间不重复、最低一档要覆盖到 0 分。
 */
function LevelConfigFormModal({ config, onClose, onSaved }: LevelConfigFormModalProps) {
  const editing = config !== undefined
  const [name, setName] = useState(config?.name ?? '')
  const [isDefault, setIsDefault] = useState(config?.is_default ?? false)
  const [rules, setRules] = useState<LevelRule[]>(
    config ? [...config.mapping].sort((a, b) => b.min - a.min) : defaultRules(),
  )
  const [failCount, setFailCount] = useState(String(config?.warning_rules.fail_count ?? 2))
  const [minGpa, setMinGpa] = useState(String(config?.warning_rules.min_gpa ?? 2))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    if (name.trim() === '') {
      setFormError('请输入规则名称')
      return
    }
    if (rules.length === 0) {
      setFormError('至少要有一档等级')
      return
    }
    if (rules.some((rule) => rule.grade.trim() === '')) {
      setFormError('每一档都要填等级名称')
      return
    }
    const mins = rules.map((rule) => rule.min)
    if (new Set(mins).size !== mins.length) {
      setFormError('有两档的分数下限相同,请调整')
      return
    }
    if (Math.min(...mins) !== 0) {
      setFormError('最低一档的分数下限要是 0,否则低分成绩无法换算')
      return
    }
    const fail = Number(failCount)
    const gpa = Number(minGpa)
    if (!Number.isInteger(fail) || fail < 0) {
      setFormError('不及格门数上限要是 0 或更大的整数')
      return
    }
    if (!Number.isFinite(gpa) || gpa < 0) {
      setFormError('绩点下限要是 0 或更大的数字')
      return
    }

    setFormError(undefined)
    setWorking(true)
    try {
      const payload = {
        name: name.trim(),
        mapping: [...rules].sort((a, b) => b.min - a.min),
        warning_rules: { fail_count: fail, min_gpa: gpa },
        is_default: isDefault,
      }
      if (editing) await api.grade.updateLevelConfig(config.id, payload)
      else await api.grade.createLevelConfig(payload)
      toast.success(editing ? '等级规则已更新' : '等级规则已创建')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [config?.id, editing, failCount, isDefault, minGpa, name, onSaved, rules])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑等级规则' : '新建等级规则'}</ModalTitle>
          <ModalDescription>
            每一档声明「不低于多少分即为某等级」。系统按分数从高到低匹配,最低一档要覆盖到 0 分。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="规则名称" htmlFor="level-name" required>
              <Input id="level-name" value={name} onChange={(event) => setName(event.target.value)} />
            </FormField>
            <FormField label="是否默认" helper="默认规则用于新课程的成绩换算">
              <Checkbox
                checked={isDefault}
                label="设为默认规则"
                onCheckedChange={(checked) => setIsDefault(checked === true)}
              />
            </FormField>
          </div>

          <div className="flex flex-col gap-3 well p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-base text-ink">等级档位</p>
              <Button
                variant="outline"
                size="sm"
                leftIcon={Plus}
                onClick={() => setRules((current) => [...current, { min: 0, grade: '', gpa: 0 }])}
              >
                添加档位
              </Button>
            </div>

            {rules.length === 0 ? (
              <Empty icon={GraduationCap} title="还没有档位" description="至少添加一档。" />
            ) : (
              // 井内的并列条目用分隔线区分,不再各自画盒(规范 §6.5.1)
              <div className="flex flex-col divide-y divide-line">
                {rules.map((rule, index) => (
                  <div key={index} className="flex flex-wrap items-end gap-2 py-3 first:pt-0 last:pb-0">
                    <FormField label="不低于" htmlFor={`rule-min-${index}`} className="mb-0 w-24">
                      <Input
                        id={`rule-min-${index}`}
                        type="number"
                        min="0"
                        max="100"
                        value={String(rule.min)}
                        onChange={(event) =>
                          patchRule(setRules, index, { min: Number(event.target.value) || 0 })
                        }
                      />
                    </FormField>
                    <FormField label="等级" htmlFor={`rule-grade-${index}`} className="mb-0 w-28">
                      <Input
                        id={`rule-grade-${index}`}
                        value={rule.grade}
                        placeholder="优"
                        onChange={(event) => patchRule(setRules, index, { grade: event.target.value })}
                      />
                    </FormField>
                    <FormField label="绩点" htmlFor={`rule-gpa-${index}`} className="mb-0 w-24">
                      <Input
                        id={`rule-gpa-${index}`}
                        type="number"
                        min="0"
                        step="0.1"
                        value={String(rule.gpa)}
                        onChange={(event) =>
                          patchRule(setRules, index, { gpa: Number(event.target.value) || 0 })
                        }
                      />
                    </FormField>
                    <IconButton
                      variant="ghost"
                      size="sm"
                      icon={Trash2}
                      aria-label={`删除第 ${index + 1} 档`}
                      onClick={() => setRules((current) => current.filter((_, i) => i !== index))}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="grid gap-4 well p-4 sm:grid-cols-2">
            <FormField
              label="不及格门数上限"
              htmlFor="level-fail"
              required
              helper="超过这个门数触发学业预警"
            >
              <Input
                id="level-fail"
                type="number"
                min="0"
                value={failCount}
                onChange={(event) => setFailCount(event.target.value)}
              />
            </FormField>
            <FormField
              label="绩点下限"
              htmlFor="level-gpa"
              required
              helper="平均学分绩点低于这个值触发学业预警"
            >
              <Input
                id="level-gpa"
                type="number"
                min="0"
                step="0.1"
                value={minGpa}
                onChange={(event) => setMinGpa(event.target.value)}
              />
            </FormField>
          </div>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            {editing ? '保存规则' : '创建规则'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** defaultRules 给新建规则一个常见的五档起点,管理员可增删。 */
function defaultRules(): LevelRule[] {
  return [
    { min: 90, grade: '优', gpa: 4 },
    { min: 80, grade: '良', gpa: 3 },
    { min: 70, grade: '中', gpa: 2 },
    { min: 60, grade: '及格', gpa: 1 },
    { min: 0, grade: '不及格', gpa: 0 },
  ]
}

/** patchRule 局部更新第 index 档。 */
function patchRule(
  setRules: React.Dispatch<React.SetStateAction<LevelRule[]>>,
  index: number,
  patch: Partial<LevelRule>,
): void {
  setRules((current) => current.map((rule, i) => (i === index ? { ...rule, ...patch } : rule)))
}

/**
 * SemestersSection 维护学期区间。
 * 后端只提供创建与列表(没有更新与删除)—— 学期一旦有成绩归档就不该改动,
 * 故界面也只给「新建」,不做编辑入口。
 */
function SemestersSection() {
  const [createOpen, setCreateOpen] = useState(false)

  const semesters = useAsyncResource(
    () => api.grade.listSemesters(),
    [],
    (value) => value.length === 0,
  )

  const columns: TableColumn<Semester>[] = [
    {
      key: 'name',
      header: '学期',
      render: (semester) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-medium text-ink">{semester.name}</span>
          {semester.is_current ? <Badge tone="jade">当前学期</Badge> : null}
        </div>
      ),
    },
    {
      key: 'start_date',
      header: '开始',
      render: (semester) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(semester.start_date)}
        </span>
      ),
    },
    {
      key: 'end_date',
      header: '结束',
      render: (semester) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(semester.end_date)}
        </span>
      ),
    },
  ]

  return (
    <PageSection
      title="学期"
      description="成绩按学期归档。学期建立后不可修改 —— 已归档的成绩依赖它的区间。"
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
          新建学期
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        <ResourceState
          resource={semesters}
          emptyIcon={CalendarDays}
          emptyTitle="还没有学期"
          emptyDescription="建立学期后教师报送的成绩才能归档到对应学期。"
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
              新建学期
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(list) => <Table columns={columns} data={list} rowKey={(item) => item.id} />}
        </ResourceState>

        <Callout tone="info">
          只有一个学期能是「当前学期」。新建时勾选当前学期会把原来的取消。
        </Callout>
      </div>

      {createOpen ? (
        <SemesterFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            semesters.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface SemesterFormModalProps {
  onClose: () => void
  onSaved: () => void
}

/**
 * SemesterFormModal 新建学期。
 */
function SemesterFormModal({ onClose, onSaved }: SemesterFormModalProps) {
  const [name, setName] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [isCurrent, setIsCurrent] = useState(false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入学期名称')
        return
      }
      if (startDate === '' || endDate === '') {
        setFormError('请选择学期的开始与结束日期')
        return
      }
      if (endDate <= startDate) {
        setFormError('结束日期要晚于开始日期')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.grade.createSemester({
          name: name.trim(),
          start_date: startDate,
          end_date: endDate,
          is_current: isCurrent,
        })
        toast.success('学期已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '创建没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [endDate, isCurrent, name, onSaved, startDate],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>新建学期</ModalTitle>
          <ModalDescription>学期建立后不可修改,请确认名称与区间无误。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="学期名称" htmlFor="semester-name" required helper="如 2026 春季学期">
              <Input
                id="semester-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="开始日期" htmlFor="semester-start" required>
                <Input
                  id="semester-start"
                  type="date"
                  value={startDate}
                  onChange={(event) => setStartDate(event.target.value)}
                />
              </FormField>
              <FormField label="结束日期" htmlFor="semester-end" required>
                <Input
                  id="semester-end"
                  type="date"
                  value={endDate}
                  onChange={(event) => setEndDate(event.target.value)}
                />
              </FormField>
            </div>
            <Checkbox
              checked={isCurrent}
              label="设为当前学期"
              onCheckedChange={(checked) => setIsCurrent(checked === true)}
            />
            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              创建学期
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/**
 * WarningRulesSection 维护全校预警阈值。
 * 这是租户级单例配置(GET/PUT /warning-rules),与等级规则里的阈值是两处不同的东西:
 * 等级规则里的阈值随该套规则,这里是全校默认。
 */
function WarningRulesSection() {
  const rules = useAsyncResource(() => api.grade.getWarningRules(), [], () => false)

  return (
    <PageSection title="预警规则" description="全校的学业预警阈值。达到条件的学生会收到预警通知。">
      <ResourceState
        resource={rules}
        emptyIcon={TriangleAlert}
        emptyTitle="暂无预警规则"
        emptyDescription="设置阈值后扫描才能产生预警。"
        skeleton={<Skeleton variant="line" lines={3} />}
      >
        {(data) => <WarningRulesForm rules={data} onSaved={rules.reload} />}
      </ResourceState>
    </PageSection>
  )
}

interface WarningRulesFormProps {
  rules: WarningRules
  onSaved: () => void
}

/**
 * WarningRulesForm 保存预警阈值。
 */
function WarningRulesForm({ rules, onSaved }: WarningRulesFormProps) {
  const [failCount, setFailCount] = useState(String(rules.fail_count))
  const [minGpa, setMinGpa] = useState(String(rules.min_gpa))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (failCount.trim() === '' || minGpa.trim() === '') {
        setFormError('请填写不及格门数上限和绩点下限')
        return
      }
      const fail = Number(failCount)
      const gpa = Number(minGpa)
      if (!Number.isInteger(fail) || fail < 0) {
        setFormError('不及格门数上限要是 0 或更大的整数')
        return
      }
      if (!Number.isFinite(gpa) || gpa < 0) {
        setFormError('绩点下限要是 0 或更大的数字')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.grade.updateWarningRules({ fail_count: fail, min_gpa: gpa })
        toast.success('预警规则已保存')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [failCount, minGpa, onSaved],
  )

  return (
    <Card>
      <CardHeader title="阈值设置" description="修改后对下一次扫描生效,已产生的预警不受影响。" />
      <CardBody>
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="不及格门数上限"
              htmlFor="warning-fail"
              required
              helper="学生不及格课程超过这个门数即触发预警"
            >
              <Input
                id="warning-fail"
                type="number"
                min="0"
                value={failCount}
                onChange={(event) => setFailCount(event.target.value)}
              />
            </FormField>
            <FormField
              label="绩点下限"
              htmlFor="warning-gpa"
              required
              helper="平均学分绩点低于这个值即触发预警"
            >
              <Input
                id="warning-gpa"
                type="number"
                min="0"
                step="0.1"
                value={minGpa}
                onChange={(event) => setMinGpa(event.target.value)}
              />
            </FormField>
          </div>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <div className="flex items-center gap-2">
            <Button type="submit" variant="primary" leftIcon={CircleCheck} loading={working}>
              保存规则
            </Button>
            <span className="text-sm text-ink-sub">
              保存后到学业预警页执行扫描,规则才会应用到学生。
            </span>
          </div>
        </form>
      </CardBody>
    </Card>
  )
}
