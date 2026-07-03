// PlatformLoginPage：SaaS 平台管理员专用登录入口。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { KeyRound, UserRound } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { readFrontendConfig } from '@chaimir/shared'
import { runSubmit, useFormState, valueOf } from '../form-state'
import { handleLoginResponse } from '../session-routing'
import { AuthBlock } from '../components/AuthBlock'
import { TextField } from '../components/TextField'

/**
 * PlatformLoginPage 对接 SaaS 平台管理员专用登录接口。
 */
export function PlatformLoginPage({ api, config }: { api: ChaimirApi; config: ReturnType<typeof readFrontendConfig> }): React.ReactElement {
  const [state, setState] = useFormState()
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      const response = await api.identity.loginPlatform({
        username: valueOf(values, 'username'),
        password: valueOf(values, 'password'),
      })
      handleLoginResponse(response, config)
      return '登录成功，正在进入平台管理端'
    })
  }

  return (
    <AuthBlock title="平台管理员登录" description="仅 SaaS 平台管理员使用，学校师生请返回普通登录入口。" state={state}>
      <form className="public-form" onSubmit={submit}>
        <TextField icon={<UserRound size={17} />} name="username" label="管理员账号" value={state.values.username} onChange={setState} autoComplete="username" required />
        <TextField icon={<KeyRound size={17} />} name="password" label="密码" type="password" value={state.values.password} onChange={setState} autoComplete="current-password" required />
        <Button type="submit" size="lg" block loading={state.loading}>进入平台管理端</Button>
      </form>
      <div className="public-links"><a href="#login">返回学校用户登录</a></div>
    </AuthBlock>
  )
}
