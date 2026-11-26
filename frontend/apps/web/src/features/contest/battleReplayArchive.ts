// 解析下载的对局归档；未知版本或损坏字段直接拒绝，不猜测缺失数据。

import type {
  BattleReplayAction,
  BattleReplayArchive,
  BattleReplayResultDetail,
  SnowflakeID,
} from '@chaimir/api-client'
import { BattleRole, BattleRule, VULN_CHAIN_OPERATION } from '@chaimir/api-client'

/** 当前认识的归档协议版本,与后端 battleReplayArchiveVersion 一致。 */
const SUPPORTED_ARCHIVE_VERSION = 1

const CHAIN_OPERATIONS = Object.values(VULN_CHAIN_OPERATION)

/**
 * parseBattleReplayArchive 把归档正文解析为协议对象。
 * 任何不合协议的输入都抛出用户向错误,由调用方转成页面提示。
 */
export function parseBattleReplayArchive(text: string): BattleReplayArchive {
  let raw: unknown
  try {
    raw = JSON.parse(text)
  } catch {
    throw new Error('轨迹归档的内容读不出来。')
  }
  if (!isObject(raw)) throw new Error('轨迹归档的内容读不出来。')

  if (raw.version !== SUPPORTED_ARCHIVE_VERSION) {
    throw new Error('这一局的轨迹归档版本比当前页面新,请刷新后再试。')
  }

  const initial = raw.initial_state
  if (!isObject(initial)) throw new Error('轨迹归档缺少对局初始状态。')
  const entryA = readEntryState(initial.entry_a)
  const entryB = readEntryState(initial.entry_b)
  if (!entryA || !entryB) throw new Error('轨迹归档缺少双方参战物信息。')

  const actions = readActions(raw.actions)
  if (actions.length === 0) throw new Error('这一局的轨迹里没有链上动作记录。')

  const result = raw.result
  if (!isObject(result)) throw new Error('轨迹归档缺少判定结果。')
  if (typeof result.passed !== 'boolean') throw new Error('轨迹归档里的判定结果格式不正确。')

  const battleRule = readBattleRule(initial.battle_rule)
  const details = readDetails(result.details)
  const hasOutcome = details.some((detail) => detail.outcome !== undefined)
  if (battleRule === BattleRule.GAME) {
    if (!result.passed || !hasOutcome || details.some((detail) => !detail.passed)) {
      throw new Error('轨迹归档缺少有效的对局结果。')
    }
  } else if (hasOutcome) {
    throw new Error('轨迹归档的对局结果与规则不一致。')
  }

  return {
    version: SUPPORTED_ARCHIVE_VERSION,
    match_id: readSnowflakeID(raw.match_id),
    task_id: readSnowflakeID(raw.task_id),
    source_ref: readString(raw.source_ref),
    initial_state: {
      contest_id: readSnowflakeID(initial.contest_id),
      problem_id: readSnowflakeID(initial.problem_id),
      battle_rule: battleRule,
      entry_a: entryA,
      entry_b: entryB,
    },
    actions,
    result: {
      passed: result.passed,
      score: readFiniteNumber(result.score),
      max_score: readFiniteNumber(result.max_score),
      details,
      result_ref: readOptionalString(result.result_ref),
    },
    finished_at: readString(raw.finished_at),
  }
}

function readActions(value: unknown): BattleReplayAction[] {
  if (!Array.isArray(value)) throw new Error('轨迹归档缺少链上动作记录。')
  const out = value.map((item) => {
    if (!isObject(item)) throw new Error('轨迹归档里的链上动作格式不正确。')
    const op = CHAIN_OPERATIONS.find((op) => op === item.op)
    if (!op) throw new Error('轨迹归档包含无法识别的链上动作。')
    const runtimeInstance = readString(item.runtime_instance).trim()
    if (runtimeInstance === '') throw new Error('轨迹归档缺少动作的目标链。')
    return {
      seq: readPositiveInteger(item.seq),
      runtime_instance: runtimeInstance,
      op,
      payload: readOptionalObject(item.payload),
      output: readOptionalObject(item.output),
    }
  })
  return out.sort((left, right) => left.seq - right.seq)
}

function readDetails(value: unknown): BattleReplayResultDetail[] {
  if (!Array.isArray(value)) throw new Error('轨迹归档缺少判定明细。')
  let hasOutcome = false
  return value.map((item) => {
    if (!isObject(item) || typeof item.passed !== 'boolean') {
      throw new Error('轨迹归档里的判定明细格式不正确。')
    }
    const outcome = item.outcome
    if (outcome !== undefined) {
      if (hasOutcome || !item.passed || (outcome !== 'a_win' && outcome !== 'b_win' && outcome !== 'draw')) {
        throw new Error('轨迹归档里的对局结果格式不正确。')
      }
      hasOutcome = true
    }
    return {
      case: readString(item.case),
      passed: item.passed,
      expected_label: readOptionalString(item.expected_label),
      actual: readOptionalString(item.actual),
      hint: readOptionalString(item.hint),
      outcome,
    }
  })
}

function readEntryState(value: unknown) {
  if (!isObject(value)) return undefined
  const role = value.role
  if (role !== BattleRole.STRATEGY && role !== BattleRole.DEFENSE && role !== BattleRole.ATTACK)
    return undefined
  return {
    role,
    version_no: readPositiveInteger(value.version_no),
    artifact_hash: readString(value.artifact_hash),
  }
}

function readBattleRule(value: unknown): BattleRule {
  if (value === BattleRule.ATTACK_DEFENSE || value === BattleRule.GAME) return value
  throw new Error('轨迹归档里的对局规则无法识别。')
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function readString(value: unknown): string {
  if (typeof value === 'string' && value.length > 0) return value
  throw new Error('轨迹归档缺少必要文本。')
}

function readOptionalString(value: unknown): string | undefined {
  if (value === undefined) return undefined
  return readString(value)
}

function readFiniteNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  throw new Error('轨迹归档里的数值格式不正确。')
}

function readPositiveInteger(value: unknown): number {
  const number = readFiniteNumber(value)
  if (Number.isInteger(number) && number > 0) return number
  throw new Error('轨迹归档里的序号格式不正确。')
}

function readOptionalObject(value: unknown): Record<string, unknown> | undefined {
  if (value === undefined) return undefined
  if (isObject(value)) return value
  throw new Error('轨迹归档里的动作数据格式不正确。')
}

/** readSnowflakeID 严格读取公开雪花 ID,拒绝数字以避免浏览器精度损失。 */
function readSnowflakeID(value: unknown): SnowflakeID {
  if (typeof value === 'string' && /^[1-9]\d*$/.test(value)) return value
  throw new Error('轨迹归档里的编号格式不正确。')
}
