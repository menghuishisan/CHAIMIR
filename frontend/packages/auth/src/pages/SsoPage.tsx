// SsoPage：学校 CAS 跳转与 LDAP 登录入口。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { Building2, LockKeyhole, UserRound } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { readFrontendConfig } from '@chaimir/shared'
import { runSubmit, useFormState, valueOf } from '../form-state'
import { handleLoginResponse } from '../session-routing'
import { AuthBlock } from '../components/AuthBlock'
import { TextField } from '../components/TextField'

/**
 * SsoPage 对接后端 CAS 跳转和 LDAP 登录，账号名单仍由学校管理员维护。
 */
export function SsoPage({ api, config }: { api: ChaimirApi; config: ReturnType<typeof readFrontendConfig> }): React.ReactElement {
  const [state, setState] = useFormState()

  const ldapSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      const response = await api.identity.ldapLogin(valueOf(values, 'tenant_code'), {
        username: valueOf(values, 'username'),
        password: valueOf(values, 'password'),
      })
      handleLoginResponse(response, config)
      return '统一认证已通过，正在进入平台'
    })
  }

  const casLogin = async () => {
    await runSubmit(setState, async (values) => {
      const service = window.location.origin + window.location.pathname
      const result = await api.identity.getCASLoginUrl(valueOf(values, 'tenant_code'), service)
      window.location.assign(result.redirect_url)
      return '正在跳转到学校统一认证'
    })
  }

  return (
    <AuthBlock title="学校统一认证" description="CAS 跳转或 LDAP 登录只验证身份，平台仍按已导入名单放行。" state={state}>
      <form className="public-form" onSubmit={ldapSubmit}>
        <TextField icon={<Building2 size={17} />} name="tenant_code" label="学校短码" value={state.values.tenant_code} onChange={setState} required />
        <TextField icon={<UserRound size={17} />} name="username" label="统一认证账号" value={state.values.username} onChange={setState} autoComplete="username" required />
        <TextField icon={<LockKeyhole size={17} />} name="password" label="统一认证密码" type="password" value={state.values.password} onChange={setState} autoComplete="current-password" required />
        <div className="public-button-row">
          <Button type="button" variant="outline" block onClick={casLogin}>CAS 登录</Button>
          <Button type="submit" block loading={state.loading}>LDAP 登录</Button>
        </div>
      </form>
    </AuthBlock>
  )
}
