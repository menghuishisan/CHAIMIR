// ApplyPage：公开学校入驻申请入口。

import React from 'react'
import type { ChaimirApi } from '@chaimir/api-client'
import { Building2, Landmark, Mail, Phone, UserRound } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { numberOf, runSubmit, useFormState, valueOf } from '../form-state'
import { AuthBlock } from '../components/AuthBlock'
import { TextField } from '../components/TextField'

/**
 * ApplyPage 提交公开学校入驻申请，申请状态由后端持久化。
 */
export function ApplyPage({ api }: { api: ChaimirApi }): React.ReactElement {
  const [state, setState] = useFormState()
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await runSubmit(setState, async (values) => {
      await api.identity.createApplication({
        school_name: valueOf(values, 'school_name'),
        school_type: numberOf(values, 'school_type'),
        contact_name: valueOf(values, 'contact_name'),
        contact_phone: valueOf(values, 'contact_phone'),
        contact_email: valueOf(values, 'contact_email'),
      })
      return '入驻申请已提交，审核结果会发送给联系人'
    })
  }

  return (
    <AuthBlock title="学校入驻申请" description="这是学校开通申请，不是个人账号注册。" state={state}>
      <form className="public-form" onSubmit={submit}>
        <TextField icon={<Landmark size={17} />} name="school_name" label="学校名称" value={state.values.school_name} onChange={setState} required />
        <TextField icon={<Building2 size={17} />} name="school_type" label="学校类型编号" value={state.values.school_type} onChange={setState} required />
        <TextField icon={<UserRound size={17} />} name="contact_name" label="联系人姓名" value={state.values.contact_name} onChange={setState} required />
        <TextField icon={<Phone size={17} />} name="contact_phone" label="联系人手机号" value={state.values.contact_phone} onChange={setState} autoComplete="tel" required />
        <TextField icon={<Mail size={17} />} name="contact_email" label="联系人邮箱" type="email" value={state.values.contact_email} onChange={setState} autoComplete="email" required />
        <Button type="submit" size="lg" block loading={state.loading}>提交申请</Button>
      </form>
    </AuthBlock>
  )
}
