// ForgotPage：通过短信验证码完成密码找回。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { Building2, LockKeyhole, Phone } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { numberOf, runSubmit, useFormState, valueOf } from '../form-state'
import { AuthBlock } from '../components/AuthBlock'
import { SmsField } from '../components/SmsField'
import { TextField } from '../components/TextField'

/**
 * ForgotPage 使用短信验证码完成找回密码流程。
 */
export function ForgotPage({ api }: { api: ChaimirApi }): React.ReactElement {
  const [state, setState] = useFormState()
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      await api.identity.resetPassword({
        phone: valueOf(values, 'phone'),
        code: valueOf(values, 'code'),
        new_password: valueOf(values, 'new_password'),
        tenant_id: numberOf(values, 'tenant_id'),
      })
      return '密码已重置，请使用新密码登录'
    })
  }

  return (
    <AuthBlock title="找回密码" description="通过已绑定手机号接收验证码并设置新密码。" state={state}>
      <form className="public-form" onSubmit={submit}>
        <TextField icon={<Phone size={17} />} name="phone" label="手机号" value={state.values.phone} onChange={setState} autoComplete="tel" required />
        <TextField icon={<Building2 size={17} />} name="tenant_id" label="学校编号" value={state.values.tenant_id} onChange={setState} required />
        <SmsField api={api} state={state} setState={setState} scene={2} />
        <TextField icon={<LockKeyhole size={17} />} name="new_password" label="新密码" type="password" value={state.values.new_password} onChange={setState} autoComplete="new-password" required />
        <Button type="submit" size="lg" block loading={state.loading}>重置密码</Button>
      </form>
    </AuthBlock>
  )
}
