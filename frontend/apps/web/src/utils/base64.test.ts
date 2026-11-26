import assert from 'node:assert/strict'
import { test } from 'node:test'
import { decodeUtf8Base64, encodeUtf8Base64 } from './base64.ts'

test('合法文本、空文件与 UTF-8 BOM 往返保持原内容', () => {
  for (const text of ['', '// 共识\nconst 名称 = "链"', '\ufeffpackage main\r\n', '\u{10437}\0']) {
    assert.equal(decodeUtf8Base64(encodeUtf8Base64(text)), text)
  }
})

test('损坏的 Base64 不得作为成功读取的空文件', () => {
  for (const value of ['not*base64', 'A']) {
    assert.throws(() => decodeUtf8Base64(value))
  }
})

test('非法 UTF-8 不得被替换后作为可编辑文本', () => {
  for (const bytes of [[0xff], [0xe4, 0xb8], [0xc0, 0xaf], [0x80]]) {
    assert.throws(() => decodeUtf8Base64(btoa(String.fromCharCode(...bytes))))
  }
})
