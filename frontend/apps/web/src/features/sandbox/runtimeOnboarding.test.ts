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

test('列表里不知道镜像数时,按自检结果推断镜像这一步', () => {
  assert.deepEqual(runtimeOnboarding({ status: ONBOARDING, selftest: PENDING }), {
    done: 1,
    next: '下一步:登记镜像版本并运行自检',
  })
  assert.deepEqual(runtimeOnboarding({ status: ONBOARDING, selftest: FAILED }), {
    done: 2,
    failedIndex: 2,
    next: '自检没有通过,查看原因后重新自检',
  })
  assert.deepEqual(runtimeOnboarding({ status: AVAILABLE, selftest: PASSED }), { done: 4 })
})
