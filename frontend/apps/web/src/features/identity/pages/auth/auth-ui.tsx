// auth-ui 是认证页共用的深色呈现原语:无盒字段(等宽大写标签 + 底线输入)、就近错误、
// 表单级错误条、验证码字段、主落点按钮、页尾弱化通路、成功态与品牌章。
// 八个认证页共处同一块墨色底层(FE-6 底层全裸露),同一种元素在各页必须是同一个写法 ——
// 各页各抄一遍类名的后果是间距、强调级别与焦点态慢慢长成八套。
// 校验规则与就近错误状态在 auth-form.ts,本文件只管呈现。
// 仅认证域使用;日常光面表单走 @chaimir/ui 的 FormField(浅色语境)。

import React from 'react'
import { Link } from 'react-router'
import { CircleAlert, CircleCheck } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button, Icon, Input, buttonVariants, cn } from '@chaimir/ui'
import type { InputProps } from '@chaimir/ui'

/** 页尾弱化通路的统一样式:返回登录、切换登录方式、退出登录共用一套(层级靠留白密度,不画分隔线) */
const QUIET_ACTION_CLASS =
  'mt-5 flex w-full items-center justify-center gap-1.5 text-sm text-on-dark-sub transition-colors duration-fast hover:text-on-dark'

/** 主落点(落印级)的统一样式:通栏、字距放宽,与表单末字段保持一个呼吸 */
const PRIMARY_ACTION_CLASS = 'mt-7 w-full tracking-widest'

export interface AuthPanelProps {
  children: React.ReactNode
}

/**
 * AuthPanel 认证页的居中容器:填满 AuthLayout 让出的主区并把内容居中。
 * 内容宽度归内层自己(常规表单 max-w-sm,字段更多的入驻申请 max-w-md),
 * 容器只负责「站在页面中间」这一件事。
 */
export function AuthPanel({ children }: AuthPanelProps) {
  return <div className="flex w-full flex-1 items-center justify-center px-6 py-10">{children}</div>
}

export interface AuthFieldProps {
  label: string
  htmlFor: string
  /** 就近错误(blur 校验产物);出现时读屏经 role=alert 感知 */
  error?: string | null
  children: React.ReactNode
}

/**
 * AuthField 渲染标签 + 控件 + 就近错误,保持认证页的纵向节奏。
 * 直接使用它的只有非文本控件(如入驻申请页的机构类型下拉);文本输入走 AuthTextField。
 */
export function AuthField({ label, htmlFor, error, children }: AuthFieldProps) {
  return (
    <div className="mt-6">
      <label
        htmlFor={htmlFor}
        className="font-mono text-xs uppercase tracking-widest text-on-dark-sub"
      >
        {label}
      </label>
      <div className="mt-2">{children}</div>
      {error ? (
        <p role="alert" className="mt-1.5 text-xs text-on-dark-danger">
          {error}
        </p>
      ) : null}
    </div>
  )
}

export interface AuthTextFieldProps
  extends Omit<InputProps, 'id' | 'variant' | 'invalid' | 'onChange'> {
  label: string
  id: string
  error?: string | null
  /** onValueChange 直接收到输入值:认证页没有需要读原生事件的字段 */
  onValueChange: (value: string) => void
}

/**
 * AuthTextField 是认证页的文本字段:底线输入 + 就近错误联动错误态边框。
 * 错误态不另传一个布尔值 —— 有错误文案就是错误态,两者不可能不一致。
 */
export function AuthTextField({ label, id, error, onValueChange, ...rest }: AuthTextFieldProps) {
  return (
    <AuthField label={label} htmlFor={id} error={error}>
      <Input
        id={id}
        variant="underline"
        invalid={Boolean(error)}
        onChange={(event) => onValueChange(event.target.value)}
        {...rest}
      />
    </AuthField>
  )
}

export interface AuthSmsFieldProps {
  id: string
  value: string
  error?: string | null
  onValueChange: (value: string) => void
  onBlur: () => void
  /** cooldownSeconds 距离可重发的剩余秒数(useSmsCooldown 产物);大于 0 时按钮改说还要等多久 */
  cooldownSeconds: number
  sending: boolean
  onSend: () => void
}

/**
 * AuthSmsField 验证码字段:输入框与发送按钮同行,按钮承担「能不能再发、还要等多久」的表达。
 * 登录页与找回密码页用的是同一个字段,只有发送时调用的场景不同。
 */
export function AuthSmsField({
  id,
  value,
  error,
  onValueChange,
  onBlur,
  cooldownSeconds,
  sending,
  onSend,
}: AuthSmsFieldProps) {
  return (
    <AuthField label="验证码" htmlFor={id} error={error}>
      <div className="flex items-end gap-3">
        <Input
          id={id}
          variant="underline"
          inputMode="numeric"
          autoComplete="one-time-code"
          value={value}
          invalid={Boolean(error)}
          onChange={(event) => onValueChange(event.target.value)}
          onBlur={onBlur}
        />
        <Button
          type="button"
          variant="on-dark"
          size="sm"
          loading={sending}
          disabled={cooldownSeconds > 0}
          onClick={onSend}
          className="shrink-0"
        >
          {cooldownSeconds > 0 ? `${cooldownSeconds} 秒后可重发` : '发送验证码'}
        </Button>
      </div>
    </AuthField>
  )
}

export interface AuthFormErrorProps {
  message: string | null
}

/**
 * AuthFormError 表单级错误条(提交失败等);message 为空时不渲染。
 * 文案是后端裁决结果的用户向表达(经 userFacingErrorMessage 收敛),本组件不加工。
 */
export function AuthFormError({ message }: AuthFormErrorProps) {
  if (!message) return null
  return (
    <div
      role="alert"
      className="mt-5 flex items-start gap-2 rounded-md bg-danger/15 px-3 py-2.5 text-sm text-on-dark-danger"
    >
      <Icon icon={CircleAlert} size="sm" className="mt-0.5" />
      <span>{message}</span>
    </div>
  )
}

export interface AuthHeadingProps {
  title: string
  description?: string
}

/** AuthHeading 表单区标题层级 */
export function AuthHeading({ title, description }: AuthHeadingProps) {
  return (
    <div>
      <h1 className="text-2xl font-bold text-on-dark">{title}</h1>
      {description ? <p className="mt-1.5 text-sm text-on-dark-sub">{description}</p> : null}
    </div>
  )
}

export interface AuthSubmitProps {
  children: React.ReactNode
  loading?: boolean
}

/**
 * AuthSubmit 表单主落点(朱砂落印,通栏)。
 * 八页的提交按钮只有文案不同,变体与间距在这里定死。
 */
export function AuthSubmit({ children, loading }: AuthSubmitProps) {
  return (
    <Button type="submit" variant="seal" size="lg" loading={loading} className={PRIMARY_ACTION_CLASS}>
      {children}
    </Button>
  )
}

export interface AuthPrimaryLinkProps {
  to: string
  children: React.ReactNode
}

/**
 * AuthPrimaryLink 落印级别的跳转落点(成功态的「前往登录」)。
 * 与 AuthSubmit 同一视觉,但落在 <a> 上 —— 它是一次导航而不是一次提交,
 * 保留链接语义(可中键新开、可复制地址),按钮样式取自设计系统同一份变体定义。
 */
export function AuthPrimaryLink({ to, children }: AuthPrimaryLinkProps) {
  return (
    <Link
      to={to}
      className={cn(buttonVariants({ variant: 'seal', size: 'lg' }), PRIMARY_ACTION_CLASS, 'flex')}
    >
      {children}
    </Link>
  )
}

export interface AuthQuietLinkProps {
  to: string
  icon?: LucideIcon
  children: React.ReactNode
}

/**
 * AuthQuietLink 页尾弱化通路(返回登录等)。
 */
export function AuthQuietLink({ to, icon, children }: AuthQuietLinkProps) {
  return (
    <Link to={to} className={QUIET_ACTION_CLASS}>
      {icon ? <Icon icon={icon} size="sm" /> : null}
      {children}
    </Link>
  )
}

export interface AuthQuietActionProps {
  icon?: LucideIcon
  onClick: () => void
  children: React.ReactNode
}

/**
 * AuthQuietAction 页尾弱化动作(切换登录方式、退出登录)。
 * 与 AuthQuietLink 同一视觉:两者对用户是同一层级的「走开」,只有一个换地址、一个换状态。
 */
export function AuthQuietAction({ icon, onClick, children }: AuthQuietActionProps) {
  return (
    <button type="button" onClick={onClick} className={QUIET_ACTION_CLASS}>
      {icon ? <Icon icon={icon} size="sm" /> : null}
      {children}
    </button>
  )
}

export interface AuthSuccessProps {
  title: string
  description: string
  /** children 是完成后的去处(强调落点或弱化通路,由该页的下一步是否明确决定) */
  children: React.ReactNode
}

/**
 * AuthSuccess 提交成功后的整页回执:玉色对勾 + 结果说明 + 去处。
 * 表单已经消失,故这里不保留任何输入痕迹,只讲清「成了什么、接下来去哪」。
 */
export function AuthSuccess({ title, description, children }: AuthSuccessProps) {
  return (
    <div className="w-full max-w-sm animate-rise">
      <Icon icon={CircleCheck} size="lg" className="text-accent" />
      <AuthHeading title={title} description={description} />
      {children}
    </div>
  )
}

export interface AuthBrandMarkProps {
  /** large 桌面品牌区的放大字号;窄屏与特权通道用常规字号 */
  large?: boolean
  /** subtitle 章下副标题:默认平台全称,特权通道在此标明这是平台管理通道 */
  subtitle?: string
}

/**
 * AuthBrandMark 品牌章 + 名称;入场时印章「盖」下并散开一圈墨晕(§4.5 落印,只在入场一次)。
 * 朱砂只用在这枚章与落印动作上(规范 §1.1),认证页的其余强调一律用玉色。
 */
export function AuthBrandMark({ large, subtitle = '区块链教学实验竞赛平台' }: AuthBrandMarkProps) {
  return (
    <div className="flex items-center gap-3">
      <span className="relative">
        <span
          className={cn(
            'flex items-center justify-center rounded-lg bg-seal font-bold text-on-solid shadow-md animate-seal-drop',
            large ? 'h-9 w-9 text-lg' : 'h-8 w-8 text-base',
          )}
        >
          链
        </span>
        <span
          aria-hidden
          className="absolute -inset-0.5 rounded-lg border border-seal/60 animate-seal-ring"
          style={{ animationDelay: 'calc(var(--t-stagger) * 3)' }}
        />
      </span>
      <span>
        <span className="block text-md font-bold leading-tight text-on-dark">Chaimir</span>
        <span className="block font-mono text-xs tracking-widest text-on-dark-sub">{subtitle}</span>
      </span>
    </div>
  )
}
