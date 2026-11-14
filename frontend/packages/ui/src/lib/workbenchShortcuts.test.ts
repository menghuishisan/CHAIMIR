// workbenchShortcuts.test.ts 验证工作台快捷键的匹配:修饰键精确匹配,单键先让给焦点所在的控件,带 Ctrl/⌘ 的组合在输入框里仍然生效。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  classifyShortcutFocus,
  createShortcutRegistry,
  matchShortcut,
  resolveShortcut,
  shortcutLabel,
  type ShortcutKeyEvent,
  type ShortcutTarget,
  type WorkbenchShortcut,
} from './workbenchShortcuts.ts'

const event = (key: string, extra: Partial<ShortcutKeyEvent> = {}): ShortcutKeyEvent => ({
  key,
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  repeat: false,
  focus: 'none',
  ...extra,
})

test('单键快捷键:键名不分大小写,带修饰键时不匹配', () => {
  assert.equal(matchShortcut({ key: 'j' }, event('j')), true)
  assert.equal(matchShortcut({ key: 'j' }, event('J')), true)
  assert.equal(matchShortcut({ key: 'j' }, event('j', { ctrlKey: true })), false)
  assert.equal(matchShortcut({ key: 'j' }, event('j', { altKey: true })), false)
  assert.equal(matchShortcut({ key: 'j' }, event('k')), false)
})

test('Shift 组合与单键是两个快捷键', () => {
  assert.equal(matchShortcut({ key: 'ArrowLeft', shift: true }, event('ArrowLeft', { shiftKey: true })), true)
  assert.equal(matchShortcut({ key: 'ArrowLeft', shift: true }, event('ArrowLeft')), false)
  assert.equal(matchShortcut({ key: 'ArrowLeft' }, event('ArrowLeft', { shiftKey: true })), false)
})

test('Ctrl/⌘ 组合:Windows 上是 Ctrl,Mac 上是 ⌘,任一即可', () => {
  assert.equal(matchShortcut({ key: 'Enter', mod: true }, event('Enter', { ctrlKey: true })), true)
  assert.equal(matchShortcut({ key: 'Enter', mod: true }, event('Enter', { metaKey: true })), true)
  assert.equal(matchShortcut({ key: 'Enter', mod: true }, event('Enter')), false)
})

test('焦点在输入里:单键都让给输入,Ctrl/⌘ 组合仍然生效', () => {
  assert.equal(matchShortcut({ key: 'ArrowLeft' }, event('ArrowLeft', { focus: 'editable' })), false)
  assert.equal(matchShortcut({ key: 'j' }, event('j', { focus: 'editable' })), false)
  assert.equal(matchShortcut({ key: 's', mod: true }, event('s', { ctrlKey: true, focus: 'editable' })), true)
})

test('焦点在按钮上:空格与回车留给按钮自己,方向键仍是快捷键', () => {
  assert.equal(matchShortcut({ key: ' ' }, event(' ', { focus: 'control' })), false)
  assert.equal(matchShortcut({ key: 'Enter' }, event('Enter', { focus: 'control' })), false)
  assert.equal(matchShortcut({ key: 'ArrowRight' }, event('ArrowRight', { focus: 'control' })), true)
})

test('焦点在标签页、单选组、滑块上:方向键也留给控件切换', () => {
  assert.equal(matchShortcut({ key: 'ArrowRight' }, event('ArrowRight', { focus: 'composite' })), false)
  assert.equal(matchShortcut({ key: 'ArrowLeft', shift: true }, event('ArrowLeft', { shiftKey: true, focus: 'composite' })), false)
  assert.equal(matchShortcut({ key: ' ' }, event(' ', { focus: 'composite' })), false)
  assert.equal(matchShortcut({ key: 'j' }, event('j', { focus: 'composite' })), true)
})

test('快捷键的显示文字', () => {
  assert.equal(shortcutLabel({ key: 'Enter', mod: true }, false), 'Ctrl + Enter')
  assert.equal(shortcutLabel({ key: 'Enter', mod: true }, true), '⌘ + Enter')
  assert.equal(shortcutLabel({ key: 'ArrowLeft' }, false), '←')
  assert.equal(shortcutLabel({ key: 'ArrowRight', shift: true }, false), 'Shift + →')
  assert.equal(shortcutLabel({ key: ' ' }, false), '空格')
  assert.equal(shortcutLabel({ key: 's', mod: true }, false), 'Ctrl + S')
})


test('浮层打开或焦点在终端里:任何快捷键都不响应', () => {
  assert.equal(matchShortcut({ key: 'ArrowRight' }, event('ArrowRight', { focus: 'exclusive' })), false)
  assert.equal(matchShortcut({ key: 'Enter', mod: true }, event('Enter', { ctrlKey: true, focus: 'exclusive' })), false)
  assert.equal(matchShortcut({ key: 's', mod: true }, event('s', { ctrlKey: true, focus: 'exclusive' })), false)
})

const target = (extra: Partial<ShortcutTarget>): ShortcutTarget => ({
  tag: 'DIV',
  contentEditable: false,
  inEditor: false,
  inTerminal: false,
  inOverlay: false,
  ...extra,
})

test('焦点归类:输入框与代码编辑器算输入', () => {
  assert.equal(classifyShortcutFocus(target({ tag: 'TEXTAREA' })), 'editable')
  assert.equal(classifyShortcutFocus(target({ tag: 'SELECT' })), 'editable')
  assert.equal(classifyShortcutFocus(target({ tag: 'INPUT', type: 'text' })), 'editable')
  assert.equal(classifyShortcutFocus(target({ contentEditable: true })), 'editable')
  assert.equal(classifyShortcutFocus(target({ tag: 'TEXTAREA', inEditor: true })), 'editable')
  assert.equal(classifyShortcutFocus(target({ inEditor: true })), 'editable')
})

test('焦点归类:终端里的按键全部归终端(Ctrl + S、Ctrl + 回车都是 shell 要的)', () => {
  assert.equal(classifyShortcutFocus(target({ tag: 'TEXTAREA', inTerminal: true })), 'exclusive')
  assert.equal(classifyShortcutFocus(target({ inTerminal: true })), 'exclusive')
})

test('焦点归类:标签页、单选、滑块是组合控件,哪怕它本身是按钮', () => {
  assert.equal(classifyShortcutFocus(target({ tag: 'BUTTON', role: 'tab' })), 'composite')
  assert.equal(classifyShortcutFocus(target({ tag: 'BUTTON', role: 'radio' })), 'composite')
  assert.equal(classifyShortcutFocus(target({ tag: 'INPUT', type: 'range' })), 'composite')
  assert.equal(classifyShortcutFocus(target({ tag: 'INPUT', type: 'radio' })), 'composite')
  assert.equal(classifyShortcutFocus(target({ tag: 'SPAN', role: 'slider' })), 'composite')
})

test('焦点归类:按钮、链接、复选框是普通控件,其余不占键', () => {
  assert.equal(classifyShortcutFocus(target({ tag: 'BUTTON' })), 'control')
  assert.equal(classifyShortcutFocus(target({ tag: 'A' })), 'control')
  assert.equal(classifyShortcutFocus(target({ tag: 'INPUT', type: 'checkbox' })), 'control')
  assert.equal(classifyShortcutFocus(target({ tag: 'g', role: 'button' })), 'control')
  assert.equal(classifyShortcutFocus(target({ tag: 'BODY' })), 'none')
  assert.equal(classifyShortcutFocus(target({ tag: 'DIV' })), 'none')
})

test('焦点归类:弹窗、菜单、下拉里的按键全部归浮层', () => {
  assert.equal(classifyShortcutFocus(target({ tag: 'INPUT', type: 'text', inOverlay: true })), 'exclusive')
  assert.equal(classifyShortcutFocus(target({ tag: 'BUTTON', inOverlay: true })), 'exclusive')
})

const declare = (description: string, extra: Partial<WorkbenchShortcut> = {}): WorkbenchShortcut => ({
  key: 'Enter',
  mod: true,
  description,
  run: () => {},
  ...extra,
})

test('登记处:各处声明按登记先后汇总,注销后移除', () => {
  const registry = createShortcutRegistry()
  registry.set('topbar', [declare('判分')])
  registry.set('editor', [declare('写入当前文件', { key: 's' })])
  assert.deepEqual(registry.all().map((item) => item.description), ['判分', '写入当前文件'])
  registry.set('topbar', [declare('提交报告')])
  assert.deepEqual(registry.snapshot().map((item) => item.description), ['提交报告', '写入当前文件'])
  registry.remove('editor')
  assert.deepEqual(registry.all().map((item) => item.description), ['提交报告'])
})

test('登记处:只换动作不通知列表,键位、说明或可用性变了才通知', () => {
  const registry = createShortcutRegistry()
  let notified = 0
  const unsubscribe = registry.subscribe(() => {
    notified += 1
  })
  registry.set('topbar', [declare('判分')])
  assert.equal(notified, 1)
  const listed = registry.snapshot()

  const run = () => undefined
  registry.set('topbar', [declare('判分', { run })])
  assert.equal(notified, 1)
  assert.equal(registry.snapshot(), listed)
  assert.equal(registry.all()[0].run, run)

  registry.set('topbar', [declare('判分', { disabled: true })])
  assert.equal(notified, 2)
  assert.notEqual(registry.snapshot(), listed)

  unsubscribe()
  registry.remove('topbar')
  assert.equal(notified, 2)
  assert.deepEqual(registry.snapshot(), [])
})

test('处理按键:没有命中的按键交还浏览器', () => {
  const result = resolveShortcut([declare('判分')], event('Enter'))
  assert.equal(result.claimed, false)
  assert.equal(result.run, undefined)
})

test('处理按键:暂时不可用的快捷键照样拦下,但不执行', () => {
  const result = resolveShortcut([declare('写入当前文件', { key: 's', disabled: true })], event('s', { ctrlKey: true }))
  assert.equal(result.claimed, true)
  assert.equal(result.run, undefined)
})

test('处理按键:按住不放时,声明可连续执行的才重复;其余照样拦下(不弹出另存网页)', () => {
  const write = declare('写入当前文件', { key: 's' })
  const held = resolveShortcut([write], event('s', { ctrlKey: true, repeat: true }))
  assert.equal(held.claimed, true)
  assert.equal(held.run, undefined)
  assert.equal(resolveShortcut([write], event('s', { ctrlKey: true })).run, write.run)

  const forward = declare('前进一局', { key: 'ArrowRight', mod: false, repeat: true })
  assert.equal(resolveShortcut([forward], event('ArrowRight', { repeat: true })).run, forward.run)
})

test('处理按键:同一键位登记了多处,执行第一个可用的', () => {
  const blocked = declare('提交答案', { disabled: true })
  const open = declare('判分')
  assert.equal(resolveShortcut([blocked, open], event('Enter', { ctrlKey: true })).run, open.run)
})
