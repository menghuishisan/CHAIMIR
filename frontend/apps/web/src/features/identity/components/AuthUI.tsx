// AuthUI 是 identity 领域内认证页共用的深色呈现原语:字段(标签 + 有底色的深色输入框)、就近错误、
// 表单级错误条、验证码字段、密码规则清单、步骤条、方式切换、主落点按钮、页尾弱化通路与成功态。
// 八个认证页共处同一块墨色底层(FE-6 底层全裸露),同一种元素在各页必须是同一个写法 ——
// 各页各抄一遍类名的后果是间距、强调级别与焦点态慢慢长成八套。
// 校验规则与就近错误状态在 authForm.ts,本文件只管呈现。
// 仅认证域使用;日常光面表单走 @chaimir/ui 的 FormField(浅色语境)。

import React from 'react'
import { Link } from 'react-router'
import { Check, CircleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button, BrandSeal, Icon, Input, buttonVariants, cn } from '@chaimir/ui'
import type { InputProps } from '@chaimir/ui'
import { AuthBrandMark, SchoolBrandLine } from '../../../components/AuthBrand'
import { PASSWORD_RULE_TEXT, passwordRuleStates } from '../passwordRules'
import { useTenantBrand } from '../useTenantBrand'

/** 页尾弱化通路的统一样式:返回登录、切换登录方式、退出登录共用一套(层级靠留白密度,不画分隔线) */
const QUIET_ACTION_CLASS =
  'mt-5 flex min-h-11 w-full items-center justify-center gap-1.5 rounded-md text-sm text-on-dark-sub transition-colors duration-fast hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2'

/** 主落点的统一样式:通栏,与表单末字段保持一个呼吸 */
const PRIMARY_ACTION_CLASS = 'mt-7 w-full'

/** 次要文字链接(忘记密码、激活账号等)的统一样式:可点区触屏达标 */
export const AUTH_TEXT_LINK_CLASS =
  'hit-target relative rounded-sm text-sm text-on-dark-sub transition-colors duration-fast hover:text-accent focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2'

export interface AuthPanelProps {
  children: React.ReactNode
}

/**
 * AuthPanel 认证页的内容容器(已由 AuthLayout 居中,本组件不再负责布局)。
 * 内容宽度归内层自己(常规表单 max-w-sm,字段更多的入驻申请 max-w-md)。
 * AuthLayout 已统一处理左右分栏、居中对齐与路由切换动画,本容器只包裹内容。
 */
export function AuthPanel({ children }: AuthPanelProps) {
  return <div className="w-full">{children}</div>
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
        className="text-sm font-medium text-on-dark-sub"
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
 * AuthTextField 是认证页的文本字段:深色输入框(高 44) + 就近错误联动错误态边框。
 * 错误态不另传一个布尔值 —— 有错误文案就是错误态,两者不可能不一致。
 */
export function AuthTextField({ label, id, error, onValueChange, ...rest }: AuthTextFieldProps) {
  return (
    <AuthField label={label} htmlFor={id} error={error}>
      <Input
        id={id}
        variant="dark"
        className="h-11"
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
      <div className="flex items-stretch gap-3">
        <Input
          id={id}
          variant="dark"
        className="h-11"
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
          loading={sending}
          disabled={cooldownSeconds > 0}
          onClick={onSend}
          className="h-11 shrink-0"
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
 * AuthSubmit 表单主落点(玉实底,通栏)。
 * 八页的提交按钮只有文案不同,变体与间距在这里定死。
 *
 * 为什么是玉而不是朱砂(§1.2):登录按钮是认证页最大的一块色块,
 * 朱砂只留给品牌章与落印类动作,主 CTA 用朱砂会把「玉:朱砂 9:1」的克制比例反过来。
 */
export function AuthSubmit({ children, loading }: AuthSubmitProps) {
  return (
    <Button type="submit" variant="primary" size="lg" loading={loading} className={PRIMARY_ACTION_CLASS}>
      {children}
    </Button>
  )
}

export interface AuthPrimaryLinkProps {
  to: string
  children: React.ReactNode
}

/**
 * AuthPrimaryLink 主落点的跳转形态(成功态的「前往登录」)。
 * 与 AuthSubmit 同一视觉,但落在 <a> 上 —— 它是一次导航而不是一次提交,
 * 保留链接语义(可中键新开、可复制地址),按钮样式取自设计系统同一份变体定义。
 */
export function AuthPrimaryLink({ to, children }: AuthPrimaryLinkProps) {
  return (
    <Link
      to={to}
      className={cn(buttonVariants({ variant: 'primary', size: 'lg' }), PRIMARY_ACTION_CLASS, 'flex')}
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
 * AuthSuccess 提交成功后的整页回执:品牌章落印 + 结果说明 + 去处。
 * 表单已经消失,故这里不保留任何输入痕迹,只讲清「成了什么、接下来去哪」。
 * 用品牌章而非对勾图标:入驻申请、改密这类回执是「已确权、不可逆」,正是落印语义(§4.5);
 * 对勾只表示「这一步过了」,承载不了不可逆。
 */
export function AuthSuccess({ title, description, children }: AuthSuccessProps) {
  return (
    <div className="w-full max-w-sm animate-rise">
      <BrandSeal size="md" animated label="已完成" className="text-seal" />
      <AuthHeading title={title} description={description} />
      {children}
    </div>
  )
}

/**
 * AuthFormBrand 表单栏顶部的品牌行:窄屏左侧介绍区收起时由它补上品牌(宽屏隐藏,品牌在左侧介绍区);
 * 学校私有部署在品牌下方多一行校徽与校名。always 用于特权通道:无论宽窄都显示,标明通道用途。
 */
export function AuthFormBrand({ subtitle, always = false }: { subtitle?: string; always?: boolean }) {
  const brand = useTenantBrand()
  return (
    <div className={cn('mb-10 flex flex-col gap-3', !always && 'lg:hidden')}>
      <AuthBrandMark subtitle={subtitle} />
      {brand ? <SchoolBrandLine name={brand.display_name} logoSrc={brand.logo_image} /> : null}
    </div>
  )
}

/**
 * PasswordRuleChecklist 设置密码时的实时规则清单:满足一条勾一条,代替提交后才报错(设计 §7.1)。
 * 规则与后端同一口径(passwordRules.ts)。清单是辅助说明:勾选状态经读屏文字表达,不只靠颜色。
 */
export function PasswordRuleChecklist({ value, id }: { value: string; id?: string }) {
  const states = passwordRuleStates(value)
  return (
    <ul id={id} className="mt-2.5 flex flex-wrap gap-x-4 gap-y-1.5 text-sm" aria-label="密码要求">
      {PASSWORD_RULE_TEXT.map((rule) => {
        const met = states[rule.key]
        return (
          <li
            key={rule.key}
            className={cn('flex items-center gap-1.5 transition-colors duration-fast', met ? 'text-accent' : 'text-on-dark-sub')}
          >
            <span
              aria-hidden="true"
              className={cn(
                'flex size-4 items-center justify-center rounded-full border',
                met ? 'border-accent bg-accent text-substrate' : 'border-dark-control',
              )}
            >
              {met ? <Icon icon={Check} size="xs" /> : null}
            </span>
            {rule.text}
            <span className="sr-only">{met ? '(已满足)' : '(未满足)'}</span>
          </li>
        )
      })}
    </ul>
  )
}

/**
 * AuthSteps 两步以上表单的步骤条(找回密码):当前步玉色,已完成的带勾,当前步 aria-current="step"。
 */
export function AuthSteps({ steps, current }: { steps: string[]; current: number }) {
  return (
    <ol className="mt-6 flex items-center gap-3" aria-label="步骤">
      {steps.map((label, index) => {
        const done = index < current
        const active = index === current
        return (
          <li key={label} className="flex min-w-0 items-center gap-3" aria-current={active ? 'step' : undefined}>
            {index > 0 ? <span aria-hidden="true" className="h-px w-6 shrink-0 bg-dark-control" /> : null}
            <span className="flex min-w-0 items-center gap-2">
              <span
                aria-hidden="true"
                className={cn(
                  'flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold',
                  done && 'bg-accent text-substrate',
                  active && 'border border-accent text-accent',
                  !done && !active && 'border border-dark-control text-on-dark-sub',
                )}
              >
                {done ? <Icon icon={Check} size="xs" /> : index + 1}
              </span>
              <span className={cn('truncate text-sm', active ? 'font-medium text-on-dark' : 'text-on-dark-sub')}>
                {label}
                {done ? <span className="sr-only">(已完成)</span> : null}
              </span>
            </span>
          </li>
        )
      })}
    </ol>
  )
}

export interface AuthSegmentedOption<T extends string> {
  value: T
  label: string
}

/**
 * AuthSegmented 深色底层上的分段切换(密码 / 验证码):一组互斥的按钮,aria-pressed 表达选中。
 * 不用设计系统 SegmentedControl:它是纸面配色,放到墨色底层上对比关系会反过来。
 */
export function AuthSegmented<T extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string
  options: AuthSegmentedOption<T>[]
  value: T
  onChange: (next: T) => void
}) {
  return (
    <div className="mt-6 flex gap-1 rounded-md bg-dark-elevated p-1" role="group" aria-label={label}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          onClick={() => onChange(option.value)}
          className={cn(
            'flex min-h-9 flex-1 items-center justify-center rounded-sm text-sm transition-colors duration-fast pointer-coarse:min-h-11',
            'focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
            value === option.value ? 'bg-dark-surface font-semibold text-accent' : 'text-on-dark-sub hover:text-on-dark',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}

/**
 * AuthLinkRow 表单底部一行次要入口(如「首次使用?激活账号 · 忘记密码 · 学校申请入驻」),按使用频率排列。
 */
export function AuthLinkRow({ lead, children }: { lead?: string; children: React.ReactNode }) {
  const items = React.Children.toArray(children).filter(Boolean)
  return (
    <p className="mt-8 flex flex-wrap items-center justify-center gap-x-2 gap-y-1.5 text-sm text-on-dark-sub">
      {lead ? <span>{lead}</span> : null}
      {items.map((item, index) => (
        <React.Fragment key={index}>
          {index > 0 ? (
            <span aria-hidden="true" className="text-on-dark-faint">
              ·
            </span>
          ) : null}
          {item}
        </React.Fragment>
      ))}
    </p>
  )
}
