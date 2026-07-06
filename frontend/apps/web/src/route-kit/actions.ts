// 路由动作定义：集中维护页面动作、行操作、字段构造和提交值转换。

import type { ChaimirApi } from '@chaimir/api-client'
import { routeHref } from '../app/router'
import type { ActionField, ActionValues, DataRow, PageAction, RowAction } from '../app/types'

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
    pageAction('update-course-detail', '更新课程', '更新课程基础信息。', [
      textInput('course_id', '课程编号', true),
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
      await api.teaching.updateCourse(valueText(values, 'course_id'), {
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
      return '课程已更新'
    }),
    pageAction('end-course', '结束课程', '结束课程并进入归档前状态。', [textInput('course_id', '课程编号', true)], async (values) => {
      await api.teaching.endCourse(valueText(values, 'course_id'))
      return '课程已结束'
    }),
    pageAction('clone-course', '克隆课程', '复制课程结构用于新学期。', [
      textInput('course_id', '课程编号', true),
      textInput('name', '新课程名称', true),
    ], async (values) => {
      await api.teaching.cloneCourse(valueText(values, 'course_id'), { name: valueText(values, 'name') })
      return '课程已克隆'
    }),
    pageAction('share-course', '共享课程', '将课程设为可共享复用。', [textInput('course_id', '课程编号', true)], async (values) => {
      await api.teaching.shareCourse(valueText(values, 'course_id'))
      return '课程已共享'
    }),
  ]
}

export function assignmentActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('create-assignment', '创建作业', '创建课程作业并保存题目列表。', [
      textInput('course_id', '课程编号', true),
      textInput('title', '作业标题', true),
      numberInput('chapter_id', '章节编号', true),
      datetimeInput('due_at', '截止时间', true),
      numberInput('max_attempts', '提交次数', true),
      numberInput('late_policy', '迟交策略', true),
      textareaInput('late_penalty', '迟交扣分规则', true),
      textareaInput('items', '题目列表', true),
    ], async (values) => {
      await api.teaching.createAssignment(valueText(values, 'course_id'), {
        title: valueText(values, 'title'),
        chapter_id: valueNumber(values, 'chapter_id'),
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
    pageAction('update-assignment', '更新作业', '更新课程作业配置。', [
      textInput('assignment_id', '作业编号', true),
      textInput('title', '作业标题', true),
      numberInput('chapter_id', '章节编号', true),
      datetimeInput('due_at', '截止时间', true),
      numberInput('max_attempts', '提交次数', true),
      numberInput('late_policy', '迟交策略', true),
      textareaInput('late_penalty', '迟交扣分规则', true),
      textareaInput('items', '题目列表', true),
    ], async (values) => {
      await api.teaching.updateAssignment(valueText(values, 'assignment_id'), {
        title: valueText(values, 'title'),
        chapter_id: valueNumber(values, 'chapter_id'),
        due_at: valueText(values, 'due_at'),
        max_attempts: valueNumber(values, 'max_attempts'),
        late_policy: valueNumber(values, 'late_policy'),
        late_penalty: valueJson(values, 'late_penalty'),
        items: valueJsonArray(values, 'items') as never,
      })
      return '作业已更新'
    }),
  ]
}

export function experimentAuthorActions(api: ChaimirApi): PageAction[] {
  return [
    pageAction('save-experiment', '保存实验草稿', '保存实验编排草稿，步骤状态由平台保存。', [
      textInput('course_id', '课程编号', true),
      textInput('template_ref', '模板来源', true),
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
    pageAction('validate-experiment', '校验实验', '执行实验编排校验。', [textInput('experiment_id', '实验编号', true)], async (values) => {
      await api.experiment.validateExperiment(valueText(values, 'experiment_id'))
      return '实验校验已完成'
    }),
    pageAction('update-experiment', '更新实验草稿', '更新实验编排草稿。', [
      textInput('experiment_id', '实验编号', true),
      textInput('course_id', '课程编号', true),
      textInput('template_ref', '模板来源', true),
      textInput('template_version', '模板版本', true),
      textInput('name', '实验名称', true),
      textareaInput('description', '实验说明', true),
      textareaInput('components', '组件配置', true),
      numberInput('collab_mode', '协作模式', true),
      textareaInput('group_config', '小组配置', true),
      numberInput('wizard_step', '当前步骤', true),
    ], async (values) => {
      await api.experiment.updateExperiment(valueText(values, 'experiment_id'), {
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
      return '实验草稿已更新'
    }),
    pageAction('unpublish-experiment', '取消发布实验', '将已发布实验撤回到不可见状态。', [textInput('experiment_id', '实验编号', true)], async (values) => {
      await api.experiment.unpublishExperiment(valueText(values, 'experiment_id'))
      return '实验已取消发布'
    }),
    pageAction('grade-experiment-report', '批改实验报告', '为实验报告提交人工评分。', [
      textInput('report_id', '报告编号', true),
      numberInput('score', '分数', true),
      textareaInput('comment', '评语', true),
    ], async (values) => {
      await api.experiment.gradeReport(valueText(values, 'report_id'), {
        manual_score: valueNumber(values, 'score'),
        comment: valueText(values, 'comment'),
      })
      return '实验报告已批改'
    }),
    pageAction('create-experiment-group', '创建实验小组', '为多人协作实验创建小组。', [
      textInput('experiment_id', '实验编号', true),
      textInput('name', '小组名称', true),
    ], async (values) => {
      await api.experiment.createGroup(valueText(values, 'experiment_id'), {
        name: valueText(values, 'name'),
      })
      return '实验小组已创建'
    }),
    pageAction('upsert-group-member', '维护小组成员', '添加或更新实验小组成员。', [
      textInput('group_id', '小组编号', true),
      numberInput('student_id', '学生编号', true),
      textInput('role', '成员角色', true),
    ], async (values) => {
      await api.experiment.upsertGroupMember(valueText(values, 'group_id'), {
        student_id: valueNumber(values, 'student_id'),
        role: valueText(values, 'role'),
      })
      return '小组成员已保存'
    }),
    pageAction('read-instance-progress', '读取实例进度', '读取实验实例订阅和进度信息。', [textInput('instance_id', '实例编号', true)], async (values) => {
      await api.experiment.getProgress(valueText(values, 'instance_id'))
      return '实例进度已读取'
    }),
    pageAction('read-experiment-group', '读取小组详情', '按小组编号读取协作小组详情。', [textInput('group_id', '小组编号', true)], async (values) => {
      await api.experiment.getGroup(valueText(values, 'group_id'))
      return '小组详情已读取'
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
        enabled: valueFlag(values, 'enabled'),
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
      textInput('init_code_ref', '初始化代码来源'),
      textInput('init_script_ref', '初始化脚本来源'),
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
    pageAction('update-content-detail', '更新内容', '更新题目或模板草稿。', [
      textInput('item_id', '内容编号', true),
      textInput('title', '标题', true),
      numberInput('category_id', '分类编号', true),
      numberInput('difficulty', '难度', true),
      textInput('tags', '标签', false, '多个标签用英文逗号分隔。'),
      textInput('knowledge_points', '知识点', false, '多个知识点用英文逗号分隔。'),
      numberInput('visibility', '可见性', true),
      textareaInput('body', '内容正文', true),
      textInput('sensitive_fields', '需保护信息', false, '多个项目用英文逗号分隔。'),
    ], async (values) => {
      await api.content.updateItem(valueText(values, 'item_id'), {
        title: valueText(values, 'title'),
        category_id: valueNumber(values, 'category_id'),
        difficulty: valueNumber(values, 'difficulty'),
        tags: valueStringArray(values, 'tags'),
        knowledge_points: valueStringArray(values, 'knowledge_points'),
        visibility: valueNumber(values, 'visibility'),
        body: valueJson(values, 'body'),
        sensitive_fields: valueStringArray(values, 'sensitive_fields'),
      })
      return '内容已更新'
    }),
    pageAction('create-content-version', '创建新版本', '基于已有版本创建新草稿。', [
      textInput('code', '内容编码', true),
      textInput('source_version', '来源版本', true),
      textInput('new_version', '新版本', true),
    ], async (values) => {
      await api.content.createNewVersion(valueText(values, 'code'), {
        source_version: valueText(values, 'source_version'),
        new_version: valueText(values, 'new_version'),
      })
      return '新版本草稿已创建'
    }),
    pageAction('clone-content', '克隆内容', '把已有内容克隆为独立草稿。', [
      textInput('code', '内容编码', true),
      textInput('version', '版本', true),
      textInput('new_code', '新编码', true),
      textInput('new_version', '新版本', true),
    ], async (values) => {
      await api.content.cloneItem(valueText(values, 'code'), valueText(values, 'version'), {
        new_code: valueText(values, 'new_code'),
        new_version: valueText(values, 'new_version'),
      })
      return '内容已克隆'
    }),
    pageAction('read-content-versions', '查看版本', '读取指定内容的版本列表。', [textInput('code', '内容编码', true)], async (values) => {
      await api.content.getVersions(valueText(values, 'code'))
      return '内容版本已读取'
    }),
    pageAction('issue-attachment-grant', '附件下载授权', '为题库附件签发短时下载授权。', [
      textInput('resource_id', '资源编号', true),
      textInput('object_ref', '文件位置', true),
    ], async (values) => {
      await api.content.issueAttachmentDownloadGrant({
        resource_id: valueText(values, 'resource_id'),
        object_ref: valueText(values, 'object_ref'),
      })
      return '附件下载授权已生成'
    }),
    pageAction('upload-content-attachment', '上传附件', '上传题库附件并绑定到指定资源。', [
      fileInput('file', '附件文件', true),
      textInput('resource_id', '资源编号'),
    ], async (values) => {
      await api.content.uploadAttachment(valueFile(values, 'file'), optionalText(values, 'resource_id'))
      return '附件已上传'
    }),
    pageAction('create-category', '创建分类', '创建题库分类。', [
      numberInput('parent_id', '上级分类编号', true),
      textInput('name', '分类名称', true),
      numberInput('sort', '顺序', true),
    ], async (values) => {
      await api.content.createCategory({
        parent_id: valueNumber(values, 'parent_id'),
        name: valueText(values, 'name'),
        sort: valueNumber(values, 'sort'),
      })
      return '分类已创建'
    }),
    pageAction('read-item-face', '读取题面', '读取学生可见的脱敏题面。', [
      textInput('code', '内容编码', true),
      textInput('version', '版本', true),
    ], async (values) => {
      await api.content.getItemFace(valueText(values, 'code'), valueText(values, 'version'))
      return '题面已读取'
    }),
    pageAction('read-item-full', '读取全量内容', '教师侧读取完整内容，学生端不可使用。', [
      textInput('code', '内容编码', true),
      textInput('version', '版本', true),
    ], async (values) => {
      await api.content.getItemFull(valueText(values, 'code'), valueText(values, 'version'))
      return '完整内容已读取'
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
    pageAction('read-paper', '查看试卷详情', '读取试卷题目和组卷条件。', [textInput('paper_id', '试卷编号', true)], async (values) => {
      await api.content.getPaper(valueText(values, 'paper_id'))
      return '试卷详情已读取'
    }),
    pageAction('regenerate-paper', '重新组卷', '按当前组卷条件重新生成试卷题目。', [textInput('paper_id', '试卷编号', true)], async (values) => {
      await api.content.regeneratePaper(valueText(values, 'paper_id'))
      return '试卷已重新生成'
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
    pageAction('create-level-config', '创建等级配置', '创建成绩等级映射和预警规则。', [
      textInput('name', '配置名称', true),
      textareaInput('mapping', '等级映射', true),
      textareaInput('warning_rules', '预警规则', true),
      numberInput('is_default', '是否默认', true),
    ], async (values) => {
      await api.grade.createLevelConfig({
        name: valueText(values, 'name'),
        mapping: valueJsonArray(values, 'mapping') as never,
        warning_rules: valueJson(values, 'warning_rules') as never,
        is_default: valueFlag(values, 'is_default'),
      })
      return '等级配置已创建'
    }),
    pageAction('update-level-config', '更新等级配置', '更新成绩等级映射和预警规则。', [
      textInput('config_id', '配置编号', true),
      textInput('name', '配置名称', true),
      textareaInput('mapping', '等级映射', true),
      textareaInput('warning_rules', '预警规则', true),
      numberInput('is_default', '是否默认', true),
    ], async (values) => {
      await api.grade.updateLevelConfig(valueText(values, 'config_id'), {
        name: valueText(values, 'name'),
        mapping: valueJsonArray(values, 'mapping') as never,
        warning_rules: valueJson(values, 'warning_rules') as never,
        is_default: valueFlag(values, 'is_default'),
      })
      return '等级配置已更新'
    }),
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
        is_current: valueFlag(values, 'is_current'),
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
    pageAction('list-semesters', '读取学期', '读取成绩中心学期列表。', [], async () => {
      await api.grade.listSemesters()
      return '学期列表已读取'
    }),
    pageAction('read-warning-rules', '读取预警规则', '读取当前学校学业预警规则。', [], async () => {
      await api.grade.getWarningRules()
      return '预警规则已读取'
    }),
    pageAction('read-student-grades', '读取学生成绩', '按学生编号读取成绩聚合结果。', [
      textInput('student_id', '学生编号', true),
      textInput('semester_id', '学期编号'),
    ], async (values) => {
      await api.grade.studentGrades(valueText(values, 'student_id'), optionalText(values, 'semester_id'))
      return '学生成绩已读取'
    }),
    pageAction('recompute-student-grade', '重算学生成绩', '按学生和学期重算成绩聚合。', [
      textInput('student_id', '学生编号', true),
      textInput('semester_id', '学期编号', true),
    ], async (values) => {
      await api.grade.recomputeStudentGrade(valueText(values, 'student_id'), { semester_id: valueText(values, 'semester_id') })
      return '学生成绩已重算'
    }),
    pageAction('generate-transcript-batch', '批量生成成绩单', '按学生列表批量生成成绩单。', [
      textInput('student_ids', '学生编号', true, '多个编号用英文逗号分隔。'),
      numberInput('scope', '成绩单范围', true),
      textInput('semester_id', '学期编号'),
    ], async (values) => {
      await api.grade.generateTranscriptBatch({
        student_ids: valueNumberArray(values, 'student_ids'),
        scope: valueNumber(values, 'scope'),
        semester_id: optionalText(values, 'semester_id'),
      })
      return '批量成绩单已生成'
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
    pageAction('create-contest', '创建竞赛', '创建竞赛草稿，赛制规则会统一校验。', [
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
    pageAction('update-contest', '更新竞赛', '更新竞赛基础配置。', [
      textInput('contest_id', '竞赛编号', true),
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
      await api.contest.updateContest(valueText(values, 'contest_id'), {
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
      return '竞赛已更新'
    }),
    pageAction('freeze-contest', '冻结榜单', '冻结竞赛榜单展示。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.freezeContest(valueText(values, 'contest_id'))
      return '竞赛榜单已冻结'
    }),
    pageAction('archive-contest', '归档竞赛', '归档竞赛并生成结果快照。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.archiveContest(valueText(values, 'contest_id'))
      return '竞赛已归档'
    }),
    pageAction('read-result-snapshot', '读取结果快照', '读取竞赛归档结果快照。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
      await api.contest.getResultSnapshot(valueText(values, 'contest_id'))
      return '竞赛结果快照已读取'
    }),
    pageAction('read-team', '读取队伍', '按队伍编号读取队伍详情。', [textInput('team_id', '队伍编号', true)], async (values) => {
      await api.contest.getTeam(valueText(values, 'team_id'))
      return '队伍详情已读取'
    }),
    pageAction('read-cheat-suspects', '读取可疑线索', '读取竞赛防作弊可疑线索。', [
      textInput('contest_id', '竞赛编号', true),
      numberInput('problem_id', '题目编号', true),
      textInput('code_hash', '代码校验值'),
      textInput('exclude_source_ref', '排除来源'),
      numberInput('threshold', '相似阈值'),
    ], async (values) => {
      await api.contest.listCheatSuspects(valueText(values, 'contest_id'), {
        problem_id: valueNumber(values, 'problem_id'),
        code_hash: optionalText(values, 'code_hash'),
        exclude_source_ref: optionalText(values, 'exclude_source_ref'),
        threshold: optionalNumber(values, 'threshold'),
      })
      return '可疑线索已读取'
    }),
    pageAction('import-vuln-source-problem', '导入漏洞源题', '从漏洞源导入题目素材。', [
      numberInput('source_id', '漏洞源编号'),
      textInput('external_ref', '外部来源'),
      textInput('title', '标题', true),
      numberInput('level', '等级', true),
      numberInput('runtime_mode', '运行模式', true),
      textareaInput('draft_body', '草稿正文', true),
    ], async (values) => {
      await api.contest.importVulnSourceProblem({
        source_id: optionalNumber(values, 'source_id'),
        external_ref: optionalText(values, 'external_ref'),
        title: valueText(values, 'title'),
        level: valueNumber(values, 'level'),
        runtime_mode: valueNumber(values, 'runtime_mode'),
        draft_body: valueJson(values, 'draft_body'),
      })
      return '漏洞源题目已导入'
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
    pageAction('update-sim-package', '更新仿真包', '更新草稿或退回后的仿真包文件和元数据。', [
      textInput('package_id', '仿真包编号', true),
      fileInput('bundle', '仿真包文件', true),
      textInput('code', '仿真包编码', true),
      textInput('version', '版本', true),
      textInput('name', '名称', true),
      textInput('category', '分类', true),
      textInput('compute', '执行方式', true, '填写 frontend 或 backend。'),
      textareaInput('scale_limit', '规模限制'),
      textInput('backend_adapter', '平台适配器'),
      textareaInput('backend_config', '平台计算配置'),
    ], async (values) => {
      await api.sim.updatePackage(valueText(values, 'package_id'), {
        bundle: valueFile(values, 'bundle'),
        code: valueText(values, 'code'),
        version: valueText(values, 'version'),
        name: valueText(values, 'name'),
        category: valueText(values, 'category'),
        compute: valueText(values, 'compute') === 'backend' ? 'backend' : 'frontend',
        scale_limit: valueJson(values, 'scale_limit'),
        backend_adapter: optionalText(values, 'backend_adapter'),
        backend_config: valueJson(values, 'backend_config'),
      })
      return '仿真包已更新'
    }),
    pageAction('preview-sim-package', '预览仿真包', '读取仿真包预览和校验报告。', [textInput('package_id', '仿真包编号', true)], async (values) => {
      await api.sim.previewPackage(valueText(values, 'package_id'))
      return '仿真包预览已读取'
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

/**
 * navigateRowAction 将列表行连接到对应深页或流程页，避免用户手动复制资源编号。
 */
export function navigateRowAction(
  key: string,
  label: string,
  description: string,
  path: string,
  paramName = 'id',
  valueKey = 'id'
): RowAction {
  return rowAction(key, label, description, async (row) => {
    window.location.hash = routeHref(path, { [paramName]: valueFromRow(row, valueKey) }).slice(1)
    return `正在进入${label}`
  })
}

/**
 * navigatePageAction 为无行上下文的流程页提供明确入口，避免把深页藏成手输地址。
 */
export function navigatePageAction(key: string, label: string, description: string, path: string): PageAction {
  return pageAction(key, label, description, [], async () => {
    window.location.hash = routeHref(path).slice(1)
    return `正在进入${label}`
  })
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

export function valueFlag(values: ActionValues, key: string): boolean {
  return valueNumber(values, key) === 1
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

export function valueNumberArray(values: ActionValues, key: string): number[] {
  return valueStringArray(values, key)
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item))
}

export function valueFromRow(row: DataRow, key: string): string {
  const value = row[key]
  return typeof value === 'string' ? value : row.id
}

export function accountTarget(values: ActionValues): 'teacher' | 'student' {
  return valueText(values, 'target_type') === 'teacher' ? 'teacher' : 'student'
}
