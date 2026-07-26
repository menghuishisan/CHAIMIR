// env.mjs 是本地联调脚本的唯一环境变量入口。
// 它按后端启动脚本完全一致的顺序加载同一批环境文件，
// 保证短信 mock、探活脚本与后端进程读到的是同一份配置，杜绝各自定义默认值造成的漂移。

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// 仓库根目录由脚本自身位置推导，避免调用方传入的工作目录影响解析结果。
export const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')

// 环境文件加载顺序与 backend 启动脚本一致：后加载的文件覆盖先加载的同名键，
// 因此密钥类配置以 deploy/config/secret.env 为准。
const ENV_FILES = [
  'backend/.env.example',
  'backend/.env',
  'deploy/config/secret.env',
]

/**
 * parseEnvFile 解析单个环境文件，规则与 PowerShell 侧加载器保持一致：
 * 跳过空行与注释行，按首个等号拆分，键与值两端去空白。
 */
function parseEnvFile(absolutePath) {
  const entries = new Map()
  const content = fs.readFileSync(absolutePath, 'utf8').replace(/^﻿/, '')
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (line === '' || line.startsWith('#')) {
      continue
    }
    const separator = line.indexOf('=')
    if (separator <= 0) {
      continue
    }
    entries.set(line.slice(0, separator).trim(), line.slice(separator + 1).trim())
  }
  return entries
}

/**
 * loadE2EEnv 返回联调环境的权威配置快照。
 * 文件缺失时立即抛错而不是静默跳过，避免带着空值继续启动后在链路中段才暴露问题。
 */
export function loadE2EEnv() {
  const merged = new Map()
  for (const relativePath of ENV_FILES) {
    const absolutePath = path.join(repoRoot, relativePath)
    if (!fs.existsSync(absolutePath)) {
      throw new Error(`联调环境文件缺失: ${absolutePath}（请先按 ${relativePath}.example 准备该文件）`)
    }
    for (const [key, value] of parseEnvFile(absolutePath)) {
      merged.set(key, value)
    }
  }
  return merged
}

/**
 * requireEnv 读取必填配置项，缺失或为空时给出可直接定位到文件的错误信息。
 */
export function requireEnv(env, key) {
  const value = (env.get(key) ?? '').trim()
  if (value === '') {
    throw new Error(`环境变量 ${key} 未配置或为空，请在 ${path.join(repoRoot, 'deploy/config/secret.env')} 中设置后重试`)
  }
  return value
}
