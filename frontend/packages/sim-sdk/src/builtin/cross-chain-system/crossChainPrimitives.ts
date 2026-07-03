// 本文件提供跨链系统仿真的共享过程模型,统一消息域、桥证明、轻客户端和重放防护摘要。

import { fnv1aHex } from '../../runtime/deterministic';

/**
 * crossChainDigest 用域分离、吸收和确认三步生成跨链教学摘要。
 */
export function crossChainDigest(domain: string, fields: Record<string, string | number | boolean>, length = 16): string {
  const encoded = Object.keys(fields)
    .sort()
    .map((key) => `${key}=${String(fields[key])}`)
    .join('|');
  const absorb = fnv1aHex(`${domain}:absorb:${encoded}`, length);
  const attest = fnv1aHex(`${domain}:attest:${absorb}:${encoded.length}`, length);
  return fnv1aHex(`${domain}:final:${attest}`, length);
}

/**
 * bridgeProofHash 绑定源链、目标链、锁仓事件和高度生成桥证明摘要。
 */
export function bridgeProofHash(sourceChain: string, targetChain: string, lockEvent: string, height: number): string {
  return crossChainDigest('bridge-proof', { height, lockEvent, sourceChain, targetChain });
}

/**
 * invalidBridgeProofHash 生成可被轻客户端拒绝的错误证明摘要。
 */
export function invalidBridgeProofHash(proofHash: string): string {
  return crossChainDigest('bridge-proof-invalid', { proofHash });
}

/**
 * crossChainMessageHash 绑定跨链消息的域、nonce 和载荷。
 */
export function crossChainMessageHash(domain: string, nonce: number, payload: string): string {
  return crossChainDigest('cross-chain-message', { domain, nonce, payload });
}

/**
 * replayProtectionHash 生成带目标域和 nonce 的重放防护摘要。
 */
export function replayProtectionHash(domain: string, nonce: number): string {
  return crossChainDigest('replay-protection', { domain, nonce, payload: 'transfer' });
}

/**
 * committeeMemberSignature 生成绑定成员和消息摘要的跨链委员会签名。
 */
export function committeeMemberSignature(memberId: string, messageHash: string): string {
  return crossChainDigest('committee-member-signature', { memberId, messageHash }, 12);
}

/**
 * aggregateCommitteeSignature 聚合有效成员签名形成委员会授权摘要。
 */
export function aggregateCommitteeSignature(messageHash: string, signatures: string[]): string {
  return crossChainDigest('committee-aggregate-signature', { messageHash, signatures: signatures.slice().sort().join('|') });
}
