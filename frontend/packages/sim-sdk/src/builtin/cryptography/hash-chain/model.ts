// 本文件定义哈希链篡改扩散仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';

export interface HashRecord {
  id: string;
  index: number;
  payload: string;
  hash: string;
  parentHash: string;
  tampered: boolean;
  valid: boolean;
}

export interface HashChainState extends SimState {
  phaseIndex: number;
  records: HashRecord[];
  selectedRecordId?: string;
  repaired: boolean;
  lastTransition: string;
}

export const hashChainPhases = [
  { id: 'normalize', label: '规范化输入', detail: '固定字段顺序', effect: '把交易字段按稳定顺序序列化,避免同义输入产生不同摘要。', reason: '哈希前必须先确定唯一字节序列。' },
  { id: 'hash', label: '计算摘要', detail: '压缩为固定长度', effect: '每条记录计算不可逆摘要。', reason: '摘要让任意微小改动都能被检测到。' },
  { id: 'link', label: '串联父哈希', detail: '写入前序摘要', effect: '后续记录把前一条摘要作为父哈希。', reason: '链式引用把局部篡改传播到后续所有摘要。' },
  { id: 'verify', label: '逐项校验', detail: '重算并比较', effect: '验证器从第一条开始重算哈希和父哈希。', reason: '不需要信任存储结果,只要重算即可发现篡改。' },
  { id: 'repair', label: '重算修复', detail: '恢复规范链', effect: '修复被篡改记录并重新计算后续摘要。', reason: '修复必须沿依赖链传播,不能只改一个字段。' },
] as const;
