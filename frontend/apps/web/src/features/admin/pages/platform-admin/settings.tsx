// 系统配置页(平台侧栏,/platform-admin/settings)。
//
// 平台级配置的查看与修改。两件事要一起说清:
//   1. 修改用乐观锁:提交时带上读到的版本号,别人在这期间改过就会冲突并要求重新读取 ——
//      两个管理员同时改同一项时不会有人的改动被静默吞掉。
//   2. 凭据类字段(含 password / secret / token / key 等词的键)后端一律脱敏返回「已配置」。
//      保存时整个配置值会被替换,所以把「已配置」原样提交会把真实密钥覆盖成这四个字。
//      故本页在编辑时把凭据字段从文档里摘出来,单独要求重新填写,绝不让掩码值进入提交体。
//
// 变更历史与回滚在深页 /platform-admin/settings/:configKey/history:
// 历史是分页数据,而浮层里不出现分页(规范 §6.5.5 A)。
//
// 配置值是开放 JSONB(每个配置项的键由使用方定义,不可枚举),故非凭据部分用文档编辑器
// 并在本地做合法性校验;凭据部分是可枚举的键,做成显式密码输入。
// 值的读取、脱敏与比较口径收敛在 ../../configValue,与变更历史页共用一套判定。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import {
  History,
  Save,
  Settings,
} from 'lucide-react'
import {
  AdminScope,
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
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { configKeyDescription, configKeyLabel } from '../../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  credentialKeysOf,
  MASKED_VALUE,
  parseValue,
  summarizeValue,
  withoutKeys,
} from '../../configValue'

/**
 * PlatformSettingsPage 承载平台级系统配置的查看与维护。
 */
export default function PlatformSettingsPage() {
  const navigate = useNavigate()
  const [editTarget, setEditTarget] = useState<SystemConfig>()

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
        kicker={<Breadcrumb items={[{ label: '底层资源' }]} />}
        title="系统配置"
        description="平台级运行参数。改动立即对全平台生效,每次改动都会留下可回滚的记录。"
        icon={Settings}
      />

      {/*
        归族:资源列表族的卡片网格形态(§6.5.3 第 ① 族 + §6.5.2 第二条出路)。
        指标降为内联摘要;三项由一次取齐的全量配置算出(接口不分页,故是全量口径,§6.5.4)。
        变更历史是深页而不是弹窗:它是分页数据,而浮层里不出现分页(§6.5.5 A)。
      */}
      <MetricStrip
        label="平台配置摘要"
        className="mb-5"
        items={[
          { label: '配置项', value: list.length, hint: '平台级运行参数' },
          {
            label: '已登记说明',
            value: stats.registered,
            hint: '其余是使用方自定义的键',
          },
          {
            label: '含凭据字段',
            value: stats.withCredentials,
            hint: '保存时需要重新填写',
          },
        ]}
      />

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
                    onHistory={() => navigate(`/platform-admin/settings/${encodeURIComponent(config.key)}/history`)}
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
              <div className="flex flex-col gap-4 well p-4">
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
            <Button type="submit" variant="primary" loading={working}>
              保存配置
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
