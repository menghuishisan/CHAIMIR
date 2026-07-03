// 本文件提供共识仿真共享的教学级判定模型,统一摘要、法定人数、投票证书和权益阈值计算。

import { fnv1aHex } from '../../runtime/deterministic';

export interface VoteCertificate {
  subject: string;
  signers: string[];
  threshold: number;
  achieved: boolean;
  proofDigest: string;
}

/**
 * canonicalConsensusDigest 用域分离和字段排序生成稳定协议摘要。
 */
export function canonicalConsensusDigest(domain: string, fields: Record<string, string | number | boolean>, length = 16): string {
  const encoded = Object.keys(fields)
    .sort()
    .map((key) => `${key}=${String(fields[key])}`)
    .join('|');
  const absorb = fnv1aHex(`${domain}:absorb:${encoded}`, length);
  const mix = fnv1aHex(`${domain}:mix:${absorb}:${encoded.length}`, length);
  return fnv1aHex(`${domain}:final:${mix}`, length);
}

/**
 * bftQuorumThreshold 计算 BFT 安全阈值;当 n=3f+1 时等价于 2f+1,更大副本集使用 n-f 保证任意两个法定集合至少交叠 f+1。
 */
export function bftQuorumThreshold(replicaCount: number): number {
  const faultTolerance = Math.floor((replicaCount - 1) / 3);
  return replicaCount - faultTolerance;
}

/**
 * majorityThreshold 计算崩溃容错共识的多数派阈值。
 */
export function majorityThreshold(nodeCount: number): number {
  return Math.floor(nodeCount / 2) + 1;
}

/**
 * weightedTwoThirdsThreshold 计算权益最终性需要的三分之二以上阈值。
 */
export function weightedTwoThirdsThreshold(totalWeight: number): number {
  return Math.floor((totalWeight * 2) / 3) + 1;
}

/**
 * makeVoteCertificate 生成绑定主题、签名者集合和阈值的投票证书。
 */
export function makeVoteCertificate(domain: string, subject: string, signers: string[], threshold: number): VoteCertificate {
  const uniqueSigners = Array.from(new Set(signers)).sort();
  const proofDigest = canonicalConsensusDigest(domain, { signers: uniqueSigners.join(','), subject, threshold }, 16);
  return { subject, signers: uniqueSigners, threshold, achieved: uniqueSigners.length >= threshold, proofDigest };
}

/**
 * aggregateConsensusSignatures 聚合已排序的教学签名,用于展示群体见证结果。
 */
export function aggregateConsensusSignatures(domain: string, signatures: string[]): string {
  return canonicalConsensusDigest(domain, { signatures: signatures.slice().sort().join('|') }, 16);
}
