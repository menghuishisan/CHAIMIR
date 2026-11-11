// accountSearch.test.ts 验证按姓名或学工号找账号:不分大小写,开头匹配排在前,超出上限时报剩余条数。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { matchAccounts } from './accountSearch.ts'

const accounts = [
  { id: '1', name: '王李', no: '2021001' },
  { id: '2', name: '李四', no: '2021002' },
  { id: '3', name: '张三', no: 'T001' },
  { id: '4', name: '赵五' },
]

const ids = (result: { list: { id: string }[] }) => result.list.map((account) => account.id)

test('没有关键字时按原顺序取前几个', () => {
  const result = matchAccounts(accounts, '  ', 3)
  assert.deepEqual(ids(result), ['1', '2', '3'])
  assert.equal(result.more, 1)
})

test('按姓名或学工号匹配,开头匹配排在前', () => {
  assert.deepEqual(ids(matchAccounts(accounts, '李', 10)), ['2', '1'])
  assert.deepEqual(ids(matchAccounts(accounts, '2021', 10)), ['1', '2'])
  assert.deepEqual(ids(matchAccounts(accounts, '002', 10)), ['2'])
})

test('学工号不分大小写,没有学工号的只按姓名', () => {
  assert.deepEqual(ids(matchAccounts(accounts, 't00', 10)), ['3'])
  assert.deepEqual(ids(matchAccounts(accounts, '赵', 10)), ['4'])
  assert.deepEqual(matchAccounts(accounts, '孙', 10), { list: [], more: 0 })
})

test('匹配超过上限时只给前几个并报剩余条数', () => {
  const result = matchAccounts(accounts, '20', 1)
  assert.deepEqual(ids(result), ['1'])
  assert.equal(result.more, 1)
})
