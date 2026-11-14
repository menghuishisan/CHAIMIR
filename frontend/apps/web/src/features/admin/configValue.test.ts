// configValue.test.ts 验证配置变更的「改前 / 改后」对照:只列变化的键,凭据不显示明文,新增与删除写清。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { changeRows } from './configValue.ts'

test('只列出变化的键,按键名排序', () => {
  assert.deepEqual(changeRows({ a: 1, b: 2, c: 'x' }, { a: 1, b: 3, c: 'y' }), [
    { key: 'b', before: '2', after: '3' },
    { key: 'c', before: 'x', after: 'y' },
  ])
})

test('新增和删除的键写清楚', () => {
  assert.deepEqual(changeRows({ old: true }, { added: 'v' }), [
    { key: 'added', before: '未设置', after: 'v' },
    { key: 'old', before: 'true', after: '未设置' },
  ])
})

test('凭据字段不显示明文,只说明改过', () => {
  assert.deepEqual(changeRows({ api_token: 'a' }, { api_token: 'b' }), [
    { key: 'api_token', before: '已配置(不显示)', after: '已配置(不显示)' },
  ])
  assert.deepEqual(changeRows({}, { db_password: 'x' }), [
    { key: 'db_password', before: '未设置', after: '已配置(不显示)' },
  ])
})

test('结构类的值给出概要', () => {
  assert.deepEqual(changeRows({ list: [1, 2] }, { list: [1, 2, 3] }), [
    { key: 'list', before: '2 项', after: '3 项' },
  ])
})

test('内容没变时没有对照行', () => {
  assert.deepEqual(changeRows({ a: { b: 1 } }, { a: { b: 1 } }), [])
})
