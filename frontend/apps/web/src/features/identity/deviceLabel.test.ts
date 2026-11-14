// deviceLabel.test.ts 验证登录设备的显示名(设计 §6 DeviceLabel):浏览器信息串写成「系统 · 浏览器 主版本」,
// 读不出系统或浏览器时只写读得出的部分,信息串为空或完全读不出时写「未知设备」。
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { deviceLabel } from './deviceLabel.ts'

test('Windows 上的 Chrome 写成「Windows · Chrome 152」', () => {
  assert.equal(
    deviceLabel('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36'),
    'Windows · Chrome 152',
  )
})

test('macOS 上的 Safari 与 iPhone 上的 Safari', () => {
  assert.equal(
    deviceLabel('Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15'),
    'macOS · Safari 17',
  )
  assert.equal(
    deviceLabel('Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1'),
    'iOS · Safari 17',
  )
})

test('空串或完全读不出的信息串写「未知设备」', () => {
  assert.equal(deviceLabel(''), '未知设备')
  assert.equal(deviceLabel(undefined), '未知设备')
  assert.equal(deviceLabel('curl'), '未知设备')
})
