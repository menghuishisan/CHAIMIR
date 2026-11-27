import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseHttpUrl } from './validators.ts'

test('HTTP地址使用原生解析与序列化，保留路径和查询语义', () => {
  assert.equal(parseHttpUrl(' HTTPS://EXAMPLE.ORG:443/a?limit=2 ')?.href, 'https://example.org/a?limit=2')
  assert.equal(parseHttpUrl('http://example.org:8080/a')?.href, 'http://example.org:8080/a')
  assert.equal(parseHttpUrl('https://example.org/学习')?.href, 'https://example.org/%E5%AD%A6%E4%B9%A0')
})

test('残缺地址、非法主机端口、其它协议和嵌入凭据均不可提交', () => {
  for (const value of ['', 'http://', 'https://', 'https://exa mple.org', 'https://[invalid',
    'https://example.org:99999', 'ftp://example.org/a', 'https://user:placeholder@example.org/a']) {
    assert.equal(parseHttpUrl(value), undefined)
  }
})
