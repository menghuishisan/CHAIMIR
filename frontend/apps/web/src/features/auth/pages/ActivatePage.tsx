// ActivatePage：一次性激活码开通账号入口。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { LockKeyhole, ShieldCheck } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { runSubmit, useFormState, valueOf } from '../form-state'
import { AuthBlock } from '../components/AuthBlock'
import { TextField } from '../components/TextField'

/**
 * ActivatePage 使用一次性激活码开通账号并设置初始密码。
 */
export function ActivatePage({ api }: { api: ChaimirApi }): React.ReactElement {
  const [state, setState] = useFormState()
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      await api.identity.activate({
        activation_code: valueOf(values, 'activation_code'),
        password: valueOf(values, 'password'),
      })
      return '账号已激活，请返回登录页使用新密码登录'
    })
  }

  return (
    <AuthBlock title="激活账号" description="输入学校管理员发放的一次性激活码并设置密码。" state={state}>
      <form className="public-form" onSubmit={submit}>
        <TextField icon={<ShieldCheck size={17} />} name="activation_code" label="激活码" value={state.values.activation_code} onChange={setState} required />
        <TextField icon={<LockKeyhole size={17} />} name="password" label="登录密码" type="password" value={state.values.password} onChange={setState} autoComplete="new-password" required />
        <Button type="submit" size="lg" block loading={state.loading}>激活账号</Button>
      </form>
    </AuthBlock>
  )
}
