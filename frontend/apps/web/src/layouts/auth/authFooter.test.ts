// authFooter.test.ts 验证认证页页脚:平台管理员登录入口只在 SaaS 部署出现,在平台管理员登录页上换成回学校登录;
// 「联系本校管理员」只在面向师生的页面显示。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { authFooterContent } from './authFooter.ts'

const toPlatform = { to: '/auth/platform-login', label: '平台管理员登录' }
const toSchool = { to: '/auth/login', label: '学校账号登录' }

test('SaaS 下学校登录页:联系本校管理员 + 平台管理员登录入口', () => {
  assert.deepEqual(authFooterContent('/auth/login', true), { showSchoolHelp: true, link: toPlatform })
})

test('私有化部署没有平台管理员,不显示入口', () => {
  assert.deepEqual(authFooterContent('/auth/login', false), { showSchoolHelp: true, link: null })
  assert.deepEqual(authFooterContent('/auth/forgot', false), { showSchoolHelp: true, link: null })
})

test('平台管理员登录页:链回学校账号登录,不提本校管理员', () => {
  assert.deepEqual(authFooterContent('/auth/platform-login', true), { showSchoolHelp: false, link: toSchool })
})

test('入驻申请页:申请人还没有本校管理员,但仍能进入平台管理员登录;末尾斜杠也认', () => {
  assert.deepEqual(authFooterContent('/auth/apply/', true), { showSchoolHelp: false, link: toPlatform })
})

test('带参数的统一认证页是面向师生的页面', () => {
  assert.deepEqual(authFooterContent('/auth/sso/demo', true), { showSchoolHelp: true, link: toPlatform })
})
