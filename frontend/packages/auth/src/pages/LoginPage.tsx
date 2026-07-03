// LoginPage：学校用户统一登录页，支持手机号、学工号和短信验证码。

import React, { useState } from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { Building2, LockKeyhole, Phone, UserRound } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { readFrontendConfig } from '@chaimir/shared'
import type { LoginMode } from '../types'
import { runSubmit, useFormState } from '../form-state'
import { loginByMode, handleLoginResponse } from '../session-routing'
import { AuthBlock } from '../components/AuthBlock'
import { SmsField } from '../components/SmsField'
import { TenantPicker } from '../components/TenantPicker'
import { TextField } from '../components/TextField'

const loginModes: Array<{ key: LoginMode; label: string }> = [
  { key: 'phone', label: '手机号密码' },
  { key: 'no', label: '学工号密码' },
  { key: 'sms', label: '短信验证码' },
]

/**
 * LoginPage 提供手机号密码、学工号密码和短信验证码三种学校用户登录方式。
 */
export function LoginPage({ api, config }: { api: ChaimirApi; config: ReturnType<typeof readFrontendConfig> }): React.ReactElement {
  const [mode, setMode] = useState<LoginMode>('phone')
  const [state, setState] = useFormState()

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      const response = await loginByMode(api, mode, values)
      if (response.need_select_tenant) {
        setState((current) => ({
          ...current,
          values: {
            ...current.values,
            need_select_tenant: '1',
            tenants: JSON.stringify(response.tenants ?? []),
          },
        }))
        return '请选择学校后继续登录'
      }
      handleLoginResponse(response, config)
      return '登录成功，正在进入平台'
    })
  }

  return (
    <AuthBlock
      title="登录 Chaimir"
      description="使用学校开通的账号进入对应角色功能页。"
      state={state}
    >
      <div className="public-tabs" role="tablist" aria-label="登录方式">
        {loginModes.map((item) => (
          <Button
            key={item.key}
            type="button"
            variant={mode === item.key ? 'primary' : 'ghost'}
            size="sm"
            onClick={() => setMode(item.key)}
          >
            {item.label}
          </Button>
        ))}
      </div>
      {state.values.need_select_tenant === '1' && <TenantPicker tenants={state.values.tenants} onSelect={(tenantId) => setState((current) => ({ ...current, values: { ...current.values, tenant_id: tenantId, need_select_tenant: '' } }))} />}
      <form className="public-form" onSubmit={submit}>
        {mode === 'phone' && <TextField icon={<Phone size={17} />} name="phone" label="手机号" value={state.values.phone} onChange={setState} autoComplete="tel" required />}
        {mode === 'phone' && <TextField icon={<Building2 size={17} />} name="tenant_id" label="学校编号" value={state.values.tenant_id} onChange={setState} required />}
        {mode === 'no' && <TextField icon={<Building2 size={17} />} name="tenant_code" label="学校短码" value={state.values.tenant_code} onChange={setState} required />}
        {mode === 'no' && <TextField icon={<UserRound size={17} />} name="no" label="学号或工号" value={state.values.no} onChange={setState} required />}
        {mode === 'sms' && <TextField icon={<Phone size={17} />} name="phone" label="手机号" value={state.values.phone} onChange={setState} autoComplete="tel" required />}
        {mode === 'sms' && <TextField icon={<Building2 size={17} />} name="tenant_id" label="学校编号" value={state.values.tenant_id} onChange={setState} required />}
        {mode === 'sms' && <SmsField api={api} state={state} setState={setState} scene={1} />}
        {mode !== 'sms' && <TextField icon={<LockKeyhole size={17} />} name="password" label="密码" type="password" value={state.values.password} onChange={setState} autoComplete="current-password" required />}
        <Button type="submit" size="lg" block loading={state.loading}>登录</Button>
      </form>
      <div className="public-links">
        <a href="#forgot">忘记密码</a>
        <a href="#activate">激活账号</a>
        <a href="#sso">统一认证</a>
        <a href="#apply">学校入驻申请</a>
        <a href="#platform-login">平台管理员入口</a>
      </div>
    </AuthBlock>
  )
}
