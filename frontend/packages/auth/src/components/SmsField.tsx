// SmsField：公共认证表单的短信验证码输入与发送控件。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { KeyRound } from 'lucide-react'
import { Button } from '@chaimir/ui'
import type { FormState } from '../types'
import { numberOf, runSubmit, valueOf } from '../form-state'
import { TextField } from './TextField'

/**
 * SmsField 统一验证码发送和验证码输入，发送动作走 M1 限频接口。
 */
export function SmsField({
  api,
  state,
  setState,
  scene,
}: {
  api: ChaimirApi
  state: FormState
  setState: React.Dispatch<React.SetStateAction<FormState>>
  scene: number
}): React.ReactElement {
  const send = async () => {
    await runSubmit(setState, async (values) => {
      await api.identity.sendSMS({
        phone: valueOf(values, 'phone'),
        tenant_id: numberOf(values, 'tenant_id'),
        scene,
      })
      return '验证码已发送，请查看短信'
    })
  }

  return (
    <div className="public-code-row">
      <TextField icon={<KeyRound size={17} />} name="code" label="短信验证码" value={state.values.code} onChange={setState} required />
      <Button type="button" variant="outline" onClick={send} loading={state.loading}>发送验证码</Button>
    </div>
  )
}
