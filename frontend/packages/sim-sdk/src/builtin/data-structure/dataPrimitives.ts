// 本文件提供链上数据结构仿真的共享过程模型,统一区块头、Merkle、Trie 和状态根摘要。

import { fnv1aHex } from '../../runtime/deterministic';

/**
 * canonicalDataEncode 按字段名排序编码结构化数据,避免摘要受对象字段顺序影响。
 */
export function canonicalDataEncode(fields: Record<string, string | number | boolean>): string {
  return Object.keys(fields)
    .sort()
    .map((key) => `${key}=${String(fields[key])}`)
    .join('|');
}

/**
 * dataDigest 用吸收、混合、截断三步模拟链上结构摘要计算。
 */
export function dataDigest(domain: string, fields: Record<string, string | number | boolean>, length = 12): string {
  const encoded = canonicalDataEncode(fields);
  const absorb = fnv1aHex(`${domain}:absorb:${encoded}`, length);
  const mix = fnv1aHex(`${domain}:mix:${absorb}:${encoded.length}`, length);
  return fnv1aHex(`${domain}:root:${mix}`, length);
}

/**
 * blockHeaderHash 绑定高度、父哈希和载荷生成区块头摘要。
 */
export function blockHeaderHash(height: number, parentHash: string, payload: string): string {
  return dataDigest('block-header', { height, parentHash, payload });
}

/**
 * merkleStructureLeafHash 绑定叶子位置和值生成 Merkle 叶子摘要。
 */
export function merkleStructureLeafHash(index: number, value: string): string {
  return dataDigest('merkle-structure-leaf', { index, value });
}

/**
 * merkleStructureParentHash 按左右方向合并两个子摘要。
 */
export function merkleStructureParentHash(left: string, right: string): string {
  return dataDigest('merkle-structure-parent', { left, right });
}

/**
 * trieLeafHash 绑定压缩路径和值生成 Patricia Trie 叶子摘要。
 */
export function trieLeafHash(key: string, path: string, value: string): string {
  return dataDigest('patricia-leaf', { key, path, value });
}

/**
 * trieRootHash 绑定有序叶子摘要和路径生成 Patricia Trie 根摘要。
 */
export function trieRootHash(entries: Array<{ path: string; hash: string }>): string {
  const folded = entries
    .map((entry) => `${entry.path}:${entry.hash}`)
    .sort()
    .join('|');
  return dataDigest('patricia-root', { folded });
}

/**
 * accountLeafHash 绑定账户、余额和 nonce 生成状态叶子摘要。
 */
export function accountLeafHash(accountId: string, balance: number, nonce: number): string {
  return dataDigest('account-state-leaf', { accountId, balance, nonce });
}

/**
 * stateRootHash 聚合账户叶子摘要生成状态根。
 */
export function stateRootHash(accounts: Array<{ id: string; balance: number; nonce: number }>): string {
  const folded = accounts
    .map((account) => accountLeafHash(account.id, account.balance, account.nonce))
    .sort()
    .join('|');
  return dataDigest('state-root', { folded });
}
