// allPages.test.ts 验证「按每页最大条数取完」:跨页拼接、取到总数即停、遇到空页即停(防止总数与实际不符时无限请求)。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { collectAllPages } from './allPages.ts'

/** fakeLoader 按页切一个数组,并记下请求过的页码 */
function fakeLoader(items: number[], reportedTotal = items.length) {
  const pages: number[] = []
  const loader = async ({ page, size }: { page: number; size: number }) => {
    pages.push(page)
    return { list: items.slice((page - 1) * size, page * size), total: reportedTotal, page, size }
  }
  return { loader, pages }
}

test('跨页拼接全部记录,取到总数即停', async () => {
  const { loader, pages } = fakeLoader(Array.from({ length: 250 }, (_, index) => index))
  const all = await collectAllPages(loader, 100)
  assert.equal(all.length, 250)
  assert.deepEqual(pages, [1, 2, 3])
})

test('一页就够时只请求一次', async () => {
  const { loader, pages } = fakeLoader([1, 2, 3])
  assert.deepEqual(await collectAllPages(loader, 100), [1, 2, 3])
  assert.deepEqual(pages, [1])
})

test('总数虚高时:不满一页就是最后一页', async () => {
  const { loader, pages } = fakeLoader([1, 2, 3], 500)
  assert.deepEqual(await collectAllPages(loader, 2), [1, 2, 3])
  assert.deepEqual(pages, [1, 2])
})

test('总数虚高且恰好整页时:遇到空页即停', async () => {
  const { loader, pages } = fakeLoader([1, 2], 500)
  assert.deepEqual(await collectAllPages(loader, 2), [1, 2])
  assert.deepEqual(pages, [1, 2])
})
