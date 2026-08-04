// 系统配置页(平台侧栏,/platform-admin/settings)。
//
// 平台级配置的查看、修改、历史与回滚。三件事要一起说清:
//   1. 修改用乐观锁:提交时带上读到的版本号,别人在这期间改过就会冲突并要求重新读取 ——
//      两个管理员同时改同一项时不会有人的改动被静默吞掉。
//   2. 凭据类字段(含 password / secret / token / key 等词的键)后端一律脱敏返回「已配置」。
//      保存时整个配置值会被替换,所以把「已配置」原样提交会把真实密钥覆盖成这四个字。
//      故本页在编辑时把凭据字段从文档里摘出来,单独要求重新填写,绝不让掩码值进入提交体。
//   3. 回滚是把某次变更的「改动前值」写回去,同样要带当前版本号。
//
// 配置值是开放 JSONB(每个配置项的键由使用方定义,不可枚举),故非凭据部分用文档编辑器
// 并在本地做合法性校验;凭据部分是可枚举的键,做成显式密码输入。

import { useCallback, useMemo, useState } from 'react'
import {
  History,
  KeyRound,
  RotateCcw,
  Save,
  Settings,
  TriangleAlert,
} from 'lucide-react'
import {
  AdminScope,
  type ConfigChangeLog,
  type SystemConfig,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FormField,
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
  Skeleton,
  Stat,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { configKeyDescription, configKeyLabel } from '../../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 后端脱敏后填回的占位文案(secretmap.MaskedValue),识别它是为了绝不把它提交回去。 */
const MASKED_VALUE = '已配置'

/**
 * 凭据类字段的词根,与后端 privacy.credentialKeyMarkers 一致。
 * 键名里包含任一词根即被后端加密保存并脱敏返回,故前端按同一口径识别 ——
 * 判据不一致会导致某个字段的掩码值被当成普通值提交,把真实密钥覆盖掉。
 */
const CREDENTIAL_KEY_MARKERS = [
  'password',
  'passwd',
  'private_key',
  'privatekey',
  'access_key',
  'accesskey',
  'signing_key',
  'signingkey',
  'session_secret',
  'sessionsecret',
  'secret',
  'token',
  'credential',
  'authorization',
  'api_key',
  'apikey',
] as const

/**
 * PlatformSettingsPage 承载平台级系统配置的查看与维护。
 */
export default function PlatformSettingsPage() {
  const [editTarget, setEditTarget] = useState<SystemConfig>()
  const [historyTarget, setHistoryTarget] = useState<SystemConfig>()

  const configs = useAsyncResource(
    () => api.admin.listConfigs({ scope: AdminScope.GLOBAL }),
    [],
    (value) => value.length === 0,
  )

  const list = useMemo(() => configs.data ?? [], [configs.data])

  const stats = useMemo(
    () => ({
      registered: list.filter((item) => configKeyDescription(item.key) !== undefined).length,
      withCredentials: list.filter((item) => credentialKeysOf(item.value).length > 0).length,
    }),
    [list],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '系统配置' }]} />}
        title="系统配置"
        description="平台级运行参数。改动立即对全平台生效,每次改动都会留下可回滚的记录。"
        icon={Settings}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="配置项" value={list.length} icon={Settings} />
          <Stat
            label="已登记说明"
            value={stats.registered}
            icon={Settings}
            hint="其余是使用方自定义的键"
          />
          <Stat
            label="含凭据字段"
            value={stats.withCredentials}
            icon={KeyRound}
            hint="保存时需要重新填写"
          />
        </div>
      </PageSection>

      <PageSection
        title="配置项"
        description="学校自己的配置由该校管理员维护,这里只有平台级配置。"
      >
        <div className="flex flex-col gap-4">
          <ResourceState
            resource={configs}
            emptyIcon={Settings}
            emptyDescription="平台首次写入配置后会出现在这里。"
            emptyTitle="还没有平台级配置"
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {(items) => (
              <div className="grid gap-4 lg:grid-cols-2">
                {items.map((config) => (
                  <ConfigCard
                    key={config.id}
                    config={config}
                    onEdit={() => setEditTarget(config)}
                    onHistory={() => setHistoryTarget(config)}
                  />
                ))}
              </div>
            )}
          </ResourceState>

          <Callout tone="info">
            两个人同时改同一项时,后提交的一方会看到冲突提示并需要重新读取 —— 不会有人的改动被悄悄覆盖。
          </Callout>
        </div>
      </PageSection>

      {editTarget ? (
        <ConfigEditModal
          config={editTarget}
          onClose={() => setEditTarget(undefined)}
          onSaved={() => {
            setEditTarget(undefined)
            configs.reload()
          }}
        />
      ) : null}

      {historyTarget ? (
        <ConfigHistoryModal
          config={historyTarget}
          onClose={() => setHistoryTarget(undefined)}
          onRolledBack={() => {
            setHistoryTarget(undefined)
            configs.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface ConfigCardProps {
  config: SystemConfig
  onEdit: () => void
  onHistory: () => void
}

/**
 * ConfigCard 展示单个配置项的当前值概要。
 * 凭据字段只显示「已配置」,不显示值也不显示密文。
 */
function ConfigCard({ config, onEdit, onHistory }: ConfigCardProps) {
  const description = configKeyDescription(config.key)
  const entries = useMemo(() => summarizeValue(config.value), [config.value])

  return (
    <Card>
      <CardHeader
        title={configKeyLabel(config.key)}
        description={description ?? '使用方自定义的配置项'}
        actions={<Badge tone="neutral">第 {config.version} 版</Badge>}
      />
      <CardBody className="flex flex-col gap-3">
        <DescriptionList
          dense
          items={[
            { term: '配置键', description: config.key, mono: true },
            { term: '最近更新', description: formatDateTime(config.updated_at), mono: true },
          ]}
        />

        {entries.length > 0 ? (
          <DescriptionList dense columns={2} items={entries} />
        ) : (
          <p className="text-sm text-ink-sub">这一项当前是空配置。</p>
        )}

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" leftIcon={Save} onClick={onEdit}>
            修改
          </Button>
          <Button variant="ghost" size="sm" leftIcon={History} onClick={onHistory}>
            变更历史
          </Button>
        </div>
      </CardBody>
    </Card>
  )
}

interface ConfigEditModalProps {
  config: SystemConfig
  onClose: () => void
  onSaved: () => void
}

/**
 * ConfigEditModal 修改一个配置项。
 * 凭据字段从文档里摘出来单独填:掩码值原样提交会把真实密钥覆盖成「已配置」四个字。
 */
function ConfigEditModal({ config, onClose, onSaved }: ConfigEditModalProps) {
  const credentialKeys = useMemo(() => credentialKeysOf(config.value), [config.value])

  const [valueText, setValueText] = useState(() =>
    JSON.stringify(withoutKeys(config.value, credentialKeys), null, 2),
  )
  const [credentials, setCredentials] = useState<Record<string, string>>(() =>
    Object.fromEntries(credentialKeys.map((key) => [key, ''])),
  )
  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const parsed = useMemo(() => parseValue(valueText), [valueText])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const next: Record<string, string | null> = { value: parsed.error ?? null }
      for (const key of credentialKeys) {
        const filled = credentials[key]?.trim() ?? ''
        next[`credential:${key}`] =
          filled === ''
            ? '现有值不会显示,保存时必须重新填入完整的值'
            : filled === MASKED_VALUE
              ? '这是占位文案,不是真实值,请填入真实内容'
              : null
      }
      setErrors(next)
      if (Object.values(next).some((item) => item !== null)) {
        setFormError('有几项还不能提交,按提示改一下。')
        return
      }

      setFormError(undefined)
      setWorking(true)
      try {
        await api.admin.updateConfig(config.key, {
          scope: AdminScope.GLOBAL,
          value: {
            ...(parsed.value ?? {}),
            ...Object.fromEntries(credentialKeys.map((key) => [key, credentials[key].trim()])),
          },
          version: config.version,
        })
        toast.success('配置已保存')
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(
            error,
            '保存没有成功。如果这一项刚被别人改过,请关闭后重新打开再改一次。',
          ),
        )
      } finally {
        setWorking(false)
      }
    },
    [config.key, config.version, credentialKeys, credentials, onSaved, parsed],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>修改{configKeyLabel(config.key)}</ModalTitle>
          <ModalDescription>
            改动立即对全平台生效,并留下一条可回滚的记录。保存基于第 {config.version} 版,
            如果这期间别人改过,保存会被拒绝。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <DescriptionList
              dense
              items={[
                { term: '配置键', description: config.key, mono: true },
                { term: '当前版本', description: `第 ${config.version} 版` },
                { term: '最近更新', description: formatDateTime(config.updated_at), mono: true },
              ]}
            />

            <FormField
              label="配置内容"
              htmlFor="config-value"
              required
              error={errors.value}
              helper="不含凭据字段。保存时会用这里的内容整体替换旧值"
            >
              <Textarea
                id="config-value"
                className="font-mono text-sm"
                value={valueText}
                rows={14}
                spellCheck={false}
                invalid={Boolean(errors.value)}
                onChange={(event) => setValueText(event.target.value)}
              />
            </FormField>

            {credentialKeys.length > 0 ? (
              <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
                <div>
                  <p className="text-base text-ink">凭据字段</p>
                  <p className="text-sm text-ink-sub">
                    这些字段加密保存、不会回显。保存会整体替换配置值,所以每次都要重新填入完整的值。
                  </p>
                </div>
                {credentialKeys.map((key) => (
                  <FormField
                    key={key}
                    label={key}
                    htmlFor={`credential-${key}`}
                    required
                    error={errors[`credential:${key}`]}
                  >
                    <Input
                      id={`credential-${key}`}
                      type="password"
                      autoComplete="off"
                      value={credentials[key] ?? ''}
                      invalid={Boolean(errors[`credential:${key}`])}
                      onChange={(event) =>
                        setCredentials((current) => ({ ...current, [key]: event.target.value }))
                      }
                    />
                  </FormField>
                ))}
                <Callout tone="warning" title="不要填「已配置」">
                  那是脱敏占位文案,不是真实值。填它会把真实密钥覆盖掉。
                </Callout>
              </div>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              保存配置
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface ConfigHistoryModalProps {
  config: SystemConfig
  onClose: () => void
  onRolledBack: () => void
}

/**
 * ConfigHistoryModal 列出变更历史并承载回滚。
 * 回滚写回的是那条记录的「改动前值」,同样受乐观锁保护。
 */
function ConfigHistoryModal({ config, onClose, onRolledBack }: ConfigHistoryModalProps) {
  const [target, setTarget] = useState<ConfigChangeLog>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const history = usePagedResource<ConfigChangeLog>(
    (params) => api.admin.listConfigHistory(config.key, { scope: AdminScope.GLOBAL, ...params }),
    [config.key],
  )

  const rollback = useCallback(async () => {
    if (!target) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.admin.rollbackConfig(config.key, {
        scope: AdminScope.GLOBAL,
        version: config.version,
        change_log_id: target.id,
      })
      toast.success('已回滚到这次改动前的内容')
      onRolledBack()
    } catch (error) {
      setActionError(
        userFacingErrorMessage(
          error,
          '回滚没有成功。如果这一项刚被别人改过,请关闭后重新打开再试。',
        ),
      )
    } finally {
      setWorking(false)
    }
  }, [config.key, config.version, onRolledBack, target])

  const columns: TableColumn<ConfigChangeLog>[] = [
    {
      key: 'created_at',
      header: '改动时间',
      render: (log) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(log.created_at)}
        </span>
      ),
    },
    {
      key: 'changed_keys',
      header: '改了哪些字段',
      render: (log) => {
        const keys = changedKeys(log.old_value, log.new_value)
        return keys.length > 0 ? (
          <span className="flex flex-wrap gap-1">
            {keys.map((key) => (
              <Badge key={key} tone="neutral">
                {key}
              </Badge>
            ))}
          </span>
        ) : (
          <span className="text-sm text-ink-sub">内容未变</span>
        )
      },
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (log) => (
        <Button variant="ghost" size="sm" leftIcon={RotateCcw} onClick={() => setTarget(log)}>
          回滚到改动前
        </Button>
      ),
    },
  ]

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{configKeyLabel(config.key)}的变更历史</ModalTitle>
          <ModalDescription>
            每次改动都会留一条记录。回滚会把内容写回到所选记录改动之前的样子。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={history}
            emptyIcon={History}
            emptyTitle="还没有变更记录"
            emptyDescription="这一项自创建以来没有改动过。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={history.page}
                  pageSize={history.pageSize}
                  total={history.total}
                  onPageChange={history.setPage}
                />
              </>
            )}
          </ResourceState>

          {target ? (
            <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
              <div className="flex flex-wrap items-center gap-2">
                <TriangleAlert aria-hidden="true" className="size-4 text-warning" />
                <span className="text-base text-ink">
                  确认回滚到 {formatDateTime(target.created_at)} 这次改动之前?
                </span>
              </div>
              <p className="text-sm text-ink-sub">
                回滚本身也会记一条新的变更记录,所以随时可以再回滚回来。凭据字段按当时的加密值写回。
              </p>
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="danger" loading={working} onClick={() => void rollback()}>
                  确认回滚
                </Button>
                <Button variant="ghost" onClick={() => setTarget(undefined)}>
                  先不回滚
                </Button>
              </div>
            </div>
          ) : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** credentialKeysOf 列出配置值顶层的凭据字段键。 */
function credentialKeysOf(value: Record<string, unknown>): string[] {
  return Object.keys(value).filter(isCredentialKey).sort()
}

/** isCredentialKey 判断键名是否带凭据语义,口径与后端一致。 */
function isCredentialKey(key: string): boolean {
  const normalized = key.trim().toLowerCase()
  return CREDENTIAL_KEY_MARKERS.some((marker) => normalized.includes(marker))
}

/** withoutKeys 去掉指定键,得到可以安全放进编辑器的部分。 */
function withoutKeys(
  value: Record<string, unknown>,
  keys: string[],
): Record<string, unknown> {
  const drop = new Set(keys)
  return Object.fromEntries(Object.entries(value).filter(([key]) => !drop.has(key)))
}

/** summarizeValue 把配置值摊成可读条目;凭据字段只说「已配置」。 */
function summarizeValue(value: Record<string, unknown>): Array<{
  term: string
  description: string
  mono?: boolean
}> {
  return Object.entries(value).map(([key, raw]) => {
    if (isCredentialKey(key)) return { term: key, description: '已配置(不显示)' }
    return { term: key, description: describeValue(raw), mono: true }
  })
}

/** describeValue 给任意配置值一个短的可读表达,不打印整段结构。 */
function describeValue(raw: unknown): string {
  if (typeof raw === 'string') return raw === '' ? '(空)' : raw
  if (typeof raw === 'number' || typeof raw === 'boolean') return String(raw)
  if (Array.isArray(raw)) return `${raw.length} 项`
  if (raw === null || raw === undefined) return '未设置'
  return `${Object.keys(raw as Record<string, unknown>).length} 个子项`
}

/** changedKeys 比较改动前后的键值,列出真正变化的键。 */
function changedKeys(
  oldValue: Record<string, unknown>,
  newValue: Record<string, unknown>,
): string[] {
  const keys = new Set([...Object.keys(oldValue), ...Object.keys(newValue)])
  return [...keys]
    .filter((key) => JSON.stringify(oldValue[key]) !== JSON.stringify(newValue[key]))
    .sort()
}

/** ParsedValue 是配置内容文本的解析结果。 */
interface ParsedValue {
  value?: Record<string, unknown>
  error?: string
}

/**
 * parseValue 解析配置内容。
 * 后端要求 value 是一个对象且非空(nil 会被拒),故这两条在本地先判定。
 */
function parseValue(text: string): ParsedValue {
  const trimmed = text.trim()
  if (trimmed === '') return { error: '配置内容不能为空。' }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch {
    return { error: '内容不是合法的配置格式,检查是否漏了逗号或引号。' }
  }
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return { error: '配置内容最外层要是一个对象。' }
  }
  return { value: raw as Record<string, unknown> }
}
