// auditDetail.test.ts 验证审计记录 detail 的整理:只展示登记过的项、按固定顺序、需要查表的项由调用方给出文案。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { auditDetailRows, auditObjectHint } from './auditDetail.ts'

test('解析失败或不是对象时没有内容', () => {
  assert.deepEqual(auditDetailRows(undefined), [])
  assert.deepEqual(auditDetailRows('not json'), [])
  assert.deepEqual(auditDetailRows('[1,2]'), [])
  assert.deepEqual(auditDetailRows('null'), [])
  assert.deepEqual(auditDetailRows('42'), [])
})

test('登记过的项按固定顺序展示,与写入顺序无关', () => {
  assert.deepEqual(auditDetailRows('{"version":3,"code":"Q-1"}'), [
    { term: '编码', value: 'Q-1', mono: true },
    { term: '版本', value: '第 3 版', mono: false },
  ])
})

test('内部编号与未登记的项不展示', () => {
  assert.deepEqual(auditDetailRows('{"course_id":"123","source_ref":"experiment:1","name":"区块链导论"}'), [
    { term: '名称', value: '区块链导论', mono: false },
  ])
})

test('需要查表的项只在调用方给出文案时展示', () => {
  assert.deepEqual(auditDetailRows('{"status":2}'), [])
  assert.deepEqual(auditDetailRows('{"status":2}', { status: (value) => (value === 2 ? '已停用' : undefined) }), [
    { term: '变更后的状态', value: '已停用', mono: false },
  ])
  assert.deepEqual(auditDetailRows('{"status":9}', { status: (value) => (value === 2 ? '已停用' : undefined) }), [])
  assert.deepEqual(auditDetailRows('{"student_id":"7"}', { student_id: () => '张三(2021001)' }), [
    { term: '学生', value: '张三(2021001)', mono: false },
  ])
})

test('布尔、文件大小、年份与班级数量', () => {
  assert.deepEqual(auditDetailRows('{"enabled":false,"size":2048,"enrollment_year":2024,"class_ids":["1","2"]}'), [
    { term: '文件大小', value: '2.0 KB', mono: false },
    { term: '入学年份', value: '2024 级', mono: false },
    { term: '涉及班级', value: '共 2 个', mono: false },
    { term: '是否启用', value: '否', mono: false },
  ])
})

test('类型不符或空值的项跳过', () => {
  assert.deepEqual(auditDetailRows('{"version":{},"name":"  ","success":"12","class_ids":[],"enabled":"yes"}'), [])
  assert.deepEqual(auditDetailRows('{"success":1200,"failed":0}'), [
    { term: '成功条数', value: '1,200', mono: false },
    { term: '失败条数', value: '0', mono: false },
  ])
})

test('对象提示:依次取名称、编码、题目编码、文件名', () => {
  assert.equal(auditObjectHint('{"name":"区块链导论","code":"C1"}'), '区块链导论')
  assert.equal(auditObjectHint('{"code":"Q-1","version":2}'), 'Q-1')
  assert.equal(auditObjectHint('{"item_code":"V-9"}'), 'V-9')
  assert.equal(auditObjectHint('{"file_name":"logo.png","size":10}'), 'logo.png')
  assert.equal(auditObjectHint('{"course_id":"1"}'), undefined)
  assert.equal(auditObjectHint('{"name":"  "}'), undefined)
  assert.equal(auditObjectHint(undefined), undefined)
})
