// 租户配置页(校管侧栏,/school-admin/settings)。
//
// 学校的展示信息与开通策略:显示名、校徽、登录方式、是否用激活码开通账号。
// 认证方式选了 CAS/LDAP 后还要在认证配置页填服务器参数 —— 两页分工明确:
// 这里选「用哪种方式」,那里填「这种方式连哪台服务器」。
//
// feature_flags 是 JSONB 开放对象,承载启用哪些业务模块。按已登记的模块键渲染开关,
// 不给裸 JSON 文本域;未登记的键原样保留不丢弃(平台侧可能写入其他开关)。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { CircleCheck, KeyRound, Settings, Shield } from 'lucide-react'
import { AuthMode, type Tenant } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  DescriptionList,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  Switch,
  TenantCrest,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ImageUploadField } from '../../../../components/ImageUploadField'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDate } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { authModeLabel, deployModeLabel, TENANT_MODULE_OPTIONS } from '../../../../utils/labels/identity'
import { readTenantModules, TENANT_MODULES_KEY } from '../../tenantModules'

/**
 * SchoolAdminSettingsPage 维护学校展示信息与开通策略。
 */
export default function SchoolAdminSettingsPage() {
  const tenant = useAsyncResource(
    () => api.identity.getTenantConfig(),
    [],
    () => false
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '系统配置' }, { label: '学校配置' }]} />}
        title="学校配置"
        description="学校的展示信息、启用的业务模块、登录方式与账号开通策略。"
        icon={Settings}
      />

      <ResourceState
        resource={tenant}
        emptyIcon={Settings}
        emptyTitle="暂无学校配置"
        emptyDescription="学校配置由平台开通时创建。如果这里是空的,请联系平台管理员。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => <SettingsContent tenant={data} onSaved={tenant.reload} />}
      </ResourceState>
    </PageScaffold>
  )
}

interface SettingsContentProps {
  tenant: Tenant
  onSaved: () => void
}

/**
 * SettingsContent 渲染只读档案与可编辑配置表单。
 */
function SettingsContent({ tenant, onSaved }: SettingsContentProps) {
  const navigate = useNavigate()

  const [displayName, setDisplayName] = useState(tenant.display_name ?? tenant.name)
  // 校徽是即时生效的:上传或移除当场落库,响应就是新的配置视图,故这里只存可显示的图。
  // 它不参与下面这个表单的提交 —— 表单负责显示名、模块开关、登录方式与激活码策略。
  const [logoImage, setLogoImage] = useState(tenant.logo_image ?? '')
  const [authMode, setAuthMode] = useState(String(tenant.auth_mode))
  const [enableActivation, setEnableActivation] = useState(tenant.enable_activation_code)
  const [modules, setModules] = useState(() => readTenantModules(tenant.feature_flags))
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const authModeValue = Number(authMode) as AuthMode
  const needsSsoConfig = authModeValue !== AuthMode.LOCAL

  const profileItems = useMemo(
    () => [
      { term: '学校名称', description: tenant.name },
      { term: '学校短码', description: tenant.code, mono: true },
      { term: '部署形态', description: deployModeLabel(tenant.deploy_mode) },
      {
        term: '服务到期',
        description: tenant.expire_at ? formatDate(tenant.expire_at) : '长期有效',
        mono: true,
      },
    ],
    [tenant]
  )

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (displayName.trim() === '') {
        setFormError('请输入学校显示名')
        return
      }
      if (modules.length === 0) {
        setFormError('至少启用一个业务模块,否则师生进入后没有可用功能')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        await api.identity.updateTenantConfig({
          display_name: displayName.trim(),
          // 未登记的开关原样保留:平台侧可能写入其他键,前端不该丢弃
          feature_flags: { ...tenant.feature_flags, [TENANT_MODULES_KEY]: modules },
          auth_mode: authModeValue,
          enable_activation_code: enableActivation,
        })
        toast.success('学校配置已保存')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [authModeValue, displayName, enableActivation, modules, onSaved, tenant.feature_flags]
  )

  return (
    <>
      <PageSection title="学校档案" description="这些信息由平台开通时设定,校内不可修改。">
        <DescriptionList dense columns={2} items={profileItems} />
      </PageSection>

      <PageSection title="可修改的配置">
        <Card>
          <CardHeader
            title="展示与策略"
            description="显示名与校徽出现在登录页与顶栏;开通策略影响新账号怎么首次登录。"
          />
          <CardBody>
            <form onSubmit={submit} noValidate className="flex flex-col gap-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="学校显示名"
                  htmlFor="tenant-display-name"
                  required
                  helper="登录页与顶栏显示这个名字"
                >
                  <Input
                    id="tenant-display-name"
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                  />
                </FormField>
                <FormField
                  label="校徽"
                  htmlFor="tenant-logo"
                  helper="选好即刻生效,不用再点保存;留空时显示学校名称的第一个字。"
                >
                  <ImageUploadField
                    inputId="tenant-logo"
                    hasImage={logoImage !== ''}
                    failureMessage="校徽这次没有更新成功,请稍后重试。"
                    preview={
                      <TenantCrest
                        name={displayName || tenant.name}
                        logoSrc={logoImage}
                        size="lg"
                      />
                    }
                    onUpload={async (file, onProgress) => {
                      // 上传即生效,响应就是新的配置视图,预览直接取服务端给的校徽
                      const saved = await api.identity.uploadTenantLogo(file, onProgress)
                      setLogoImage(saved.logo_image ?? '')
                      toast.success('校徽已更新')
                    }}
                    onCleared={async () => {
                      await api.identity.clearTenantLogo()
                      setLogoImage('')
                      toast.success('校徽已移除')
                    }}
                  />
                </FormField>
              </div>

              <FormField
                label="启用的业务模块"
                required
                helper="关掉的模块对师生不可见。已有数据不会删除,重新开启即恢复"
              >
                <div className="flex flex-col gap-3">
                  {TENANT_MODULE_OPTIONS.map((option) => (
                    <div key={option.value} className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="text-base text-ink">{option.label}</div>
                        <p className="text-xs text-ink-sub">{option.description}</p>
                      </div>
                      <Switch
                        checked={modules.includes(option.value)}
                        aria-label={`启用${option.label}模块`}
                        onCheckedChange={(checked) =>
                          setModules((current) =>
                            checked
                              ? [...current, option.value]
                              : current.filter((item) => item !== option.value)
                          )
                        }
                      />
                    </div>
                  ))}
                </div>
              </FormField>

              <FormField
                label="登录方式"
                required
                helper="选了统一认证或目录服务后,还要在认证配置页填服务器参数"
              >
                <SegmentedControl
                  aria-label="登录方式"
                  options={[
                    { value: String(AuthMode.LOCAL), label: authModeLabel(AuthMode.LOCAL) },
                    { value: String(AuthMode.CAS), label: authModeLabel(AuthMode.CAS) },
                    { value: String(AuthMode.LDAP), label: authModeLabel(AuthMode.LDAP) },
                  ]}
                  value={authMode}
                  onValueChange={setAuthMode}
                />
              </FormField>

              {needsSsoConfig ? (
                <Callout tone="warning" title="别忘了填服务器参数">
                  <div className="flex flex-wrap items-center gap-2">
                    <span>
                      {authModeLabel(authModeValue)}
                      需要在认证配置页填服务器地址等参数,否则师生无法登录。
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      leftIcon={Shield}
                      onClick={() => navigate('/school-admin/auth-config')}
                    >
                      去认证配置
                    </Button>
                  </div>
                </Callout>
              ) : null}

              <div className="flex flex-col gap-3 well p-4">
                <Checkbox
                  checked={enableActivation}
                  label="允许用激活码开通账号"
                  onCheckedChange={(checked) => setEnableActivation(checked === true)}
                />
                <p className="text-sm text-ink-sub">
                  开启后新增账号可以生成一次性激活码,由本人自行设置密码。关闭则必须由管理员设初始密码。
                </p>
                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge tone={enableActivation ? 'success' : 'neutral'}>
                    {enableActivation ? '激活码可用' : '激活码已关闭'}
                  </Badge>
                  <Badge tone="neutral">
                    <span className="flex items-center gap-1">
                      <KeyRound aria-hidden className="size-3" />
                      当前 {authModeLabel(tenant.auth_mode)}
                    </span>
                  </Badge>
                </div>
              </div>

              {formError ? <Callout tone="danger">{formError}</Callout> : null}

              <div className="flex items-center gap-2">
                <Button type="submit" variant="primary" leftIcon={CircleCheck} loading={working}>
                  保存配置
                </Button>
                <span className="text-sm text-ink-sub">修改会记入审计日志。</span>
              </div>
            </form>
          </CardBody>
        </Card>
      </PageSection>
    </>
  )
}
