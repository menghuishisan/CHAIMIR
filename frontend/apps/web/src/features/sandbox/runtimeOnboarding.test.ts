// runtimeOnboarding.test.ts 验证运行时接入进度:已经走到第几步、卡在哪一步、下一步写什么。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { runtimeOnboarding } from './runtimeOnboarding.ts'

// 取值与 @chaimir/api-client 一致:状态 1 可用 / 2 接入中 / 3 已停用;自检 1 未自检 / 2 通过 / 3 未通过
const AVAILABLE = 1
const ONBOARDING = 2
const DISABLED = 3
const PENDING = 1
const PASSED = 2
const FAILED = 3

test('可用且自检通过时四步走完,没有下一步', () => {
  assert.deepEqual(runtimeOnboarding({ status: AVAILABLE, selftest: PASSED, availableImages: 2 }), {
    done: 4,
  })
})

test('没有可用镜像时停在登记镜像', () => {
  assert.deepEqual(
    runtimeOnboarding({ status: ONBOARDING, selftest: PENDING, availableImages: 0 }),
    {
      done: 1,
      next: '下一步:登记镜像版本',
    }
  )
})

test('有镜像未自检时下一步是运行自检;自检未通过时标出失败的一步', () => {
  assert.deepEqual(
    runtimeOnboarding({ status: ONBOARDING, selftest: PENDING, availableImages: 1 }),
    {
      done: 2,
      next: '下一步:运行自检',
    }
  )
  assert.deepEqual(
    runtimeOnboarding({ status: ONBOARDING, selftest: FAILED, availableImages: 1 }),
    {
      done: 2,
      failedIndex: 2,
      next: '自检没有通过,查看原因后重新自检',
    }
  )
})

test('自检通过但还没开放时,优先写服务端给出的原因', () => {
  assert.deepEqual(
    runtimeOnboarding({ status: ONBOARDING, selftest: PASSED, availableImages: 1 }),
    {
      done: 3,
      next: '自检已通过,还没有开放给学校',
    }
  )
  assert.deepEqual(
    runtimeOnboarding({
      status: ONBOARDING,
      selftest: PASSED,
      availableImages: 1,
      disabledReason: '尚未声明标准链能力',
    }),
    { done: 3, next: '尚未声明标准链能力' }
  )
})

test('已停用时进度照算,下一步说明已停用', () => {
  assert.deepEqual(runtimeOnboarding({ status: DISABLED, selftest: PASSED, availableImages: 1 }), {
    done: 3,
    next: '已停用,不会分配给学校',
  })
})

test('未读到当前镜像数时，不借用历史自检结果推断镜像可用', () => {
  for (const status of [AVAILABLE, ONBOARDING]) {
    for (const selftest of [PENDING, PASSED, FAILED]) {
      assert.deepEqual(runtimeOnboarding({ status, selftest }), {
        done: 1,
        next: '先确认镜像版本是否可用',
      })
    }
  }
})

test('已通过自检但没有可用镜像时，不能标为已开放', () => {
  assert.deepEqual(runtimeOnboarding({ status: AVAILABLE, selftest: PASSED, availableImages: 0 }), {
    done: 1,
    next: '下一步:登记镜像版本',
  })
})

test('已有可用状态与自检证明不能覆盖当前的不可用原因', () => {
  assert.deepEqual(
    runtimeOnboarding({
      status: AVAILABLE,
      selftest: PASSED,
      availableImages: 1,
      disabledReason: '缺少链能力实现',
    }),
    { done: 3, next: '缺少链能力实现' }
  )
})
