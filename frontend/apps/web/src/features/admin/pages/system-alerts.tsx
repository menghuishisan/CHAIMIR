// 管理学校或平台告警；事件名称随已授权列表返回，规则按当前管理范围维护。
import { useState } from 'react'
import {
  BellRing,
  CircleCheck,
  CircleSlash,
  Info,
  OctagonAlert,
  Plus,
  Settings2,
  TriangleAlert,
} from 'lucide-react'
import {
  AdminScope,
  AlertLevel,
  AlertStatus,
  type AlertEvent,
  type AlertRule,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  FormField,
  Icon,
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
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  Switch,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import {
  type AsyncResourceState,
  useAsyncResource,
  usePagedResource,
  useUrlState,
} from '../../../hooks'
import { segmentCounts as segmentCountsOf } from '../../../utils/facets'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../utils/formatters'
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

/** 地址参数:tab 标签;status 事件状态分段(默认待处理),level 级别(all 为不限);页码在 page */
const ALERTS_QUERY = {
  tab: { defaultValue: 'events', allowed: ['events', 'rules'] },
  status: { defaultValue: 'pending', allowed: ['pending', 'handled', 'ignored', 'all'] },
  level: { defaultValue: 'all', allowed: ['all', ...ALERT_LEVELS.map(String)] },
} as const

const STATUS_SEGMENTS = [
  { value: 'pending', label: '待处理', status: AlertStatus.PENDING },
  { value: 'handled', label: '已处理', status: AlertStatus.HANDLED },
  { value: 'ignored', label: '已忽略', status: AlertStatus.IGNORED },
  { value: 'all', label: '全部' },
] as const

/** 级别的图形(级别同时用图形和文字表达,不只靠颜色) */
const LEVEL_ICON: Record<AlertLevel, typeof Info> = {
  [AlertLevel.NOTICE]: Info,
  [AlertLevel.WARNING]: TriangleAlert,
  [AlertLevel.SEVERE]: OctagonAlert,
  [AlertLevel.CRITICAL]: OctagonAlert,
}

/** 两种作用范围的页面文案:范围决定了事件覆盖面与规则归属,故各自说清。 */
const SCOPE_COPY: Record<
  AdminScope,
  {
    title: string
    eventScope: string
    ruleDescription: string
    ruleEmptyDescription: string
  }
> = {
  [AdminScope.GLOBAL]: {
    title: '告警中心',
    eventScope: '全平台',
    ruleDescription: '平台级规则对所有学校生效。学校自己还能加本校规则,两者互不覆盖。',
    ruleEmptyDescription: '建立平台级规则后,任一学校的指标异常都会产生告警。',
  },
  [AdminScope.TENANT]: {
    title: '学校告警',
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

/** 按管理范围展示告警事件与规则。 */
export default function SystemAlertsPage({ scope }: SystemAlertsPageProps) {
  const copy = SCOPE_COPY[scope]
  const [query, setQuery] = useUrlState(ALERTS_QUERY)
  const rules = useAsyncResource(
    () => api.admin.listAlertRules({ scope }),
    [scope],
    () => false
  )

  return (
    <PageScaffold>
      <PageHeader title={copy.title} />

      <Tabs value={query.tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="告警">
          <TabsTrigger value="events" icon={BellRing}>
            告警事件
          </TabsTrigger>
          <TabsTrigger
            value="rules"
            icon={Settings2}
            count={rules.status === 'success' ? rules.data?.length : undefined}
          >
            告警规则
          </TabsTrigger>
        </TabsList>

        <TabsContent value="events">
          <AlertEventsSection
            eventScope={copy.eventScope}
            status={query.status}
            level={query.level}
            onStatusChange={(value) => setQuery({ status: value })}
            onLevelChange={(value) => setQuery({ level: value })}
          />
        </TabsContent>
        <TabsContent value="rules">
          <AlertRulesSection
            scope={scope}
            rules={rules}
            description={copy.ruleDescription}
            emptyDescription={copy.ruleEmptyDescription}
          />
        </TabsContent>
      </Tabs>
    </PageScaffold>
  )
}

interface AlertEventsSectionProps {
  eventScope: string
  status: string
  level: string
  onStatusChange: (value: string) => void
  onLevelChange: (value: string) => void
}

/** AlertEventsSection 告警事件区:状态与级别筛选、事件表格,以及改变事件状态前的确认弹窗 */
function AlertEventsSection({
  eventScope,
  status,
  level,
  onStatusChange,
  onLevelChange,
}: AlertEventsSectionProps) {
  const [target, setTarget] = useState<{ event: AlertEvent; status: AlertStatus }>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const segment = STATUS_SEGMENTS.find((item) => item.value === status)

  const events = usePagedResource<AlertEvent>(
    (params) =>
      api.admin.listAlertEvents({
        status: segment && 'status' in segment ? segment.status : undefined,
        level: level === 'all' ? undefined : (Number(level) as AlertLevel),
        ...params,
      }),
    [level, status],
    { urlPageKey: 'page' }
  )

  const handle = async () => {
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
  }

  // 分段计数取列表接口的 facets:套用级别筛选,不套用状态筛选
  const facets =
    events.status === 'success' || events.status === 'empty' ? events.data?.facets : undefined
  const segmentCounts = segmentCountsOf(facets, 'status', STATUS_SEGMENTS, (item) =>
    'status' in item ? item.status : undefined
  )

  return (
    <>
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按处理状态筛选"
          options={STATUS_SEGMENTS.map((item) => ({
            value: item.value,
            label: item.label,
            count: segmentCounts[item.value],
          }))}
          value={status}
          onValueChange={onStatusChange}
          className="max-w-full overflow-x-auto"
        />
        <Select
          aria-label="按告警级别筛选"
          value={level}
          onValueChange={onLevelChange}
          options={[
            { value: 'all', label: '全部级别' },
            ...ALERT_LEVELS.map((item) => ({ value: String(item), label: alertLevelLabel(item) })),
          ]}
          className="w-full sm:w-36"
        />
      </div>

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      <ResourceState
        resource={events}
        emptyIcon={BellRing}
        emptyTitle={
          status === 'pending'
            ? '没有待处理的告警'
            : status !== 'all' || level !== 'all'
              ? '这个筛选下没有告警'
              : '暂无告警'
        }
        emptyDescription={
          status === 'all' && level === 'all'
            ? `${eventScope}的资源或业务指标超过规则阈值时会产生告警。`
            : '换个状态或级别看看。'
        }
        skeleton={<Skeleton variant="line" lines={6} />}
      >
        {(page) => (
          <>
            <ul
              aria-label={`告警事件,范围为${eventScope}`}
              className="divide-y divide-line rounded-lg bg-surface px-4"
            >
              {page.list.map((event) => {
                const pending = event.status === AlertStatus.PENDING
                const tone = !pending
                  ? 'text-ink-sub'
                  : event.level === AlertLevel.CRITICAL || event.level === AlertLevel.SEVERE
                    ? 'text-danger'
                    : event.level === AlertLevel.WARNING
                      ? 'text-warning'
                      : 'text-ink-sub'
                const rule = event.rule_name
                return (
                  <li key={event.id} className="flex flex-wrap items-start gap-x-4 gap-y-2 py-3">
                    <span
                      className={`flex w-16 shrink-0 items-center gap-1.5 text-sm font-medium ${tone}`}
                    >
                      <Icon icon={LEVEL_ICON[event.level]} size="sm" />
                      {alertLevelLabel(event.level)}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span
                        className={
                          pending ? 'block text-sm font-medium text-ink' : 'block text-sm text-ink'
                        }
                      >
                        {event.message}
                      </span>
                      <span className="block text-xs text-ink-sub">
                        触发规则:{rule ?? '…'} ·{' '}
                        <span title={formatDateTime(event.triggered_at)}>
                          {formatRelativeOccurrence(event.triggered_at)}
                        </span>
                        {pending
                          ? ''
                          : ` · ${alertStatusLabel(event.status)}${event.handled_at ? `(${formatRelativeOccurrence(event.handled_at)})` : ''}`}
                      </span>
                    </span>
                    {pending ? (
                      <span className="flex shrink-0 items-center gap-1">
                        <Button
                          variant="secondary"
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
                      </span>
                    ) : null}
                  </li>
                )
              })}
            </ul>
            <div className="mt-4">
              <Pagination
                page={events.page}
                pageSize={events.pageSize}
                total={events.total}
                onPageChange={events.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>

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
                    ? '确认这个问题已经处理完毕。指标恢复正常后再次异常，才会产生新的告警。'
                    : '忽略表示这次不需要跟进。规则继续生效；指标恢复后再次异常仍会告警。'}
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
  /** 当前管理范围的规则列表 */
  rules: AsyncResourceState<AlertRule[]>
  description: string
  emptyDescription: string
}

/** 维护当前管理范围的规则，权限由服务端检查。 */
function AlertRulesSection({
  scope,
  rules,
  description,
  emptyDescription,
}: AlertRulesSectionProps) {
  const [formTarget, setFormTarget] = useState<{ rule?: AlertRule } | undefined>()
  const [togglingId, setTogglingId] = useState<string>()
  const [toggleError, setToggleError] = useState<string>()

  /** toggle 行内启用开关:按规则现有字段整条保存,只改 enabled */
  const toggle = async (rule: AlertRule, enabled: boolean) => {
    setTogglingId(rule.id)
    setToggleError(undefined)
    try {
      await api.admin.updateAlertRule(rule.id, {
        scope,
        name: rule.name,
        metric: rule.metric,
        condition: rule.condition,
        level: rule.level,
        enabled,
      })
      toast.success(enabled ? `已启用「${rule.name}」` : `已停用「${rule.name}」`)
      rules.reload()
    } catch (error) {
      setToggleError(userFacingErrorMessage(error, '切换没有成功,请稍后重试。'))
    } finally {
      setTogglingId(undefined)
    }
  }

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
          resource={{ ...rules, status: rules.status === 'empty' ? 'success' : rules.status }}
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
          {(list) =>
            list.length === 0 ? (
              <p className="rounded-lg bg-surface px-4 py-8 text-center text-sm text-ink-sub">
                {emptyDescription}
              </p>
            ) : (
              <ul className="divide-y divide-line rounded-lg bg-surface px-4 ">
                {list.map((rule) => (
                  <AlertRuleRow
                    key={rule.id}
                    rule={rule}
                    toggling={togglingId === rule.id}
                    onToggle={(enabled) => void toggle(rule, enabled)}
                    onEdit={() => setFormTarget({ rule })}
                  />
                ))}
              </ul>
            )
          }
        </ResourceState>

        {toggleError ? <Callout tone="danger">{toggleError}</Callout> : null}
        <p className="text-sm text-ink-sub">
          规则改动对下一次评估生效,已产生的告警不受影响;停用规则不会删除历史告警。
        </p>
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

interface AlertRuleRowProps {
  rule: AlertRule
  toggling: boolean
  onToggle: (enabled: boolean) => void
  onEdit: () => void
}

/** AlertRuleRow 一条规则:名称、可读的触发条件与级别,行内启用开关和编辑 */
function AlertRuleRow({ rule, toggling, onToggle, onEdit }: AlertRuleRowProps) {
  const operator = readString(rule.condition, CONDITION_FIELDS.operator)
  const threshold = readNumber(rule.condition, CONDITION_FIELDS.threshold)
  const duration = readNumber(rule.condition, CONDITION_FIELDS.durationSeconds)
  const condition = operator
    ? `${alertMetricLabel(rule.metric)} ${alertConditionOperatorLabel(operator)} ${threshold}${duration > 0 ? `,持续 ${duration} 秒` : ',立即触发'}`
    : '条件未完整配置'
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium text-ink">{rule.name}</span>
        <span className="block text-xs text-ink-sub">
          {condition} · {alertLevelLabel(rule.level)}
        </span>
      </span>
      <Switch
        checked={rule.enabled}
        disabled={toggling}
        aria-label={`${rule.enabled ? '停用' : '启用'}规则 ${rule.name}`}
        onCheckedChange={onToggle}
      />
      <Button variant="ghost" size="sm" onClick={onEdit}>
        编辑
      </Button>
    </li>
  )
}

interface AlertRuleFormModalProps {
  rule?: AlertRule
  /** 规则作用范围:与所在页面一致,提交时原样带上(后端按身份最终裁定) */
  scope: AdminScope
  onClose: () => void
  onSaved: () => void
}

/** 编辑结构化告警条件，保留校验与保存错误。 */
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
    String(rule ? readNumber(rule.condition, CONDITION_FIELDS.durationSeconds) : 60)
  )
  const [level, setLevel] = useState(String(rule?.level ?? ALERT_LEVELS[1]))
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(() => ({
    name: name.trim() === '' ? '请输入规则名称' : null,
    // 先检查原文,显式0合法,空白不得转换为0。
    threshold:
      threshold.trim() === ''
        ? '请输入阈值'
        : !Number.isFinite(Number(threshold))
          ? '阈值要是一个数字'
          : null,
    duration:
      duration.trim() === ''
        ? '请输入持续时间'
        : !Number.isInteger(Number(duration)) || Number(duration) < 0
          ? '持续时间要是 0 或更大的整数秒'
          : null,
  }))

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (working || !validate()) return
    const thresholdValue = Number(threshold)
    const durationValue = Number(duration)
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
  }

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
              error={errors.name}
              helper="告警列表里显示这个名字"
            >
              <Input
                id="rule-name"
                value={name}
                invalid={Boolean(errors.name)}
                onBlur={() => validate(['name'])}
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
              <FormField
                label="阈值"
                htmlFor="rule-threshold"
                required
                error={errors.threshold}
                helper={
                  metric.endsWith('.failed_rate')
                    ? '填 0 到 1 的比例，例如 0.2 表示 20%'
                    : undefined
                }
              >
                <Input
                  id="rule-threshold"
                  required
                  type="number"
                  step="0.1"
                  value={threshold}
                  invalid={Boolean(errors.threshold)}
                  onBlur={() => validate(['threshold'])}
                  onChange={(event) => setThreshold(event.target.value)}
                />
              </FormField>
              <FormField
                label="持续时间(秒)"
                htmlFor="rule-duration"
                required
                error={errors.duration}
                helper="填 0 表示立即触发"
              >
                <Input
                  id="rule-duration"
                  required
                  type="number"
                  min="0"
                  value={duration}
                  invalid={Boolean(errors.duration)}
                  onBlur={() => validate(['duration'])}
                  onChange={(event) => setDuration(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="告警级别" required helper="级别表示需要关注的程度">
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
