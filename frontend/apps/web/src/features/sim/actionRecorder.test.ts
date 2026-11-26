import assert from 'node:assert/strict'
import { setTimeout as delay } from 'node:timers/promises'
import test from 'node:test'
import { createSimActionRecorder } from './actionRecorder'

const action = (event_type: string) => ({ at_tick: 0, event_type, payload: {} })

test('serializes concurrent user actions and assigns sequence after each commit', async () => {
  const saved: { seq: number; event_type: string }[] = []
  const committed: number[] = []
  let releaseFirst!: () => void
  const first = new Promise<void>((resolve) => (releaseFirst = resolve))
  const recorder = createSimActionRecorder(3, async ({ seq, event_type }) => {
    if (seq === 4) await first
    saved.push({ seq, event_type })
  }, ({ seq }) => committed.push(seq), () => assert.fail('unexpected report failure'))

  recorder(action('select'))
  recorder(action('broadcast'))
  await delay(0)
  assert.deepEqual(saved, [])
  releaseFirst()
  await delay(0)

  assert.deepEqual(saved, [{ seq: 4, event_type: 'select' }, { seq: 5, event_type: 'broadcast' }])
  assert.deepEqual(committed, [4, 5])
})

test('stops the sequence after the first failed write', async () => {
  const attempts: { seq: number; event_type: string }[] = []
  const failures: string[] = []
  const recorder = createSimActionRecorder(0, async ({ seq, event_type }) => {
    attempts.push({ seq, event_type })
    throw new Error('database unavailable')
  }, () => assert.fail('failed write must not be confirmed'), (_error, failed) => failures.push(failed.event_type))

  recorder(action('first'))
  recorder(action('second'))
  await delay(0)
  recorder(action('third'))
  await delay(0)

  assert.deepEqual(attempts, [{ seq: 1, event_type: 'first' }])
  assert.deepEqual(failures, ['first'])
})
