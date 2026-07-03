// 本文件定义授权缺陷仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { SecurityActor, SecurityCall } from '../securityView';

export interface AccessState extends SimState {
  phaseIndex: number;
  roles: Record<string, string>;
  protectedFunction: boolean;
  unauthorizedExecuted: boolean;
  unauthorizedBlocked: boolean;
  auditLogged: boolean;
  actors: SecurityActor[];
  calls: SecurityCall[];
  lastTransition: string;
}

export const accessPhases = [
  { id: 'roles', label: '声明角色', detail: '管理员与普通用户', effect: '合约为敏感操作声明调用角色。', reason: '授权边界必须先有清晰角色模型。' },
  { id: 'check', label: '执行鉴权检查', detail: 'require role', effect: '敏感函数读取服务端或链上可信角色。', reason: '不能把客户端传参当成权限来源。' },
  { id: 'exploit', label: '越权调用', detail: '未校验函数被调用', effect: '普通用户调用管理员函数并修改关键状态。', reason: '缺少鉴权会让任何地址执行敏感操作。' },
  { id: 'audit', label: '记录敏感操作', detail: '写入审计事件', effect: '越权尝试和敏感操作进入审计轨迹。', reason: '审计让事后追踪和风控阻断成为可能。' },
  { id: 'least', label: '最小权限修复', detail: '只授权必要角色', effect: '函数加上角色校验并只授予必要账户。', reason: '最小权限降低密钥泄露或误授带来的影响。' },
];
