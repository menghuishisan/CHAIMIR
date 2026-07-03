// 表单状态工具：认证公共页统一处理字段更新、提交反馈和用户向错误。

import React, { useState } from 'react'
import { toUserFacingError } from '@chaimir/shared'
import type { FormState, FormValues } from './types'

/**
 * useFormState 创建公共页表单状态，集中管理加载、成功和失败反馈。
 */
export function useFormState(): [FormState, React.Dispatch<React.SetStateAction<FormState>>] {
  return useState<FormState>({ values: {}, loading: false })
}

/**
 * runSubmit 包装异步提交，把错误转换为用户向文案和 trace_id。
 */
export async function runSubmit(
  setState: React.Dispatch<React.SetStateAction<FormState>>,
  submit: (values: FormValues) => Promise<string>
): Promise<void> {
  setState((current) => ({ ...current, loading: true, error: undefined, message: undefined }))
  try {
    const values = await new Promise<FormValues>((resolve) => {
      setState((current) => {
        resolve(current.values)
        return current
      })
    })
    const message = await submit(values)
    setState((current) => ({ ...current, loading: false, message }))
  } catch (error) {
    const userError = toUserFacingError(error)
    const message = userError.traceId ? `${userError.message} 如需帮助，请提供编号 ${userError.traceId}。` : userError.message
    setState((current) => ({ ...current, loading: false, error: message }))
  }
}

/**
 * updateField 写入单个字段并保留其他表单状态。
 */
export function updateField(setState: React.Dispatch<React.SetStateAction<FormState>>, name: string, value: string): void {
  setState((current) => ({ ...current, values: { ...current.values, [name]: value } }))
}

/**
 * valueOf 读取必填字段，空值时抛出用户可理解的错误。
 */
export function valueOf(values: FormValues, key: string): string {
  const value = values[key]?.trim()
  if (!value) {
    throw new Error('请补全必填信息后再提交')
  }
  return value
}

/**
 * numberOf 将编号字段转换为数字，非法值交由用户修正。
 */
export function numberOf(values: FormValues, key: string): number {
  const parsed = Number(valueOf(values, key))
  if (!Number.isFinite(parsed)) {
    throw new Error('编号格式不正确，请检查后重试')
  }
  return parsed
}

/**
 * safeParseTenants 解析后端返回的学校候选，不影响主登录流程。
 */
export function safeParseTenants(raw: string): Array<{ tenant_id: string; name: string; code: string }> {
  try {
    const parsed = JSON.parse(raw) as unknown
    return Array.isArray(parsed)
      ? parsed.filter((item): item is { tenant_id: string; name: string; code: string } => Boolean(item && typeof item === 'object'))
      : []
  } catch {
    return []
  }
}
