// contest options 文件维护竞赛表单使用的封闭选项顺序。

import {
  BattleRule,
  CHAIN_ASSERT_OPERATION,
  CheatAction,
  CheatType,
  VULN_CHAIN_OPERATION,
  VulnLevel,
  VulnRuntimeMode,
  VulnSourceType,
  type ChainAssertOperation,
  type VulnChainOperation,
} from '@chaimir/api-client'

/** BATTLE_RULES 供赛题表单按登记顺序渲染对抗规则选项。 */
export const BATTLE_RULES = [BattleRule.ATTACK_DEFENSE, BattleRule.GAME] as const

/** CHEAT_TYPES 供违规处理表单按登记顺序渲染类型选项。 */
export const CHEAT_TYPES = [
  CheatType.SIMILARITY,
  CheatType.BEHAVIOR,
  CheatType.ENVIRONMENT,
] as const

/** CHEAT_ACTIONS 供违规处理表单按登记顺序渲染处理方式选项。 */
export const CHEAT_ACTIONS = [
  CheatAction.WARN,
  CheatAction.PENALTY,
  CheatAction.DISQUALIFY,
] as const

/** VULN_SOURCE_TYPES 供漏洞源表单按登记顺序渲染类型选项。 */
export const VULN_SOURCE_TYPES = [
  VulnSourceType.SWC,
  VulnSourceType.INTELLIGENCE,
  VulnSourceType.CVE_ONCHAIN,
] as const satisfies readonly VulnSourceType[]

/** VULN_LEVELS 供漏洞题表单按登记顺序渲染分级选项。 */
export const VULN_LEVELS = [VulnLevel.A, VulnLevel.B, VulnLevel.C] as const

/** VULN_RUNTIME_MODES 供漏洞题表单按登记顺序渲染运行方式选项。 */
export const VULN_RUNTIME_MODES = [VulnRuntimeMode.ISOLATED, VulnRuntimeMode.FORKED] as const

/** VULN_CHAIN_OPS 供步骤表单按后端支持顺序渲染操作选项。 */
export const VULN_CHAIN_OPS = [
  VULN_CHAIN_OPERATION.DEPLOY,
  VULN_CHAIN_OPERATION.TX,
  VULN_CHAIN_OPERATION.RESET,
  VULN_CHAIN_OPERATION.QUERY,
] as const

/** VulnChainOp 是漏洞预验证步骤允许的操作类型。 */
export type VulnChainOp = VulnChainOperation

/** VULN_ASSERT_OPS 供断言表单按后端支持顺序渲染判定方式。 */
export const VULN_ASSERT_OPS = [
  CHAIN_ASSERT_OPERATION.EQ,
  CHAIN_ASSERT_OPERATION.NE,
  CHAIN_ASSERT_OPERATION.CONTAINS,
  CHAIN_ASSERT_OPERATION.EXISTS,
] as const

/** VulnAssertOp 是漏洞预验证断言允许的操作类型。 */
export type VulnAssertOp = ChainAssertOperation

/** 漏洞源请求方法:后端只允许 GET 与 POST。 */
export const VULN_SOURCE_METHODS = ['GET', 'POST'] as const
