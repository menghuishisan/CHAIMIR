// urlQuery.test.ts 验证地址参数读写:不认识的值回到默认值、写回时去掉默认值并清掉页码、页码只接受正整数。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readPage, readQuery, writeQuery } from './urlQuery.ts'

const schema = {
  status: { defaultValue: 'all', allowed: ['all', 'running', 'ended'] },
  q: { defaultValue: '' },
}

test('缺失或不认识的值回到默认值', () => {
  assert.deepEqual(readQuery(new URLSearchParams(''), schema), { status: 'all', q: '' })
  assert.deepEqual(readQuery(new URLSearchParams('status=bogus'), schema), { status: 'all', q: '' })
})

test('认识的值原样读出,搜索词去掉首尾空白', () => {
  assert.deepEqual(readQuery(new URLSearchParams('status=running&q=%20链%20'), schema), {
    status: 'running',
    q: '链',
  })
})

test('写回时去掉默认值、清掉页码、保留别的参数', () => {
  const next = writeQuery(new URLSearchParams('tab=members&status=running&page=3'), schema, { status: 'all' }, ['page'])
  assert.equal(next.toString(), 'tab=members')
})

test('写回非默认值并且不改动传入的参数对象', () => {
  const current = new URLSearchParams('page=2')
  const next = writeQuery(current, schema, { status: 'ended', q: '共识' }, ['page'])
  assert.equal(next.get('status'), 'ended')
  assert.equal(next.get('q'), '共识')
  assert.equal(next.has('page'), false)
  assert.equal(current.toString(), 'page=2')
})

test('页码只接受正整数,其余都回到第一页', () => {
  for (const raw of ['abc', '0', '-2', '2.5', '', '1e3']) {
    assert.equal(readPage(new URLSearchParams(`page=${raw}`), 'page'), 1, raw)
  }
  assert.equal(readPage(new URLSearchParams(''), 'page'), 1)
  assert.equal(readPage(new URLSearchParams('page=3'), 'page'), 3)
})
