// 本文件定义 P2P 节点发现仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { discoveryPhases } from './model';

export const discoverySource = [
  'function discoverPeers(localNetwork, minVersion) {',
  '  seeds = connectBootnodes();',
  '  addrs = requestAddr(seeds);',
  '  candidates = scoreAndDeduplicate(addrs);',
  '  for peer in candidates:',
  '    require(handshake(peer.network, peer.version));',
  '  probeHealthyPeers();',
  '  banMaliciousPeers();',
  '}',
];

/**
 * traceLinesForDiscovery 把节点发现内核迁移映射到伪代码行。
 */
export function traceLinesForDiscovery(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    bootstrap: [2],
    addr: [3, 4],
    handshake: [5, 6],
    probe: [7],
    ban: [8],
    poison: [3, 4],
  };
  return mapping[transition] ?? [1];
}

export const discoveryCodeTrace: CodeTraceDef = {
  sourceCode: discoverySource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == bootstrap', annotation: '新节点先连接可信引导节点。' },
    { line: 3, triggerCondition: 'lastTransition == addr || lastTransition == poison', annotation: '地址簿从已知节点返回,需要继续评分和去重。' },
    { line: 4, triggerCondition: 'lastTransition == addr || lastTransition == poison', annotation: '候选地址按来源、分数和重复项归一化。' },
    { line: 6, triggerCondition: 'lastTransition == handshake', annotation: '网络标识和协议版本不匹配会拒绝连接。' },
    { line: 7, triggerCondition: 'lastTransition == probe', annotation: '健康探测决定连接是否继续保留。' },
    { line: 8, triggerCondition: 'lastTransition == ban', annotation: '恶意或异常地址进入本地黑名单。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'handshakeCount', extract: 'state.handshakeCount', format: 'number' },
    { name: 'connected', extract: 'state.metrics.connected', format: 'number' },
    { name: 'banned', extract: 'state.metrics.banned', format: 'number' },
  ],
};

export const discoveryNarrative = phaseNarrative(discoveryPhases, 'p2p-discovery-healthy');
