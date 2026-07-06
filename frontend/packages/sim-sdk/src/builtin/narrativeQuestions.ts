// 本文件集中维护内置仿真包检查点对应的教学判断题,避免各 trace 文件重复构造叙事问题。

import type { QuestionCheckpoint } from '../types';

export interface PhaseNarrativeQuestion extends Omit<QuestionCheckpoint, 'checkpointId'> {
  phaseId?: string;
}

export const narrativeQuestions: Record<string, PhaseNarrativeQuestion> = {
  'access-control-safe': {
    prompt: '敏感函数是否已经同时具备角色校验和审计记录?',
    options: ['已经受控', '仍可越权'],
    answer: '已经受控',
  },
  'block-validation-accepted': {
    prompt: '区块头、交易根、收据根和状态根是否已经全部通过本地验证?',
    options: ['全部通过', '仍有不一致'],
    answer: '全部通过',
  },
  'blockchain-link-valid': {
    prompt: '当前规范链是否已经按父哈希链接并完成分叉重组?',
    options: ['链接有效', '仍有分叉风险'],
    answer: '链接有效',
  },
  'bridge-proof-valid': {
    prompt: '跨链桥是否已经在同步轻客户端后验证锁仓证明?',
    options: ['证明可信', '证明仍不可信'],
    answer: '证明可信',
  },
  'call-stack-safe': {
    prompt: 'EVM 调用栈是否已经处理 revert 冒泡并恢复到安全状态?',
    options: ['已经收敛', '仍有未处理失败'],
    answer: '已经收敛',
  },
  'committee-authorized': {
    prompt: '多签委员会是否只聚合了达到门限的有效签名?',
    options: ['授权有效', '签名仍不可信'],
    answer: '授权有效',
  },
  'eip1559-fee-split': {
    prompt: '当前区块费用是否已经完成 base fee 销毁、小费支付和下一块 base fee 计算?',
    options: ['已经完成', '仍未完成'],
    answer: '已经完成',
    phaseId: 'adjust',
  },
  'eth-pos-finalized': {
    prompt: '当前 PoS 流程是否已经区分链头选择、justified checkpoint 和 finalized checkpoint?',
    options: ['已经区分', '仍然混淆'],
    answer: '已经区分',
    phaseId: 'finalize',
  },
  'cross-message-executed': {
    prompt: '跨链消息是否已经完成锁定、中继、证明验证和目标链执行?',
    options: ['已经执行', '仍未闭环'],
    answer: '已经执行',
  },
  'dht-lookup-found': {
    prompt: '当前 DHT 查找是否已避开污染路由并找到目标?',
    options: ['已经找到', '还没有'],
    answer: '已经找到',
  },
  'finality-release-safe': {
    prompt: '资产释放前是否已经等待足够确认并排除源链重组风险?',
    options: ['可以释放', '仍需等待'],
    answer: '可以释放',
  },
  'flash-loan-contained': {
    prompt: '闪电贷组合攻击是否已经被限额和价格保护阻断?',
    options: ['已经受控', '仍可获利'],
    answer: '已经受控',
  },
  'gossip-coverage': {
    prompt: '当前 Gossip 是否已覆盖大多数节点且隔离污染消息?',
    options: ['已经收敛', '还没有'],
    answer: '已经收敛',
  },
  'hash-chain-valid': {
    prompt: '当前哈希链是否已经恢复到可验证状态?',
    options: ['已经恢复', '还没有'],
    answer: '已经恢复',
  },
  'hotstuff-three-chain': {
    prompt: 'HotStuff 三链提交是否依赖连续 QC?',
    options: ['依赖', '不依赖'],
    answer: '依赖',
    phaseId: 'chain-commit',
  },
  'gas-execution-settled': {
    prompt: 'Gas 是否已经完成扣费、失败回滚、退款和最终结算?',
    options: ['已经结算', '仍未结清'],
    answer: '已经结算',
  },
  'integer-boundary-safe': {
    prompt: '整数输入是否已经经过范围限制、checked 运算和边界用例覆盖?',
    options: ['边界受控', '仍会溢出'],
    answer: '边界受控',
  },
  'latency-loss-delivered': {
    prompt: '当前丢失的数据包是否已经可靠重传并完成窗口恢复?',
    options: ['已经完成', '还没有'],
    answer: '已经完成',
  },
  'merkle-tree-root-valid': {
    prompt: 'Merkle Tree 修改叶子后是否已经沿影响路径重建可信根?',
    options: ['根有效', '根不一致'],
    answer: '根有效',
  },
  'merkle-proof-valid': {
    prompt: '当前 Merkle 证明是否能被可信根接受?',
    options: ['可以接受', '不能接受'],
    answer: '可以接受',
  },
  'mempool-replacement-valid': {
    prompt: '同 nonce 替换交易是否满足加价阈值,并且后续 queued 交易只在前序 nonce 入块后释放?',
    options: ['规则满足', '仍有违例'],
    answer: '规则满足',
    phaseId: 'release',
  },
  'nonce-order-valid': {
    prompt: '账户交易是否已经按连续 nonce 顺序打包并修复缺口?',
    options: ['顺序有效', '仍有缺口'],
    answer: '顺序有效',
  },
  'oracle-price-safe': {
    prompt: '预言机价格是否已经通过 TWAP 和多源聚合抵消现货操纵?',
    options: ['价格受控', '仍被操纵'],
    answer: '价格受控',
  },
  'partition-merged': {
    prompt: '当前网络是否已恢复连通并完成状态合并?',
    options: ['已经完成', '还没有'],
    answer: '已经完成',
  },
  'p2p-discovery-healthy': {
    prompt: '当前节点发现拓扑是否已过滤异常地址并保持可用?',
    options: ['已经可用', '仍有风险'],
    answer: '已经可用',
  },
  'pbft-safety': {
    prompt: 'PBFT 在本轮提交后是否已经满足安全执行条件?',
    options: ['满足', '不满足'],
    answer: '满足',
  },
  'pos-two-thirds-finality': {
    prompt: 'PoS 检查点最终确定是否需要足够权益见证?',
    options: ['需要', '不需要'],
    answer: '需要',
    phaseId: 'finalize',
  },
  'pow-fork-choice': {
    prompt: '当前规范链是否由最高累计工作量决定?',
    options: ['是', '否'],
    answer: '是',
  },
  'raft-majority-commit': {
    prompt: 'Raft 日志提交是否必须由多数派复制支撑?',
    options: ['必须', '不需要'],
    answer: '必须',
    phaseId: 'commit',
  },
  'reentrancy-blocked': {
    prompt: '提款流程是否已经用重入锁阻断 fallback 再次进入?',
    options: ['重入已阻断', '仍可重入'],
    answer: '重入已阻断',
  },
  'replay-protected': {
    prompt: '跨链消息是否已经通过 domain、nonce 和已执行集合拒绝重放?',
    options: ['重放已拒绝', '仍可重放'],
    answer: '重放已拒绝',
  },
  'optimistic-rollup-verdict': {
    prompt: '欺诈证明是否已经通过二分定位和 L1 单步验证得到明确裁决?',
    options: ['已有裁决', '仍在争议中'],
    answer: '已有裁决',
    phaseId: 'verdict',
  },
  'snapshot-root-valid': {
    prompt: '异常写入后是否已经回滚快照并恢复到可信状态根?',
    options: ['快照一致', '仍有脏状态'],
    answer: '快照一致',
  },
  'signature-valid': {
    prompt: '当前签名是否同时满足来源可信和 nonce 新鲜?',
    options: ['满足', '不满足'],
    answer: '满足',
  },
  'threshold-signature-valid': {
    prompt: '当前有效份额是否足以形成可验证的聚合签名?',
    options: ['足够', '不足'],
    answer: '足够',
  },
  'tendermint-commit': {
    prompt: '当前值是否已经获得超过三分之二 precommit,并受到 lock 约束保护?',
    options: ['可以提交', '不能提交'],
    answer: '可以提交',
    phaseId: 'commit',
  },
  'trie-root-valid': {
    prompt: 'Patricia Trie 是否已经重算路径哈希并给出有效根或缺失证明?',
    options: ['证明有效', '证明不一致'],
    answer: '证明有效',
  },
  'tx-lifecycle-receipt': {
    prompt: '交易是否已经完成签名、入池、打包、执行并生成回执?',
    options: ['回执已生成', '仍未完成'],
    answer: '回执已生成',
  },
  'utxo-set-valid': {
    prompt: 'UTXO 集合是否已经拒绝双花并写入新的未花费输出?',
    options: ['集合有效', '仍有双花'],
    answer: '集合有效',
  },
  'zk-proof-valid': {
    prompt: '当前响应是否满足承诺关系且没有泄露秘密?',
    options: ['满足', '不满足'],
    answer: '满足',
  },
  'zk-rollup-verifier': {
    prompt: 'L1 verifier 是否只在 proof 与 public inputs 绑定一致时更新 newRoot?',
    options: ['只在一致时更新', '不需要一致'],
    answer: '只在一致时更新',
    phaseId: 'verify',
  },
};
