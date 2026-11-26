// 组合连接检查键变化时,旧结果必须失效。

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { currentConnectionCheck } from './compositionCheck.ts'
import { compositionSpecFromDeclaration, declarationFromSpec, reconcileRuntimeNames } from './composition.ts'
import { SANDBOX_ACCESS_PROFILE, type SandboxCompositionSpec } from '@chaimir/api-client'

test('保留与当前声明键匹配的连接检查结果', () => {
  const check = { key: 'current', complete: true }

  assert.equal(currentConnectionCheck(check, 'current'), check)
})

test('清除属于旧声明的连接检查结果', () => {
  assert.equal(currentConnectionCheck({ key: 'stale', complete: true }, 'current'), undefined)
})

test('编辑运行时保留链初始化、代码与脚本，并同步实例更名', () => {
  const base: SandboxCompositionSpec = {
    id: 'fixture', access_profile: SANDBOX_ACCESS_PROFILE.EXPERIMENT,
    workspace_runtime_instance: 'chain',
    runtimes: [{ instance_code: 'chain', runtime_code: 'evm', image_version: '1' }],
    init_code_ref: 'code', init_script_ref: 'script',
    init_chain_steps: [{ runtime_instance: 'chain', op: 'deploy', payload: { contract: 'fixture' } }],
  }
  const declaration = declarationFromSpec(base)
  assert.equal(declaration.runtimes[0].initializationBound, true)
  declaration.runtimes[0].instanceCode = 'renamed-chain'
  declaration.workspaceRuntimeInstance = 'renamed-chain'
  const updated = compositionSpecFromDeclaration({
    id: 'fixture', declaration, base, accessProfile: SANDBOX_ACCESS_PROFILE.EXPERIMENT,
  })
  assert.deepEqual(updated.init_chain_steps, [
    { runtime_instance: 'renamed-chain', op: 'deploy', payload: { contract: 'fixture' } },
  ])
  assert.equal(updated.init_code_ref, 'code')
  assert.equal(updated.init_script_ref, 'script')
  assert.equal(base.init_chain_steps?.[0].runtime_instance, 'chain')
})

test('实例清空后保留上次有效名称，完成时只映射有效名称', () => {
  let declaration = declarationFromSpec({
    runtimes: [
      { instance_code: 'left', runtime_code: 'evm', image_version: '1' },
      { instance_code: 'right', runtime_code: 'evm', image_version: '1' },
    ],
    workspace_runtime_instance: 'left',
    links: ['left', '', 'right'].map((target) => ({
      source_component: 'tool', source_endpoint: 'rpc', target_component: target,
      target_endpoint: '', protocol: '', required_at_start: false, access_scope: '', config_binding: '',
    })),
  })
  const clientId = declaration.runtimes[0].clientId
  for (const name of ['temporary', '', '   ', 'final']) {
    const result = reconcileRuntimeNames(declaration.runtimes,
      declaration.runtimes.map((runtime) => runtime.clientId === clientId ? { ...runtime, instanceCode: name } : runtime))
    declaration = { ...declaration, runtimes: result.runtimes }
    assert.equal(result.renames.has(''), false)
    assert.equal(declaration.runtimes[0].pendingInstanceCode, name.trim() ? undefined : 'temporary')
    assert.equal(declaration.runtimes[1].instanceCode, 'right')
  }
  assert.equal(declaration.runtimes[0].originalInstanceCode, 'left')
  assert.equal(declaration.runtimes[0].pendingInstanceCode, undefined)
})

test('同次互换两个实例名称按原引用逐一迁移', () => {
  const previous = declarationFromSpec({
    workspace_runtime_instance: 'left',
    runtimes: [
      { instance_code: 'left', runtime_code: 'evm', image_version: '1' },
      { instance_code: 'right', runtime_code: 'evm', image_version: '1' },
    ],
  })
  const result = reconcileRuntimeNames(previous.runtimes,
    previous.runtimes.map((runtime) => ({ ...runtime, instanceCode: runtime.instanceCode === 'left' ? 'right' : 'left' })))
  assert.deepEqual([...result.renames], [['left', 'right'], ['right', 'left']])
  assert.deepEqual(previous.runtimes.map((runtime) => runtime.instanceCode), ['left', 'right'])
})

test('重名中间态保留原引用，完成互换时一次性迁移', () => {
  let declaration = declarationFromSpec({
    workspace_runtime_instance: 'left',
    runtimes: [
      { instance_code: 'left', runtime_code: 'evm', image_version: '1' },
      { instance_code: 'right', runtime_code: 'evm', image_version: '1' },
    ],
  })
  const duplicate = reconcileRuntimeNames(declaration.runtimes,
    declaration.runtimes.map((runtime, index) => index === 0 ? { ...runtime, instanceCode: 'right' } : runtime))
  declaration = { ...declaration, runtimes: duplicate.runtimes }
  assert.equal(duplicate.renames.size, 0)
  assert.equal(declaration.runtimes[0].pendingInstanceCode, 'left')
  const swapped = reconcileRuntimeNames(declaration.runtimes,
    declaration.runtimes.map((runtime, index) => index === 1 ? { ...runtime, instanceCode: 'left' } : runtime))
  assert.deepEqual([...swapped.renames], [['left', 'right'], ['right', 'left']])
})
