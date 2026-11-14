// pageLocation.test.ts 验证详情页在顶栏里的对象名:读取中显示占位、读到后是对象名、读取失败或不存在时用通用名称。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { locationLabel } from './pageLocation.ts'

const pick = (course: { name: string }) => course.name

test('读取中显示占位,即使还留着上一个对象的数据', () => {
  assert.equal(locationLabel({ status: 'loading', data: { name: '上一门课' } }, pick, '课程详情'), undefined)
})

test('读到后是对象名', () => {
  assert.equal(locationLabel({ status: 'success', data: { name: '区块链导论' } }, pick, '课程详情'), '区块链导论')
})

test('读取失败、不存在或名字为空时用通用名称', () => {
  assert.equal(locationLabel({ status: 'error', data: null }, pick, '课程详情'), '课程详情')
  assert.equal(locationLabel({ status: 'empty', data: null }, pick, '课程详情'), '课程详情')
  assert.equal(locationLabel({ status: 'success', data: { name: '' } }, pick, '课程详情'), '课程详情')
})
