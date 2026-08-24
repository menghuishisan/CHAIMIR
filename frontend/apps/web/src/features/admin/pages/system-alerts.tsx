// 告警页(校管侧栏「学校告警」/ 平台侧栏「告警中心」共用一个实现)。
//
// 告警事件与规则同页分 Tab(对齐清单 §3.3/§3.4:规则维护放在告警页内 Tab,不单独放侧栏)。
// 事件是「已经发生的」,规则是「什么条件下发生」—— 处理事件时常要顺手调规则,
// 拆成两页会让管理员来回跳。
//
// 校管与平台看的是同一套后端接口(M9 的 mixed 组),分叉只在作用范围:
// 平台身份只能读写 scope=平台全局的规则、看到全平台事件;校管固定 scope=本校、只看本校事件
// (后端 ListAlertRules/ListAlertEvents 按身份分叉,平台传非全局 scope 会被拒)。
// 故这里由调用方通过 scope 显式声明,不复制第二份页面,也不在运行时判角色枚举。
//
// 规则的 condition 是 JSONB,按「比较方式 + 阈值 + 持续时间」三个显式字段组装;
// 指标从已登记清单里选,不让管理员手写指标名(写错要等到告警不触发才发现)。

import { useCallback, useState } from 'react'
import { BellRing, CircleCheck, CircleSlash, Plus, Settings2 } from 'lucide-react'
import { AdminScope, AlertLevel, AlertStatus, type AlertEvent, type AlertRule } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  DataPanel,
  DescriptionList,
  EventTimeline,
  FilterBar,
  FilterField,
  FormField,
  Input,
  MetricStrip,
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
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
  type EventTone,
  type TimelineDay,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../hooks'
import { formatDate, formatDateTime, formatTime } from '../../../utils/formatters'
import {
  alertConditionOperatorLabel,
  alertLevelLabel,
  alertMetricLabel,
  alertStatusLabel,
} from '../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { ALERT_CONDITION_OPERATORS, ALERT_LEVELS, ALERT_METRICS } from '../options'

/** 告警条件里的结构化键。 */
const CONDITION_FIELDS = {
  operator: 'operator',
  threshold: 'threshold',
  durationSeconds: 'duration_seconds',
} as const

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(AlertStatus.PENDING), label: '待处理' },
  { value: String(AlertStatus.HANDLED), label: '已处理' },
  { value: String(AlertStatus.IGNORED), label: '已忽略' },
] as const

/**
 * LEVEL_FILTERS 是告警级别筛选项:值为空串表示不限。
 * 后端 listAlertEvents 的 level 以「不传」表示不限,故空串换算成 undefined。
 */
const LEVEL_FILTERS = [
  { value: '', label: '全部级别' },
  ...ALERT_LEVELS.map((level) => ({ value: String(level), label: alertLevelLabel(level) })),
] as const

/** 两种作用范围的页面文案:范围决定了事件覆盖面与规则归属,故各自说清。 */
const SCOPE_COPY: Record<
  AdminScope,
  {
    group: string
    title: string
    description: string
    eventScope: string
    ruleDescription: string
    ruleEmptyDescription: string
  }
> = {
  [AdminScope.GLOBAL]: {
    group: '底层资源',
    title: '告警中心',
    description: '全平台的资源与业务异常告警。处理完标记为已处理,不需要跟进的可以忽略。',
    eventScope: '全平台',
    ruleDescription: '平台级规则对所有学校生效。学校自己还能加本校规则,两者互不覆盖。',
    ruleEmptyDescription: '建立平台级规则后,任一学校的指标异常都会产生告警。',
  },
  [AdminScope.TENANT]: {
    group: '系统配置',
    title: '学校告警',
    description: '本校范围的资源与业务异常告警。处理完标记为已处理,不需要跟进的可以忽略。',
    eventScope: '本校',
    ruleDescription: '指标超过阈值并持续一段时间才触发告警,避免瞬时波动造成误报。',
    ruleEmptyDescription: '建立规则后,指标异常时会自动产生告警。平台级规则由平台管理员维护。',
  },
}

export interface SystemAlertsPageProps {
  /**
   * 告警作用范围。
   * 平台端传平台全局,校管端传本校 —— 后端按身份强制这一分叉(平台传本校 scope 会被拒),
   * 前端按声明渲染,不判角色枚举。
   */
  scope: AdminScope
}

/**
 * SystemAlertsPage 承载告警事件处理与规则维护。
 */
export default function SystemAlertsPage({ scope }: SystemAlertsPageProps) {
  const copy = SCOPE_COPY[scope]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: copy.group }]} />}
        title={copy.title}
        description={copy.description}
        icon={BellRing}
      />

      <Tabs defaultValue="events">
        <TabsList>
          <TabsTrigger value="events" icon={BellRing}>
            告警事件
          </TabsTrigger>
          <TabsTrigger value="rules" icon={Settings2}>
            告警规则
          </TabsTrigger>
        </TabsList>

        <TabsContent value="events">
          <AlertEventsSection eventScope={copy.eventScope} />
        </TabsContent>
        <TabsContent value="rules">
          <AlertRulesSection
            scope={scope}
            description={copy.ruleDescription}
            emptyDescription={copy.ruleEmptyDescription}
          />
        </TabsContent>
      </Tabs>
    </PageScaffold>
  )
}

/**
 * AlertEventsSection 列出告警事件并承载处理与忽略。
 * 事件覆盖面由服务端身份决定(平台看全平台、校管看本校),故这里只用文案说明范围。
 */
function AlertEventsSection({ eventScope }: { eventScope: string }) {
  const [statusFilter, setStatusFilter] = useState<string>(String(AlertStatus.PENDING))
  const [levelFilter, setLevelFilter] = useState<string>('')
  const [target, setTarget] = useState<{ event: AlertEvent; status: AlertStatus }>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const events = usePagedResource<AlertEvent>(
    (params) =>
      api.admin.listAlertEvents({
        status: statusFilter ? (Number(statusFilter) as AlertStatus) : undefined,
        level: levelFilter ? (Number(levelFilter) as AlertLevel) : undefined,
        ...params,
      }),
    [levelFilter, statusFilter]
  )

  const handle = useCallback(async () => {
    if (!target) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.admin.handleAlertEvent(target.event.id, { status: target.status })
      toast.success(target.status === AlertStatus.HANDLED ? '已标记为已处理' : '已忽略这条告警')
      setTarget(undefined)
      events.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [events, target])

  // 指标带取服务端全量口径,不随下方筛选变化(§6.5.4)。
  // 「待处理紧急」现在也走服务端 level 参数,不再用当前页切片数 —— 那是错数。
  const totalCount = useResourceTotal((params) => api.admin.listAlertEvents(params), [])
  const pendingCount = useResourceTotal(
    (params) => api.admin.listAlertEvents({ status: AlertStatus.PENDING, ...params }),
    []
  )
  const handledCount = useResourceTotal(
    (params) => api.admin.listAlertEvents({ status: AlertStatus.HANDLED, ...params }),
    []
  )
  const urgentPendingCount = useResourceTotal(
    (params) =>
      api.admin.listAlertEvents({
        status: AlertStatus.PENDING,
        level: AlertLevel.CRITICAL,
        ...params,
      }),
    []
  )

  /**
   * alertTimelineTone 把告警映射成事件轴的状态点语义。
   * 已处理/已忽略的一律退成 normal(灰点):它们已经不需要注意力,
   * 仍按级别染色会让一屏红点里分不出「还没处理的」是哪几条。
   */
  const alertTimelineTone = (event: AlertEvent): EventTone => {
    if (event.status !== AlertStatus.PENDING) return 'normal'
    return event.level === AlertLevel.NOTICE ? 'normal' : event.level === AlertLevel.WARNING ? 'warning' : 'danger'
  }

  /**
   * groupByDay 把告警按天分组成事件轴的输入(§6.5.3 第 ⑥ 族)。
   * 告警的读法是「最近出了什么、还有哪些没处理」,时间序是主轴;
   * 五列等宽表格把级别、内容、时间、状态、动作摊平,反而看不出「哪天集中爆了一批」。
   * 级别映射到状态点的形状 + 色(FE-2 双通道),级别文字仍在细节行里给出。
   */
  const groupByDay = (list: AlertEvent[]): TimelineDay[] => {
    const today = new Date()
    const dayKey = (value: Date) => `${value.getFullYear()}-${value.getMonth()}-${value.getDate()}`
    const todayKey = dayKey(today)
    const yesterday = new Date(today)
    yesterday.setDate(today.getDate() - 1)
    const yesterdayKey = dayKey(yesterday)

    const days: TimelineDay[] = []
    for (const event of list) {
      const at = new Date(event.triggered_at)
      const key = dayKey(at)
      const label =
        key === todayKey
          ? `今天 · ${formatDate(event.triggered_at)}`
          : key === yesterdayKey
            ? `昨天 · ${formatDate(event.triggered_at)}`
            : formatDate(event.triggered_at)
      let day = days.find((entry) => entry.label === label)
      if (!day) {
        day = { label, events: [] }
        days.push(day)
      }
      day.events.push({
        id: event.id,
        time: formatTime(event.triggered_at),
        tone: alertTimelineTone(event),
        title: event.message,
        detail: `${alertLevelLabel(event.level)} · ${alertStatusLabel(event.status)}${
          event.handled_at ? ` · 处理于 ${formatDateTime(event.handled_at)}` : ''
        }`,
        action:
          event.status === AlertStatus.PENDING ? (
            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="sm"
                leftIcon={CircleCheck}
                onClick={() => setTarget({ event, status: AlertStatus.HANDLED })}
              >
                已处理
              </Button>
              <Button
                variant="ghost"
                size="sm"
                leftIcon={CircleSlash}
                onClick={() => setTarget({ event, status: AlertStatus.IGNORED })}
              >
                忽略
              </Button>
            </div>
          ) : undefined,
      })
    }
    return days
  }

  return (
    <>
      {/* 指标降为内联摘要(§6.5.3 第 ① 族口径):本页主体是告警事件流 */}
      <MetricStrip
        label="告警总量摘要"
        className="mb-5"
        items={[
          { label: '告警总数', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '待处理', value: pendingCount ?? '—', hint: '需要跟进' },
          { label: '待处理特急', value: urgentPendingCount ?? '—', hint: '优先处理这些' },
          { label: '已处理', value: handledCount ?? '—', hint: '含已忽略' },
        ]}
      />

      {/* 动作失败就近内联(§6.7 C) */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 时间流族:筛选井 + 按天事件轴 + 分页同处一块抬起片(§6.5.3 第 ⑥ 族) */}
      <DataPanel
        label="告警事件"
        filter={
          <FilterBar label="告警事件筛选">
            <FilterField label="处理状态" group>
              <SegmentedControl
                aria-label="按处理状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="告警级别" group>
              <SegmentedControl
                aria-label="按告警级别筛选"
                size="sm"
                options={LEVEL_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={levelFilter}
                onValueChange={setLevelFilter}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={events.page}
            pageSize={events.pageSize}
            total={events.total}
            onPageChange={events.setPage}
          />
        }
      >
        <ResourceState
          resource={events}
          emptyIcon={BellRing}
          emptyTitle={statusFilter || levelFilter ? '这个筛选下没有告警' : '暂无告警'}
          emptyDescription={
            statusFilter || levelFilter
              ? '换个状态或级别看看。'
              : '资源或业务指标超过规则阈值时会产生告警。'
          }
          skeleton={<Skeleton variant="line" lines={6} />}
        >
          {(page) => <EventTimeline label={`告警事件,范围为${eventScope}`} days={groupByDay(page.list)} />}
        </ResourceState>
      </DataPanel>

      <Modal open={target !== undefined} onOpenChange={(open) => !open && setTarget(undefined)}>
        <ModalContent size="sm">
          {target ? (
            <>
              <ModalHeader>
                <ModalTitle>
                  {target.status === AlertStatus.HANDLED ? '标记为已处理' : '忽略这条告警'}
                </ModalTitle>
                <ModalDescription>
                  {target.status === AlertStatus.HANDLED
                    ? '确认这个问题已经处理完毕。同样的条件再次触发会产生新的告警。'
                    : '忽略表示这次不需要跟进。规则不变,下次触发仍会产生告警。'}
                </ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base text-ink">{target.event.message}</p>
              </ModalBody>
              <ModalFooter>
                <Button variant="outline" onClick={() => setTarget(undefined)}>
                  取消
                </Button>
                <Button variant="primary" loading={working} onClick={() => void handle()}>
                  确认
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </>
  )
}

interface AlertRulesSectionProps {
  scope: AdminScope
  description: string
  emptyDescription: string
}

/**
 * AlertRulesSection 维护告警规则。
 * scope 由页面按端声明:平台维护全局规则,校管维护本校规则 ——
 * 后端强制这一分叉,平台传本校 scope 会被拒,校管的 scope 一律被改写成本校。
 */
function AlertRulesSection({ scope, description, emptyDescription }: AlertRulesSectionProps) {
  const [formTarget, setFormTarget] = useState<{ rule?: AlertRule } | undefined>()

  const rules = useAsyncResource(
    () => api.admin.listAlertRules({ scope }),
    [scope],
    (value) => value.length === 0
  )

  return (
    <PageSection
      title="告警规则"
      description={description}
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
          新建规则
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        <ResourceState
          resource={rules}
          emptyIcon={Settings2}
          emptyTitle="还没有告警规则"
          emptyDescription={emptyDescription}
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
              新建规则
            </Button>
          }
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(list) => (
            <div className="grid gap-4 lg:grid-cols-2">
              {list.map((rule) => (
                <AlertRuleCard key={rule.id} rule={rule} onEdit={() => setFormTarget({ rule })} />
              ))}
            </div>
          )}
        </ResourceState>

        <Callout tone="info">
          规则改动对下一次评估生效,已产生的告警不受影响。停用规则不会删除历史告警。
        </Callout>
      </div>

      {formTarget ? (
        <AlertRuleFormModal
          rule={formTarget.rule}
          scope={scope}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            rules.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface AlertRuleCardProps {
  rule: AlertRule
  onEdit: () => void
}

/**
 * AlertRuleCard 展示单条规则的可读描述。
 */
function AlertRuleCard({ rule, onEdit }: AlertRuleCardProps) {
  const operator = readString(rule.condition, CONDITION_FIELDS.operator)
  const threshold = readNumber(rule.condition, CONDITION_FIELDS.threshold)
  const duration = readNumber(rule.condition, CONDITION_FIELDS.durationSeconds)

  return (
    <Card>
      <CardHeader
        title={rule.name}
        description={alertMetricLabel(rule.metric)}
        actions={
          <div className="flex items-center gap-2">
            {rule.enabled ? (
              <Badge tone="success">已启用</Badge>
            ) : (
              <Badge tone="neutral">已停用</Badge>
            )}
            <Button variant="ghost" size="sm" onClick={onEdit}>
              编辑
            </Button>
          </div>
        }
      />
      <CardBody>
        <DescriptionList
          dense
          items={[
            {
              term: '触发条件',
              description: operator
                ? `${alertMetricLabel(rule.metric)} ${alertConditionOperatorLabel(operator)} ${threshold}`
                : '条件未完整配置',
            },
            {
              term: '持续时间',
              description: duration > 0 ? `${duration} 秒` : '立即触发',
            },
            { term: '告警级别', description: alertLevelLabel(rule.level) },
          ]}
        />
      </CardBody>
    </Card>
  )
}

interface AlertRuleFormModalProps {
  rule?: AlertRule
  /** 规则作用范围:与所在页面一致,提交时原样带上(后端按身份最终裁定) */
  scope: AdminScope
  onClose: () => void
  onSaved: () => void
}

/**
 * AlertRuleFormModal 维护一条告警规则。
 */
function AlertRuleFormModal({ rule, scope, onClose, onSaved }: AlertRuleFormModalProps) {
  const editing = rule !== undefined
  const [name, setName] = useState(rule?.name ?? '')
  const [metric, setMetric] = useState(rule?.metric ?? ALERT_METRICS[0])
  const [operator, setOperator] = useState(
    readString(rule?.condition, CONDITION_FIELDS.operator) || ALERT_CONDITION_OPERATORS[0]
  )
  const [threshold, setThreshold] = useState(
    String(readNumber(rule?.condition, CONDITION_FIELDS.threshold))
  )
  const [duration, setDuration] = useState(
    String(readNumber(rule?.condition, CONDITION_FIELDS.durationSeconds) || 60)
  )
  const [level, setLevel] = useState(String(rule?.level ?? ALERT_LEVELS[1]))
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (name.trim() === '') {
        setFormError('请输入规则名称')
        return
      }
      const thresholdValue = Number(threshold)
      if (!Number.isFinite(thresholdValue)) {
        setFormError('阈值要是一个数字')
        return
      }
      const durationValue = Number(duration)
      if (!Number.isInteger(durationValue) || durationValue < 0) {
        setFormError('持续时间要是 0 或更大的整数秒')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload = {
          scope,
          name: name.trim(),
          metric,
          // 条件按结构化字段组装,不接受用户手写 JSON
          condition: {
            [CONDITION_FIELDS.operator]: operator,
            [CONDITION_FIELDS.threshold]: thresholdValue,
            [CONDITION_FIELDS.durationSeconds]: durationValue,
          },
          level: Number(level),
          enabled,
        }
        if (editing) await api.admin.updateAlertRule(rule.id, payload)
        else await api.admin.createAlertRule(payload)
        toast.success(editing ? '规则已更新' : '规则已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [duration, editing, enabled, level, metric, name, onSaved, operator, rule?.id, scope, threshold]
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑告警规则' : '新建告警规则'}</ModalTitle>
          <ModalDescription>
            指标满足条件并持续指定时间后触发告警。持续时间可以过滤掉瞬时波动。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField
              label="规则名称"
              htmlFor="rule-name"
              required
              helper="告警列表里显示这个名字"
            >
              <Input
                id="rule-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>

            <FormField label="监控指标" htmlFor="rule-metric" required>
              <Select
                id="rule-metric"
                options={ALERT_METRICS.map((item) => ({
                  value: item,
                  label: alertMetricLabel(item),
                }))}
                value={metric}
                onValueChange={setMetric}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-3">
              <FormField label="比较方式" htmlFor="rule-operator" required>
                <Select
                  id="rule-operator"
                  options={ALERT_CONDITION_OPERATORS.map((item) => ({
                    value: item,
                    label: alertConditionOperatorLabel(item),
                  }))}
                  value={operator}
                  onValueChange={setOperator}
                />
              </FormField>
              <FormField label="阈值" htmlFor="rule-threshold" required>
                <Input
                  id="rule-threshold"
                  type="number"
                  step="0.1"
                  value={threshold}
                  onChange={(event) => setThreshold(event.target.value)}
                />
              </FormField>
              <FormField
                label="持续时间(秒)"
                htmlFor="rule-duration"
                required
                helper="填 0 表示立即触发"
              >
                <Input
                  id="rule-duration"
                  type="number"
                  min="0"
                  value={duration}
                  onChange={(event) => setDuration(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="告警级别" required helper="级别决定列表里的排序与提示强度">
              <SegmentedControl
                aria-label="告警级别"
                options={ALERT_LEVELS.map((item) => ({
                  value: String(item),
                  label: alertLevelLabel(item),
                }))}
                value={level}
                onValueChange={setLevel}
              />
            </FormField>

            <Checkbox
              checked={enabled}
              label="启用这条规则"
              onCheckedChange={(checked) => setEnabled(checked === true)}
            />

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存规则' : '创建规则'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** readString 从条件对象里读字符串;非字符串回空串。 */
function readString(condition: Record<string, unknown> | undefined, key: string): string {
  const value = condition?.[key]
  return typeof value === 'string' ? value : ''
}

/** readNumber 从条件对象里读数字;非数字回 0。 */
function readNumber(condition: Record<string, unknown> | undefined, key: string): number {
  const value = condition?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}
