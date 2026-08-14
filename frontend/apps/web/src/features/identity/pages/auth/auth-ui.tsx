// auth-ui 是认证页共用的深色呈现原语:无盒字段(等宽大写标签 + 底线输入)、就近错误、
// 表单级错误条、验证码字段、主落点按钮、页尾弱化通路、成功态与品牌章。
// 八个认证页共处同一块墨色底层(FE-6 底层全裸露),同一种元素在各页必须是同一个写法 ——
// 各页各抄一遍类名的后果是间距、强调级别与焦点态慢慢长成八套。
// 校验规则与就近错误状态在 auth-form.ts,本文件只管呈现。
// 仅认证域使用;日常光面表单走 @chaimir/ui 的 FormField(浅色语境)。

import React from 'react'
import { Link } from 'react-router'
import { CircleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Button, BrandLockup, BrandSeal, Icon, Input, TenantCrest, buttonVariants, cn } from '@chaimir/ui'
import type { InputProps } from '@chaimir/ui'

/** 页尾弱化通路的统一样式:返回登录、切换登录方式、退出登录共用一套(层级靠留白密度,不画分隔线) */
const QUIET_ACTION_CLASS =
  'mt-5 flex min-h-11 w-full items-center justify-center gap-1.5 rounded-md text-sm text-on-dark-sub transition-colors duration-fast hover:text-on-dark focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2'

/** 主落点(落印级)的统一样式:通栏、字距放宽,与表单末字段保持一个呼吸 */
const PRIMARY_ACTION_CLASS = 'mt-7 w-full'

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
        className="font-mono text-xs uppercase text-on-dark-sub"
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

export interface AuthBrandMarkProps {
  /** large 桌面品牌区的放大字号;窄屏与特权通道用常规字号 */
  large?: boolean
  /** subtitle 锁定组合下的副标题:默认平台全称,特权通道在此标明这是平台管理通道 */
  subtitle?: string
}

/**
 * AuthBrandMark 认证页品牌区:锁定组合 + 副标题,入场走显影(§4.5)。
 * 这里只出现一次锁定组合(规范 §1.3),不并排放品牌章 ——
 * 章与主标志是同源两态(开口 vs 缺口合上),同屏摆两个等于把同一个记忆点讲两遍;
 * 章与它的落印动效留给 AuthSuccess 这类「已确权、不可逆」回执。
 *
 * 学校私有部署另有一条学校身份行(SchoolBrandLine):锁定组合讲这是什么产品,
 * 那一行讲这是哪所学校的部署。平台托管不显示 —— 登录页面对的学校还未确定。
 */
export function AuthBrandMark({ large, subtitle = '区块链教学实验竞赛平台' }: AuthBrandMarkProps) {
  return (
    <div className="animate-develop">
      <BrandLockup
        variant={large ? 'wide' : 'narrow'}
        markClassName="text-accent"
        className="text-on-dark"
      />
      <span className="mt-1.5 block font-mono text-xs text-on-dark-sub">{subtitle}</span>
    </div>
  )
}

export interface SchoolBrandLineProps {
  /** 学校显示名 */
  name: string
  /** 校徽的内联地址;为空时徽记位回落学校名首字 */
  logoSrc: string
}

/**
 * SchoolBrandLine 在认证页显示学校徽记与校名。
 * 徽记用 sm 档而不是与锁定组合等大:锁定组合是主标识,学校身份是这台部署的归属,
 * 两个等大的图形会互相抢焦点(规范 §1.3 不堆砌品牌图形)。
 */
export function SchoolBrandLine({ name, logoSrc }: SchoolBrandLineProps) {
  return (
    <div className="flex items-center gap-2.5">
      <TenantCrest name={name} logoSrc={logoSrc} size="sm" onDark />
      <span className="truncate text-sm text-on-dark">{name}</span>
    </div>
  )
}
