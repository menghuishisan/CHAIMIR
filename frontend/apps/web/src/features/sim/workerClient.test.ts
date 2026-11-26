import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import { setTimeout as delay } from 'node:timers/promises'
import test, { type TestContext } from 'node:test'
const { SimWorkerClient, validateTeachingFrame, isRuntimeFrame, isSimPackageDescriptor } = createRequire(import.meta.url)('@chaimir/sim-sdk') as typeof import('@chaimir/sim-sdk')

test('teaching frames reject missing explanation and pattern data before rendering', () => {
  const frame = { summary: '初态', phase: { id: 'initial', title: '初态', intent: 'observe', explanation: { what: '观察', why: '验证' } }, focus: { primary: ['flow'] }, layout: { primary: 'flow' }, patterns: [{ id: 'flow', mode: 'pipeline', title: '流程', data: { steps: [], currentStepId: '' } }] }
  for (const missing of ['explanation', 'data']) {
    const invalid = structuredClone(frame)
    if (missing === 'explanation') Reflect.deleteProperty(invalid.phase, missing)
    else Reflect.deleteProperty(invalid.patterns[0]!, missing)
    assert.ok(validateTeachingFrame(invalid as import('@chaimir/sim-sdk').TeachingFrame).length > 0, missing)
  }
})

test('runtime contract accepts seven visual modes and rejects incomplete snapshots', () => {
  const fixture = JSON.parse(readFileSync(new URL('../../../../../../backend/internal/modules/sim/testdata/runtime_protocol.json', import.meta.url), 'utf8'))
  assert.equal(isSimPackageDescriptor(fixture.descriptor), true)
  assert.equal(isRuntimeFrame(fixture.snapshot), true)
  for (const pattern of fixture.patterns) {
    const view = { ...fixture.snapshot.view, patterns: [pattern] }
    assert.deepEqual(validateTeachingFrame(view, { nodes: 8, maxTick: 8, maxEvents: 8 }), [], pattern.mode)
    assert.ok(validateTeachingFrame(view, { nodes: 0, maxTick: 0, maxEvents: 0 }).length > 0, pattern.mode)
    assert.ok(validateTeachingFrame({ ...view, patterns: [{ ...pattern, data: undefined }] }).length > 0, pattern.mode)
  }
  for (const field of ['state', 'view', 'interactionAvailability', 'checkpointResults']) {
    assert.equal(isRuntimeFrame({ ...fixture.snapshot, [field]: null }), false, field)
  }
  assert.equal(isRuntimeFrame({ ...fixture.snapshot, state: { ...fixture.snapshot.state, _trace: { triggeredLines: 'wrong', variables: {} } } }), false)
  assert.equal(isRuntimeFrame({ ...fixture.snapshot, checkpointResults: { done: { achieved: 'wrong' } } }), false)
  assert.equal(isRuntimeFrame({ ...fixture.snapshot, tick: -1 }), false)
  assert.equal(isRuntimeFrame({ ...fixture.snapshot, tick: 0.5 }), false)
})

class TestWorker {
  static response: 'accepted' | 'rejected' | 'pending' = 'accepted'
  onmessage?: (event: { data: unknown }) => void
  onerror?: (event: unknown) => void
  terminate() {}
  postMessage(request: { requestId: number }) {
    const response = TestWorker.response
    if (response === 'pending') return
    queueMicrotask(() => this.onmessage?.({ data: response === 'rejected'
      ? { type: 'error', requestId: request.requestId, message: '当前阶段不允许这个操作' }
      : { type: 'snapshot', requestId: request.requestId, snapshot: {}, event: {
          type: 'select', source: 'user', seq: 4, atTick: 7, payload: { target: 'node-2' }, target: 'node-2',
        } } }))
  }
}

function client(t: TestContext) {
  const original = globalThis.Worker
  globalThis.Worker = TestWorker as unknown as typeof Worker
  const runtime = new SimWorkerClient({ builtinCode: 'builtin__test', seed: 1, initParams: {}, commandTimeoutMs: 1000 })
  t.after(() => { runtime.destroy(); globalThis.Worker = original })
  return runtime
}

test('interaction confirmation returns the executed event and its authoritative tick', async (t) => {
  TestWorker.response = 'accepted'
  const event = await client(t).inject('select', {}, 'node-2')
  assert.deepEqual(event, { type: 'select', source: 'user', seq: 4, atTick: 7, payload: { target: 'node-2' }, target: 'node-2' })
})

test('rejected interactions produce no confirmed event', async (t) => {
  TestWorker.response = 'rejected'
  const confirmed: unknown[] = []
  await assert.rejects(client(t).inject('select').then((event) => confirmed.push(event)), /当前阶段不允许/)
  assert.deepEqual(confirmed, [])
})

test('destroy rejects pending commands and prevents commands against a terminated Worker', async (t) => {
  TestWorker.response = 'pending'
  const runtime = client(t)
  const result = runtime.inject('select').then(() => 'resolved', () => 'rejected')
  runtime.destroy()
  assert.equal(await Promise.race([result, delay(50).then(() => 'pending')]), 'rejected')
  await assert.rejects(runtime.inject('select'))
})
