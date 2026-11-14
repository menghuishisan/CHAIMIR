// organizationTree.test.ts 组织树(教师组织查看 / 校管组织架构共用):
// 搜索按院系名或代码、专业名、班级名收窄树;地址里的选中节点不存在时回到默认(第一个有专业的院系的第一个专业);
// 专业下的班级在读在前、入学年份新的在前,搜索词命中班级名时只列命中的班级。

import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildOrgTree, classesOfMajor, resolveOrgSelection } from './organizationTree.ts'

const departments = [
  { id: 'd1', name: '计算机学院', code: 'CS' },
  { id: 'd2', name: '经济学院', code: 'ECON' },
  { id: 'd3', name: '艺术学院', code: 'ART' },
]
const majors = [
  { id: 'm1', department_id: 'd1', name: '区块链工程' },
  { id: 'm2', department_id: 'd1', name: '软件工程' },
  { id: 'm3', department_id: 'd2', name: '金融科技' },
]
const classes = [
  { id: 'c1', major_id: 'm1', name: '区块链 2201', enrollment_year: 2022, status: 1 },
  { id: 'c2', major_id: 'm1', name: '区块链 2401', enrollment_year: 2024, status: 1 },
  { id: 'c3', major_id: 'm1', name: '区块链 2001', enrollment_year: 2020, status: 2 },
  { id: 'c4', major_id: 'm3', name: '金科 2301', enrollment_year: 2023, status: 1 },
]

test('没有搜索词:全部院系与专业,带每个专业的班级数', () => {
  const tree = buildOrgTree(departments, majors, classes, '')
  assert.deepEqual(tree.map((node) => node.department.id), ['d1', 'd2', 'd3'])
  assert.deepEqual(tree[0].majors.map((node) => [node.major.id, node.classCount]), [['m1', 3], ['m2', 0]])
  assert.deepEqual(tree[2].majors, [])
})

test('搜索:命中院系时带上全部专业;命中专业或班级时只留对应专业;大小写不敏感', () => {
  assert.deepEqual(buildOrgTree(departments, majors, classes, 'cs').map((node) => [node.department.id, node.majors.length]), [['d1', 2]])
  assert.deepEqual(buildOrgTree(departments, majors, classes, '软件').map((node) => node.majors.map((m) => m.major.id)), [['m2']])
  assert.deepEqual(buildOrgTree(departments, majors, classes, '2301').map((node) => node.majors.map((m) => m.major.id)), [['m3']])
  assert.deepEqual(buildOrgTree(departments, majors, classes, '不存在'), [])
})

test('选中节点:地址里的专业、院系存在就用,否则回到第一个有专业的院系的第一个专业', () => {
  const tree = buildOrgTree(departments, majors, classes, '')
  assert.deepEqual(resolveOrgSelection(tree, { dept: '', major: 'm3' }), { kind: 'major', departmentId: 'd2', majorId: 'm3' })
  assert.deepEqual(resolveOrgSelection(tree, { dept: 'd3', major: '' }), { kind: 'department', departmentId: 'd3' })
  assert.deepEqual(resolveOrgSelection(tree, { dept: '', major: 'gone' }), { kind: 'major', departmentId: 'd1', majorId: 'm1' })
  assert.deepEqual(resolveOrgSelection(tree, { dept: '', major: '' }), { kind: 'major', departmentId: 'd1', majorId: 'm1' })
})

test('选中节点:没有专业时选第一个院系,树为空时没有选中', () => {
  const onlyDepartments = buildOrgTree(departments, [], [], '')
  assert.deepEqual(resolveOrgSelection(onlyDepartments, { dept: '', major: '' }), { kind: 'department', departmentId: 'd1' })
  assert.equal(resolveOrgSelection([], { dept: 'd1', major: 'm1' }), undefined)
})

test('专业下的班级:在读在前、入学年份新的在前;搜索词命中班级名时只列命中的', () => {
  assert.deepEqual(classesOfMajor(classes, 'm1', '').list.map((entity) => entity.id), ['c2', 'c1', 'c3'])
  assert.deepEqual(classesOfMajor(classes, 'm1', '2201'), { list: [classes[0]], filtered: true })
  // 搜索词没命中班级(命中的是专业名)或全部命中时列全部
  assert.equal(classesOfMajor(classes, 'm1', '区块链工程').filtered, false)
  assert.equal(classesOfMajor(classes, 'm1', '区块链').filtered, false)
})
