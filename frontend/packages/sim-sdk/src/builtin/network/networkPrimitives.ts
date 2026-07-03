// 本文件提供网络传播仿真的共享过程模型,统一消息标识和污染载荷标识生成。

import { deterministicId, fnv1aHex } from '../../runtime/deterministic';

/**
 * networkDigest 生成网络事件的稳定短摘要。
 */
export function networkDigest(domain: string, fields: Record<string, string | number | boolean>, length = 12): string {
  const encoded = Object.keys(fields)
    .sort()
    .map((key) => `${key}=${String(fields[key])}`)
    .join('|');
  const route = fnv1aHex(`${domain}:route:${encoded}`, length);
  return fnv1aHex(`${domain}:deliver:${route}:${encoded.length}`, length);
}

/**
 * networkMessageId 生成跨网络算法通用的消息 ID。
 */
export function networkMessageId(prefix: string, fields: Record<string, string | number | boolean>): string {
  return deterministicId(prefix, { ...fields, digest: networkDigest(prefix, fields) });
}

/**
 * pollutedMessageId 生成污染消息的稳定标识。
 */
export function pollutedMessageId(round: number): string {
  return `polluted-${networkDigest('polluted-gossip', { round }, 4)}`;
}
