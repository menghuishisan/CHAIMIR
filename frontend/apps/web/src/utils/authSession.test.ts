// 验证登录目标消费、角色边界与凭证存储约束，不启动浏览器或认证服务。
import assert from 'node:assert/strict'
import { after, beforeEach, test } from 'node:test'
import { AccountStatus, BaseIdentity, UserRole, type LoginResponse } from '@chaimir/api-client'
import {
  clearLoginTokens,
  clearLoginTokensLocally,
  completeLogin,
  completeRequiredPasswordChange,
  expireLoginSession,
  getStoredAccessToken,
  isPasswordChangeRequired,
  rememberCurrentEntryPath,
} from './authSession'

const destination = '/student/contests/55/join?view=team#invite'
const storage = new Map<string, string>()
const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
const originalChannel = Object.getOwnPropertyDescriptor(globalThis, 'BroadcastChannel')
const login: LoginResponse = {
  access_token: 'access-secret',
  account: {
    id: '7', name: '学生', base_identity: BaseIdentity.STUDENT,
    roles: [UserRole.STUDENT], status: AccountStatus.ACTIVE,
  },
}

function setLocation(path: string): void {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      location: new URL(path, 'https://school.example'),
      sessionStorage: {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => storage.set(key, value),
        removeItem: (key: string) => storage.delete(key),
      },
    },
  })
}

beforeEach(() => {
  Object.defineProperty(globalThis, 'BroadcastChannel', { configurable: true, value: undefined })
  setLocation('/auth/login')
  clearLoginTokensLocally()
  storage.clear()
})

after(() => {
  if (originalWindow) Object.defineProperty(globalThis, 'window', originalWindow)
  else Reflect.deleteProperty(globalThis, 'window')
  if (originalChannel) Object.defineProperty(globalThis, 'BroadcastChannel', originalChannel)
  else Reflect.deleteProperty(globalThis, 'BroadcastChannel')
})

test('普通登录进入角色首页，访问令牌只在内存中', () => {
  assert.equal(completeLogin(login), '/student/courses')
  assert.equal(getStoredAccessToken(), 'access-secret')
  assert.equal(storage.size, 0)
})

test('受保护地址跨认证失效保留，登录成功后只消费一次', () => {
  setLocation(destination)
  rememberCurrentEntryPath()
  expireLoginSession()
  assert.equal(getStoredAccessToken(), null)
  assert.equal(completeLogin(login), destination)
  assert.equal(completeLogin(login), '/student/courses')
  assert.equal(storage.size, 0)
})

test('必要改密完成前保留目标，成功后清除拦截与目标', () => {
  setLocation(destination)
  rememberCurrentEntryPath()
  assert.equal(completeLogin({ ...login, must_change_pwd: true }), '/auth/change-pwd')
  assert.equal(isPasswordChangeRequired(), true)
  assert.equal(completeRequiredPasswordChange([UserRole.STUDENT]), destination)
  assert.equal(isPasswordChangeRequired(), false)
  assert.equal(storage.size, 0)
})

for (const unsafe of [
  'https://evil.example/student/contests',
  '//evil.example/student/contests',
  'javascript:alert(1)',
  '/teacher/courses',
  '/student/../teacher/courses',
  '/student/%2e%2e/platform-admin/schools',
  '/student/\\evil.example',
  '/student/\ncontests',
]) {
  test(`登录拒绝不安全或跨角色目标 ${JSON.stringify(unsafe)}`, () => {
    storage.set('chaimir.pending_entry_path', unsafe)
    assert.equal(completeLogin(login), '/student/courses')
    assert.equal(storage.size, 0)
  })
}

test('多角色账号可回到任一已授予角色范围', () => {
  storage.set('chaimir.pending_entry_path', '/teacher/courses/66')
  assert.equal(completeLogin({
    ...login,
    account: { ...login.account!, roles: [UserRole.SCHOOL_ADMIN, UserRole.TEACHER] },
  }), '/teacher/courses/66')
})

test('改密后按当前角色重新验证目标', () => {
  storage.set('chaimir.pending_entry_path', '/platform-admin/schools')
  storage.set('chaimir.must_change_password', 'true')
  assert.equal(completeRequiredPasswordChange([UserRole.STUDENT]), '/student/courses')
  assert.equal(storage.size, 0)
})

test('认证页票据不能覆盖原目标或进入存储', () => {
  setLocation(destination)
  rememberCurrentEntryPath()
  setLocation('/auth/sso/school?ticket=cas-secret')
  rememberCurrentEntryPath()
  assert.doesNotMatch(JSON.stringify([...storage]), /cas-secret|access-secret/)
  assert.equal(completeLogin(login), destination)
})

test('主动退出清除目标、凭据与改密状态', () => {
  setLocation(destination)
  rememberCurrentEntryPath()
  completeLogin({ ...login, must_change_pwd: true })
  clearLoginTokens()
  assert.equal(getStoredAccessToken(), null)
  assert.equal(isPasswordChangeRequired(), false)
  assert.equal(storage.size, 0)
  assert.equal(completeLogin(login), '/student/courses')
})
