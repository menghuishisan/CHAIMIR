// 路由动作定义：集中维护页面动作、行操作、字段构造和提交值转换。

import type { ChaimirApi } from '@chaimir/api-client'
import type { ActionField, ActionValues, DataRow, PageAction, RowAction } from '../types'

export function teacherCourseActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-course-detail', '保存课程', '创建课程基础信息，章节、作业和成员在子页继续维护。', [
      textInput('name', '课程名称', true),
      textareaInput('description', '课程说明', true),
      numberInput('type', '课程类型', true),
      numberInput('difficulty', '难度', true),
      textInput('semester', '学期', true),
      numberInput('credits', '学分', true),
      textareaInput('schedule', '课程安排', true),
      datetimeInput('start_at', '开始时间', true),
      datetimeInput('end_at', '结束时间', true),
    ], async (values) => {
      await api.teaching.createCourse({
        name: valueText(values, 'name'),
        description: valueText(values, 'description'),
        type: valueNumber(values, 'type'),
        difficulty: valueNumber(values, 'difficulty'),
        semester: valueText(values, 'semester'),
        credits: valueNumber(values, 'credits'),
        schedule: valueJson(values, 'schedule'),
        start_at: valueText(values, 'start_at'),
        end_at: valueText(values, 'end_at'),
      })
      return '课程已保存'
    }),
    pageAction('refresh-invite-code', '刷新邀请码', '刷新课程邀请码，旧邀请码立即失效。', [textInput('course_id', '课程编号', true)], async (values) => {
      await api.teaching.refreshInviteCode(valueText(values, 'course_id'))
      return '课程邀请码已刷新'
    }),
  ]
}

export function assignmentActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-assignment', '创建作业', '创建课程作业并保存题目列表。', [
      textInput('course_id', '课程编号', true),
      textInput('title', '作业标题', true),
      textInput('chapter_id', '章节编号', true),
      datetimeInput('due_at', '截止时间', true),
      numberInput('max_attempts', '提交次数', true),
      numberInput('late_policy', '迟交策略', true),
      textareaInput('late_penalty', '迟交扣分规则', true),
      textareaInput('items', '题目列表', true),
    ], async (values) => {
      await api.teaching.createAssignment(valueText(values, 'course_id'), {
        title: valueText(values, 'title'),
        chapter_id: valueText(values, 'chapter_id'),
        due_at: valueText(values, 'due_at'),
        max_attempts: valueNumber(values, 'max_attempts'),
        late_policy: valueNumber(values, 'late_policy'),
        late_penalty: valueJson(values, 'late_penalty'),
        items: valueJsonArray(values, 'items') as never,
      })
      return '作业已创建'
    }),
    pageAction('publish-assignment', '发布作业', '发布作业后学生可以作答。', [textInput('assignment_id', '作业编号', true)], async (values) => {
      await api.teaching.publishAssignment(valueText(values, 'assignment_id'))
      return '作业已发布'
    }),
  ]
}

export function experimentAuthorActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('save-experiment', '保存实验草稿', '保存实验编排草稿，步骤状态由服务端持久化。', [
      textInput('course_id', '课程编号', true),
      textInput('template_ref', '模板引用', true),
      textInput('template_version', '模板版本', true),
      textInput('name', '实验名称', true),
      textareaInput('description', '实验说明', true),
      textareaInput('components', '组件配置', true),
      numberInput('collab_mode', '协作模式', true),
      textareaInput('group_config', '小组配置', true),
      numberInput('wizard_step', '当前步骤', true),
    ], async (values) => {
      await api.experiment.createExperiment({
        course_id: valueNumber(values, 'course_id'),
        template_ref: valueText(values, 'template_ref'),
        template_version: valueText(values, 'template_version'),
        name: valueText(values, 'name'),
        description: valueText(values, 'description'),
        components: valueJson(values, 'components') as never,
        collab_mode: valueNumber(values, 'collab_mode'),
        group_config: valueJson(values, 'group_config') as never,
        require_report: true,
        wizard_step: valueNumber(values, 'wizard_step'),
      })
      return '实验草稿已保存'
    }),
    pageAction('validate-experiment', '校验实验', '触发服务端编排校验。', [textInput('experiment_id', '实验编号', true)], async (values) => {
      await api.experiment.validateExperiment(valueText(values, 'experiment_id'))
      return '实验校验已完成'
    }),
  ]
}

export function vulnSourceActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('save-vuln-source', '保存漏洞源', '保存外部漏洞来源配置。', [
      numberInput('type', '来源类型', true),
      textInput('name', '来源名称', true),
      textareaInput('config', '来源配置', true),
      numberInput('default_level', '默认等级', true),
      numberInput('enabled', '是否启用', true),
    ], async (values) => {
      await api.contest.upsertVulnSource({
        type: valueNumber(values, 'type'),
        name: valueText(values, 'name'),
        config: valueJson(values, 'config'),
        default_level: valueNumber(values, 'default_level'),
        enabled: valueNumber(values, 'enabled') === 1,
      })
      return '漏洞源已保存'
    }),
  ]
}

export function vulnProblemActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('prevalidate-vuln', '预验证漏洞题', '触发正向和反向预验证。', [
      textInput('problem_id', '漏洞题编号', true),
      textInput('runtime_code', '运行时编码', true),
      textInput('runtime_image_version', '运行时镜像版本', true),
      textInput('tool_codes', '工具编码', true, '多个编码用英文逗号分隔。'),
      textInput('init_code_ref', '初始化代码引用'),
      textInput('init_script_ref', '初始化脚本引用'),
    ], async (values) => {
      await api.contest.prevalidateVulnProblem(valueText(values, 'problem_id'), {
        runtime_code: valueText(values, 'runtime_code'),
        runtime_image_version: valueText(values, 'runtime_image_version'),
        tool_codes: valueStringArray(values, 'tool_codes'),
        init_code_ref: optionalText(values, 'init_code_ref'),
        init_script_ref: optionalText(values, 'init_script_ref'),
      })
      return '漏洞题预验证已触发'
    }),
    pageAction('finalize-vuln', '转为正式题', '预验证通过后转入题库。', [textInput('problem_id', '漏洞题编号', true)], async (values) => {
      await api.contest.finalizeVulnProblem(valueText(values, 'problem_id'))
      return '漏洞题已转为正式题'
    }),
  ]
}

export function contentActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-content-detail', '保存内容', '保存题目或模板草稿。', [
      textInput('code', '内容编码', true),
      textInput('version', '版本', true),
      numberInput('type', '内容类型', true),
      textInput('title', '标题', true),
      numberInput('category_id', '分类编号', true),
      numberInput('difficulty', '难度', true),
      numberInput('visibility', '可见性', true),
      textareaInput('body', '内容正文', true),
    ], async (values) => {
      await api.content.createItem({
        code: valueText(values, 'code'),
        version: valueText(values, 'version'),
        type: valueNumber(values, 'type'),
        title: valueText(values, 'title'),
        category_id: valueNumber(values, 'category_id'),
        difficulty: valueNumber(values, 'difficulty'),
        tags: [],
        knowledge_points: [],
        visibility: valueNumber(values, 'visibility'),
        body: valueJson(values, 'body'),
        sensitive_fields: [],
      })
      return '内容已保存'
    }),
  ]
}

export function paperActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-paper-detail', '保存试卷', '保存试卷题目列表或组卷条件。', [
      textInput('name', '试卷名称', true),
      numberInput('gen_mode', '组卷方式', true),
      textareaInput('gen_criteria', '组卷条件'),
      textareaInput('items', '题目列表'),
    ], async (values) => {
      await api.content.createPaper({
        name: valueText(values, 'name'),
        gen_mode: valueNumber(values, 'gen_mode'),
        gen_criteria: valueJson(values, 'gen_criteria'),
        items: valueJsonArray(values, 'items') as never,
      })
      return '试卷已保存'
    }),
  ]
}

export function simPackageActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('submit-sim-detail', '上传仿真包', '上传仿真包并提交审核。', [
      fileInput('bundle', '仿真包文件', true),
      textInput('code', '仿真编码', true),
      textInput('version', '版本', true),
      textInput('name', '名称', true),
      textInput('category', '分类', true),
      textInput('compute', '计算方式', true, 'frontend 或 backend。'),
    ], async (values) => {
      await api.sim.submitPackage({
        bundle: valueFile(values, 'bundle'),
        code: valueText(values, 'code'),
        version: valueText(values, 'version'),
        name: valueText(values, 'name'),
        category: valueText(values, 'category'),
        compute: valueText(values, 'compute') === 'backend' ? 'backend' : 'frontend',
      })
      return '仿真包已提交审核'
    }),
  ]
}

export function appealReviewActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('accept-appeal-detail', '受理申诉', '受理成绩申诉并写入说明。', [
      textInput('appeal_id', '申诉编号', true),
      textareaInput('comment', '处理说明', true),
    ], async (values) => {
      await api.grade.acceptAppeal(valueText(values, 'appeal_id'), { comment: valueText(values, 'comment') })
      return '成绩申诉已受理'
    }),
    pageAction('reject-appeal-detail', '驳回申诉', '驳回成绩申诉并写入原因。', [
      textInput('appeal_id', '申诉编号', true),
      textareaInput('comment', '驳回原因', true),
    ], async (values) => {
      await api.grade.rejectAppeal(valueText(values, 'appeal_id'), { comment: valueText(values, 'comment') })
      return '成绩申诉已驳回'
    }),
  ]
}

export function gradeConfigActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-semester', '创建学期', '创建成绩归档学期。', [
      textInput('name', '学期名称', true),
      textInput('start_date', '开始日期', true),
      textInput('end_date', '结束日期', true),
      numberInput('is_current', '是否当前学期', true),
    ], async (values) => {
      await api.grade.createSemester({
        name: valueText(values, 'name'),
        start_date: valueText(values, 'start_date'),
        end_date: valueText(values, 'end_date'),
        is_current: valueNumber(values, 'is_current') === 1,
      })
      return '学期已创建'
    }),
    pageAction('update-warning-rules', '更新预警规则', '更新挂科数和最低绩点规则。', [
      numberInput('fail_count', '挂科门数', true),
      numberInput('min_gpa', '最低绩点', true),
    ], async (values) => {
      await api.grade.updateWarningRules({
        fail_count: valueNumber(values, 'fail_count'),
        min_gpa: valueNumber(values, 'min_gpa'),
      })
      return '预警规则已更新'
    }),
  ]
}

export function alertActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('handle-alert-detail', '处理告警', '处理或关闭告警事件。', [
      textInput('event_id', '告警编号', true),
      numberInput('status', '处理状态', true),
    ], async (values) => {
      await api.admin.handleAlertEvent(valueText(values, 'event_id'), { status: valueNumber(values, 'status') })
      return '告警事件已处理'
    }),
  ]
}

export function contestManagementActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-contest', '创建竞赛', '创建竞赛草稿，赛制规则由后端统一校验。', [
      textInput('name', '竞赛名称', true),
      numberInput('mode', '赛制', true),
      numberInput('match_mode', '对抗模式'),
      numberInput('team_mode', '组队模式', true),
      datetimeInput('signup_start', '报名开始', true),
      datetimeInput('signup_end', '报名结束', true),
      datetimeInput('start_at', '比赛开始', true),
      datetimeInput('end_at', '比赛结束', true),
      numberInput('freeze_minutes', '封榜分钟数', true),
      textareaInput('rules', '赛事规则', true),
    ], async (values) => {
      await api.contest.createContest({
        name: valueText(values, 'name'),
        mode: valueNumber(values, 'mode'),
        match_mode: optionalNumber(values, 'match_mode'),
        team_mode: valueNumber(values, 'team_mode'),
        signup_start: valueText(values, 'signup_start'),
        signup_end: valueText(values, 'signup_end'),
        start_at: valueText(values, 'start_at'),
        end_at: valueText(values, 'end_at'),
        freeze_minutes: valueNumber(values, 'freeze_minutes'),
        rules: valueJson(values, 'rules'),
      })
      return '竞赛草稿已创建'
    }),
    pageAction('publish-contest', '发布竞赛', '发布竞赛并开放报名或赛前展示。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.publishContest(valueText(values, 'contest_id'))
      return '竞赛已发布'
    }),
    pageAction('start-contest', '开始竞赛', '将竞赛切换到进行中。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.startContest(valueText(values, 'contest_id'))
      return '竞赛已开始'
    }),
    pageAction('end-contest', '结束竞赛', '结束竞赛并停止参赛提交。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.endContest(valueText(values, 'contest_id'))
      return '竞赛已结束'
    }),
    pageAction('add-contest-problem', '添加赛题', '为竞赛添加或更新题目。', [
      textInput('contest_id', '竞赛编号', true),
      textInput('item_code', '题目编码', true),
      textInput('item_version', '题目版本', true),
      numberInput('score', '分值', true),
      numberInput('seq', '顺序', true),
      textareaInput('dynamic_score', '动态分规则'),
      textareaInput('battle_config', '对抗配置'),
      numberInput('battle_rule', '对抗规则'),
    ], async (values) => {
      await api.contest.addProblem(valueText(values, 'contest_id'), {
        item_code: valueText(values, 'item_code'),
        item_version: valueText(values, 'item_version'),
        score: valueNumber(values, 'score'),
        seq: valueNumber(values, 'seq'),
        dynamic_score: valueJson(values, 'dynamic_score'),
        battle_config: valueJson(values, 'battle_config'),
        battle_rule: optionalNumber(values, 'battle_rule'),
      })
      return '竞赛题目已保存'
    }),
  ]
}

export function simGovernanceActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('approve-sim-review', '通过审核', '通过指定仿真包审核。', [textInput('review_id', '审核编号', true)], async (values) => {
      await api.sim.approveReview(valueText(values, 'review_id'))
      return '仿真审核已通过'
    }),
    pageAction('reject-sim-review', '退回审核', '退回仿真包并写入修改意见。', [
      textInput('review_id', '审核编号', true),
      textareaInput('comment', '退回原因', true),
    ], async (values) => {
      await api.sim.rejectReview(valueText(values, 'review_id'), valueText(values, 'comment'))
      return '仿真审核已退回'
    }),
    pageAction('archive-sim-package', '下架仿真包', '下架已发布仿真包。', [textInput('package_id', '仿真包编号', true)], async (values) => {
      await api.sim.archivePackage(valueText(values, 'package_id'))
      return '仿真包已下架'
    }),
    pageAction('republish-sim-package', '重新上架', '重新上架已下架仿真包。', [textInput('package_id', '仿真包编号', true)], async (values) => {
      await api.sim.republishPackage(valueText(values, 'package_id'))
      return '仿真包已重新上架'
    }),
  ]
}

export function pageAction(
  key: string,
  label: string,
  description: string,
  fields: ActionField[],
  execute: (values: ActionValues) => Promise<string>
): PageAction {
  return { key, label, description, fields, submitLabel: label, execute }
}

export function rowAction(
  key: string,
  label: string,
  description: string,
  execute: (row: DataRow) => Promise<string>
): RowAction {
  return { key, label, description, execute }
}

export function textInput(name: string, label: string, required = false, helper?: string): ActionField {
  return { name, label, type: 'text', required, helper }
}

export function passwordInput(name: string, label: string, required = false): ActionField {
  return { name, label, type: 'password', required }
}

export function numberInput(name: string, label: string, required = false, helper?: string): ActionField {
  return { name, label, type: 'number', required, helper }
}

export function textareaInput(name: string, label: string, required = false, helper?: string): ActionField {
  return { name, label, type: 'textarea', required, helper }
}

export function fileInput(name: string, label: string, required = false, helper?: string): ActionField {
  return { name, label, type: 'file', required, helper }
}

export function datetimeInput(name: string, label: string, required = false, helper?: string): ActionField {
  return { name, label, type: 'datetime-local', required, helper }
}

export function valueText(values: ActionValues, key: string): string {
  const value = values[key]
  return typeof value === 'string' ? value.trim() : ''
}

export function optionalText(values: ActionValues, key: string): string | undefined {
  const value = valueText(values, key)
  return value ? value : undefined
}

export function valueNumber(values: ActionValues, key: string): number {
  const parsed = Number(valueText(values, key))
  return Number.isFinite(parsed) ? parsed : 0
}

export function optionalNumber(values: ActionValues, key: string): number | undefined {
  const value = optionalText(values, key)
  if (!value) {
    return undefined
  }
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

export function optionalNumberPayload(values: ActionValues, key: string): { group_id?: number } {
  const value = optionalText(values, key)
  return value ? { group_id: Number(value) } : {}
}

export function valueFile(values: ActionValues, key: string): File {
  const value = values[key]
  if (value instanceof File) {
    return value
  }
  throw new Error('请选择需要上传的文件')
}

export function valueJson(values: ActionValues, key: string): Record<string, unknown> {
  const raw = valueText(values, key)
  if (!raw) {
    return {}
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {}
  } catch {
    throw new Error('配置内容格式不正确，请检查后重试')
  }
}

export function valueJsonArray(values: ActionValues, key: string): Record<string, unknown>[] {
  const raw = valueText(values, key)
  if (!raw) {
    return []
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    return Array.isArray(parsed) ? parsed.filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object' && !Array.isArray(item)) : []
  } catch {
    throw new Error('列表内容格式不正确，请检查后重试')
  }
}

export function valueStringArray(values: ActionValues, key: string): string[] {
  const raw = valueText(values, key)
  return raw ? raw.split(',').map((item) => item.trim()).filter(Boolean) : []
}

export function valueFromRow(row: DataRow, key: string): string {
  const value = row[key]
  return typeof value === 'string' ? value : row.id
}

export function accountTarget(values: ActionValues): 'teacher' | 'student' {
  return valueText(values, 'target_type') === 'teacher' ? 'teacher' : 'student'
}