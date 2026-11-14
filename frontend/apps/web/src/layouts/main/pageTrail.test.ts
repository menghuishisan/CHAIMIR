// pageTrail.test.ts 验证顶栏「当前位置」与浏览器标题的计算:侧栏入口、详情页读取中、不在侧栏里的页、最长前缀匹配。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { pageTitle, resolvePageTrail } from './pageTrail.ts'

const student = [
  { name: '我的课程', path: '/student/courses' },
  { name: '实验实训', path: '/student/experiments' },
]
const teacher = [
  { name: '课程管理', path: '/teacher/courses' },
  { name: '实验编排', path: '/teacher/experiments' },
  { name: '赛事组织', path: '/teacher/contests' },
]

test('侧栏一级页不声明时只显示自己的名称', () => {
  assert.deepEqual(resolvePageTrail(student, '/student/courses', null), [{ label: '我的课程' }])
})

test('详情页读取中:侧栏入口是可点的上级,当前层级是占位', () => {
  assert.deepEqual(resolvePageTrail(student, '/student/courses/42', [{ label: undefined }]), [
    { label: '我的课程', to: '/student/courses' },
    { label: undefined },
  ])
})

test('详情页读到对象名后接在侧栏入口之后', () => {
  assert.deepEqual(resolvePageTrail(student, '/student/courses/42', [{ label: '区块链导论' }]), [
    { label: '我的课程', to: '/student/courses' },
    { label: '区块链导论' },
  ])
})

test('声明的第一项就是侧栏入口时不重复', () => {
  const declared = [
    { label: '实验编排', to: '/teacher/experiments' },
    { label: '链上投票', to: '/teacher/experiments/7' },
    { label: '报告与小组' },
  ]
  assert.deepEqual(resolvePageTrail(teacher, '/teacher/experiments/7/reports', declared), declared)
})

test('不在侧栏里的页按声明原样显示', () => {
  const declared = [{ label: '赛事组织', to: '/teacher/contests' }, { label: '漏洞题工坊' }]
  assert.deepEqual(resolvePageTrail(teacher, '/teacher/vuln-workshop', declared), declared)
})

test('更深的页没有声明时,侧栏入口作为可点的上级', () => {
  assert.deepEqual(resolvePageTrail(student, '/student/courses/42/lessons/9', null), [
    { label: '我的课程', to: '/student/courses' },
  ])
})

test('按路径段匹配,最长前缀胜出', () => {
  const nav = [
    { name: '课程', path: '/x/courses' },
    { name: '课程设置', path: '/x/courses/settings' },
  ]
  assert.deepEqual(resolvePageTrail(nav, '/x/courses/settings', null), [{ label: '课程设置' }])
  assert.deepEqual(resolvePageTrail(nav, '/x/courses-archive', null), [])
})

test('浏览器标题取最后一个有名字的层级', () => {
  assert.equal(pageTitle([{ label: '我的课程', to: '/student/courses' }, { label: '区块链导论' }]), '区块链导论 · Chaimir')
  assert.equal(pageTitle([{ label: '我的课程', to: '/student/courses' }, { label: undefined }]), '我的课程 · Chaimir')
  assert.equal(pageTitle([{ label: undefined }]), 'Chaimir')
  assert.equal(pageTitle([]), 'Chaimir')
})
