// judgerEnvironment.test.ts 验证判题器列表上的「判题环境」与「上次自测」说法(设计 §7.6 判题器)。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { judgerEnvironment, judgerSelftest } from './judgerEnvironment.ts'

// 取值与 @chaimir/api-client 一致:自检 1 未自检 / 2 通过 / 3 未通过;类型 6 为人工评分
const PENDING = 1
const PASSED = 2
const FAILED = 3
const MANUAL = 6
const TESTCASE = 1

test('不需要链环境的判题器没有判题环境这一项', () => {
  assert.equal(
    judgerEnvironment({ runtimeRequired: false, frozen: false, selftest: PENDING }),
    undefined
  )
})

test('需要链环境但还没有冻结时是未准备', () => {
  assert.deepEqual(judgerEnvironment({ runtimeRequired: true, frozen: false, selftest: PENDING }), {
    tone: 'warning',
    label: '判题环境尚未准备完成',
  })
})

test('已冻结时按自检结果区分:失败单独标出,冻结的环境事实不隐藏', () => {
  assert.deepEqual(judgerEnvironment({ runtimeRequired: true, frozen: true, selftest: PASSED }), {
    tone: 'success',
    label: '判题环境已冻结',
  })
  assert.deepEqual(judgerEnvironment({ runtimeRequired: true, frozen: true, selftest: PENDING }), {
    tone: 'neutral',
    label: '判题环境已冻结,还没有自测',
  })
  assert.deepEqual(judgerEnvironment({ runtimeRequired: true, frozen: true, selftest: FAILED }), {
    tone: 'danger',
    label: '判题环境已冻结,自测失败',
  })
})

test('上次自测:人工评分不需要自测,其余按结果', () => {
  assert.deepEqual(judgerSelftest({ type: MANUAL, selftest: PENDING }), {
    tone: 'neutral',
    label: '人工评分不需要自测',
  })
  assert.deepEqual(judgerSelftest({ type: TESTCASE, selftest: PENDING }), {
    tone: 'neutral',
    label: '还没有自测',
  })
  assert.deepEqual(judgerSelftest({ type: TESTCASE, selftest: PASSED }), {
    tone: 'success',
    label: '自测通过',
  })
  assert.deepEqual(judgerSelftest({ type: TESTCASE, selftest: FAILED }), {
    tone: 'danger',
    label: '自测失败',
  })
})
