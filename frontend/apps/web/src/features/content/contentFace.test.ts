// contentFace.test.ts 题面展示:只把已登记的题面字段换成用户看得懂的小节,不显示字段名,
// 也不显示答案、判题配置、攻击步骤与断言这类不该出现在题面里的内容(即使接口漏下了)。

import assert from 'node:assert/strict'
import { test } from 'node:test'
import { contentFaceSections } from './contentFace.ts'

test('理论题:题干、选项按登记顺序输出,数组按行', () => {
  const sections = contentFaceSections({ options: ['A', 'B'], statement: '哪个对?', q_type: 'single_choice', max_score: 10 })
  assert.deepEqual(sections, [
    { key: 'statement', label: '题干', lines: ['哪个对?'] },
    { key: 'options', label: '选项', lines: ['A', 'B'] },
  ])
})

test('竞赛题与实验模板的题面字段', () => {
  assert.deepEqual(
    contentFaceSections({ scenario: '链上金库', statement: '取出全部资金', summary: '部署并调用', steps: ['部署', '调用'] }).map(
      (section) => section.label,
    ),
    ['概述', '操作步骤', '场景', '题干'],
  )
})

test('漏洞题:显示说明与合约源码,不显示初始化步骤、攻击步骤与判定断言', () => {
  const sections = contentFaceSections({
    description: '重入漏洞',
    contract_source: 'contract Vault {}',
    fork_block: 19000000,
    init_steps: [{ op: 'deploy' }],
    positive_steps: [{ op: 'call' }],
    assertions: [{ label: '余额为零' }],
  })
  assert.deepEqual(sections.map((section) => section.key), ['description', 'contract_source', 'fork_block'])
  assert.deepEqual(sections[2].lines, ['19000000'])
})

test('答案、解析、判题配置、未登记字段和空值都不输出', () => {
  assert.deepEqual(
    contentFaceSections({ answer: 'A', explanation: '因为', judge_config: { x: 1 }, unknown_field: 'x', statement: '  ', steps: [] }),
    [],
  )
})
