// ===== M3 Judge 模块 =====

import type {
  JudgeTaskStatus,
  JudgerSelftestStatus,
  JudgerStatus,
  JudgerType,
} from '../constants/judge'
import type { SnowflakeID } from './common'
import type { SandboxCompositionSnapshot, SandboxCompositionSpec } from './composition'

export interface JudgeTask {
  task_id: SnowflakeID
  tenant_id: SnowflakeID
  source_ref: string
  submitter_id: SnowflakeID
  status: JudgeTaskStatus
  existing?: boolean
  result?: JudgeTaskResult
}

export interface JudgeTaskResult {
  passed: boolean
  score: number
  max_score: number
  version?: number
  is_rejudge?: boolean
  details: JudgeResultDetail[]
  result_ref: string
}

export interface JudgeResultDetail {
  case?: string
  source?: string
  target?: string
  passed: boolean
  expected_label?: string
  actual?: string
  hint?: string
}

export interface JudgeManualScoreRequest {
  score: number
  max_score: number
  passed: boolean
  comment: string
}

/**
 * JudgerRequest 是平台管理员创建或更新判题器的请求。
 * 需要沙箱的判题器必须声明 judge-private 组合;镜像地址、digest 与工作负载由服务端编译,
 * 不得把 GET 返回的 resource_spec.composition_snapshot 重新拼装后当输入(§8.3)。
 */
export interface JudgerRequest {
  code: string
  name: string
  type: JudgerType
  executor_ref: string
  runtime_required: boolean
  default_timeout_sec: number
  /** composition 只在写入时提交,服务端编译后不原样持久化 */
  composition: SandboxCompositionSpec
  resource_spec: JudgerExecutionSpec
  status: JudgerStatus
}

/** JudgerExecutionSpec 是可提交的受控执行策略,不含任何组合快照或镜像字段。 */
export interface JudgerExecutionSpec {
  genesis_ref?: string
  init_script_ref?: string
  command?: string[]
  exec_target?: string
  execution_sidecars?: unknown[]
  timeout_sec?: number
  max_retries?: number
  suite_archive_name?: string
  selftest?: Record<string, unknown>
}

/**
 * JudgerResourceSpec 是数据库里唯一持久化的判题执行事实,只读。
 * 它比可提交的执行策略多一个服务端编译冻结的组合快照 —— 详情页把它当事实展示,不回填为编辑输入。
 */
export interface JudgerResourceSpec extends JudgerExecutionSpec {
  composition_snapshot?: SandboxCompositionSnapshot
}

/** Judger 是判题器的读取投影:声明字段 + 只读执行事实 + 自检状态。 */
export interface Judger extends Omit<JudgerRequest, 'composition' | 'resource_spec'> {
  id: SnowflakeID
  resource_spec: JudgerResourceSpec
  selftest_status: JudgerSelftestStatus
  created_at?: string
  updated_at?: string
}

/**
 * JudgerCatalog 是判题方式目录响应:配置检查点只需编码、名称与类型。
 * 它刻意不是 Judger 的子集别名 —— resource_spec 里的判题镜像、受控命令与执行组件
 * 属判题私密面,后端也不下发,故编排面按独立类型对接。
 */
export interface JudgerCatalog {
  judgers: CatalogJudger[]
}

export interface CatalogJudger {
  code: string
  name: string
  type: JudgerType
}
