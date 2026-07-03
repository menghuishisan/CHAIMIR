// 本文件提供交易运行时仿真的共享过程模型,统一交易哈希、区块验证项摘要和无效根构造。

import { fnv1aHex } from '../../runtime/deterministic';

/**
 * runtimeDigest 用阶段化压缩生成交易执行相关摘要。
 */
export function runtimeDigest(domain: string, fields: Record<string, string | number | boolean>, length = 16): string {
  const encoded = Object.keys(fields)
    .sort()
    .map((key) => `${key}=${String(fields[key])}`)
    .join('|');
  const absorb = fnv1aHex(`${domain}:absorb:${encoded}`, length);
  const execute = fnv1aHex(`${domain}:execute:${absorb}:${encoded.length}`, length);
  return fnv1aHex(`${domain}:commit:${execute}`, length);
}

/**
 * transactionIntentHash 绑定发送方、接收方、金额和 nonce 生成交易意图摘要。
 */
export function transactionIntentHash(from: string, to: string, amount: number, nonce: number): string {
  return runtimeDigest('transaction-intent', { amount, from, nonce, to });
}

/**
 * signedTransactionHash 绑定交易意图和签名者生成已签名交易哈希。
 */
export function signedTransactionHash(intentHash: string, signer: string): string {
  return runtimeDigest('signed-transaction', { intentHash, signer });
}

/**
 * blockValidationDigest 生成区块验证项的期望摘要。
 */
export function blockValidationDigest(label: string, blockNumber: number): string {
  return runtimeDigest('block-validation-item', { blockNumber, label }, 12);
}

/**
 * invalidValidationDigest 生成用于异常路径展示的错误摘要。
 */
export function invalidValidationDigest(label: string): string {
  return runtimeDigest('invalid-validation-item', { label }, 12);
}

/**
 * blockHeaderDigest 聚合验证项摘要生成待验证区块头哈希。
 */
export function blockHeaderDigest(items: Array<{ label: string; expected: string }>): string {
  const folded = items.map((item) => `${item.label}:${item.expected}`).sort().join('|');
  return runtimeDigest('validated-block-header', { folded });
}
