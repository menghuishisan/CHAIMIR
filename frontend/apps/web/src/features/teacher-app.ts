// 教师端路由：教学、实践、资源、成绩报送、组织与账户页面定义。

import { BookOpen, Building2, ClipboardCheck, ClipboardList, FilePenLine, FileText, Flag, Layers, Library, ListChecks, MonitorCog, Network, Pencil, ScrollText, ShieldAlert, ShieldCheck, Swords, Users } from 'lucide-react'
import { routeHref, type AppDefinition, type MetricItem, type ResourceResult } from '@chaimir/shared'
import {
  appealColumns,
  arrayResult,
  assignmentActions,
  assignmentColumns,
  chapterColumns,
  cheatColumns,
  contestColumns,
  contestManagementActions,
  contestProblemColumns,
  contentActions,
  contentCategoryColumns,
  contentColumns,
  courseColumns,
  datetimeInput,
  defaultPageParams,
  emptyResult,
  experimentAuthorActions,
  experimentColumns,
  gradeReviewColumns,
  hiddenResourceRoute,
  judgeTaskColumns,
  listResult,
  memberColumns,
  navigatePageAction,
  navigateRowAction,
  numberInput,
  objectResult,
  optionalNumber,
  optionalText,
  orgColumns,
  pageAction,
  paperActions,
  paperColumns,
  routeParam,
  rowAction,
  resourceRoute,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  sharedTransferRoute,
  simPackageActions,
  simPackageColumns,
  submissionColumns,
  teacherCourseActions,
  teachingGradeColumns,
  teachingPostColumns,
  textInput,
  textareaInput,
  valueFlag,
  valueFromRow,
  valueJson,
  valueJsonArray,
  valueNumber,
  valueNumberArray,
  valueStringArray,
  valueText,
  vulnProblemActions,
  vulnProblemColumns,
  vulnSourceActions,
  vulnSourceColumns,
} from '../route-kit'

export const teacherApp: AppDefinition = {
  role: 'teacher',
  title: '教师端',
  subtitle: '课程建设、实验编排、题库内容、批改与教学诊断',
  homePath: 'courses',
  routes: [
    {
      path: 'courses',
      label: '课程管理',
      description: '管理授课课程、成员、章节和发布状态',
      icon: BookOpen,
      group: '教学',
      load: async (api) => ({
        ...teacherMetrics(listResult(await api.teaching.getCourses({ role: 'teacher', ...defaultPageParams() }), courseColumns(), '暂无课程', '创建课程后会在这里显示。'), '授课课程', '章节成员', '发布归档'),
        actions: [
          navigatePageAction('open-assignments-page', '作业管理', '进入课程作业、提交和批改入口。', 'assignments'),
          navigatePageAction('open-assignment-edit-page', '作业编辑', '进入作业创建、更新和发布页面。', 'assignment-edit'),
          navigatePageAction('open-judge-page', '判题任务', '进入判题进度、人工评分和重判页面。', 'judge'),
          pageAction('create-course', '创建课程', '创建课程基础信息，章节和课时在课程详情中继续维护。', [
            textInput('name', '课程名称', true),
            textareaInput('description', '课程说明', true),
            numberInput('type', '课程类型', true),
            numberInput('difficulty', '难度', true),
            textInput('semester', '学期', true),
            numberInput('credits', '学分', true),
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
              schedule: {},
              start_at: valueText(values, 'start_at'),
              end_at: valueText(values, 'end_at'),
            })
            return '课程已创建'
          }),
          pageAction('publish-course', '发布课程', '发布课程后学生可以查看课程内容。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.publishCourse(valueText(values, 'course_id'))
            return '课程已发布'
          }),
          pageAction('archive-course', '归档课程', '归档已完成课程，保留历史学习记录。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.archiveCourse(valueText(values, 'course_id'))
            return '课程已归档'
          }),
        ],
        rowActions: [
          navigateRowAction('open-course-edit', '编辑课程', '进入课程基础信息编辑页。', 'course-edit'),
          navigateRowAction('open-chapters', '章节课时', '进入章节和课时维护页。', 'chapters', 'course_id'),
          navigateRowAction('open-members', '选课成员', '进入课程成员管理页。', 'members', 'course_id'),
          navigateRowAction('open-community', '课程讨论', '进入课程讨论和公告页。', 'course-community', 'course_id'),
          navigateRowAction('open-course-grades', '课程成绩', '进入成绩权重、计算和导出页。', 'course-grades', 'course_id'),
        ],
      }),
    },
    {
      path: 'experiments',
      label: '实验编排',
      description: '配置实验组件、检查点、报告和协作小组',
      icon: Layers,
      group: '实践',
      load: async (api) => ({
        ...teacherMetrics(listResult(await api.experiment.getExperiments(defaultPageParams()), experimentColumns(), '暂无实验', '新建实验后会在这里显示。'), '实验草稿', '编排校验', '发布实验'),
        actions: [
          pageAction('create-experiment', '创建实验', '创建实验编排草稿，组件配置会统一校验。', [
            textInput('course_id', '课程编号', true),
            textInput('template_ref', '模板引用', true),
            textInput('template_version', '模板版本', true),
            textInput('name', '实验名称', true),
            textareaInput('description', '实验说明', true),
            textareaInput('components', '组件配置', true, '包含 envs、sims、checkpoints、stages。'),
            numberInput('collab_mode', '协作模式', true),
            textareaInput('group_config', '小组配置', true),
          ], async (values) => {
            await api.experiment.createExperiment({
              course_id: Number(valueText(values, 'course_id')),
              template_ref: valueText(values, 'template_ref'),
              template_version: valueText(values, 'template_version'),
              name: valueText(values, 'name'),
              description: valueText(values, 'description'),
              components: valueJson(values, 'components') as never,
              collab_mode: valueNumber(values, 'collab_mode'),
              group_config: valueJson(values, 'group_config') as never,
              require_report: true,
              wizard_step: 1,
            })
            return '实验草稿已创建'
          }),
          pageAction('publish-experiment', '发布实验', '校验通过后发布实验。', [textInput('experiment_id', '实验编号', true)], async (values) => {
            const id = valueText(values, 'experiment_id')
            await api.experiment.validateExperiment(id)
            await api.experiment.publishExperiment(id)
            return '实验已校验并发布'
          }),
        ],
        rowActions: [
          navigateRowAction('open-exp-wizard', '继续编排', '进入实验编排向导。', 'exp-wizard', 'experiment_id'),
        ],
      }),
    },
    {
      path: 'content',
      label: '题库内容',
      description: '管理题目、模板、版本和发布状态',
      icon: Library,
      group: '资源',
      load: async (api) => ({
        ...teacherMetrics(listResult(await api.content.getItems(defaultPageParams()), contentColumns(), '暂无内容', '创建题目或模板后会在这里显示。'), '题库内容', '版本发布', '共享复用'),
        actions: [
          navigatePageAction('open-content-categories-page', '题库分类', '进入分类树和展示顺序维护页面。', 'content-categories'),
          pageAction('create-content', '创建内容', '创建题目或模板草稿，答案等敏感内容只在授权场景可见。', [
            textInput('code', '内容编码', true),
            textInput('version', '版本', true),
            numberInput('type', '内容类型', true),
            textInput('title', '标题', true),
            numberInput('category_id', '分类编号', true),
            numberInput('difficulty', '难度', true),
            textInput('tags', '标签', false, '多个标签用英文逗号分隔。'),
            textInput('knowledge_points', '知识点', false, '多个知识点用英文逗号分隔。'),
            numberInput('visibility', '可见性', true),
            textareaInput('body', '内容正文', true),
            textInput('sensitive_fields', '敏感字段', false, '多个字段用英文逗号分隔。'),
          ], async (values) => {
            await api.content.createItem({
              code: valueText(values, 'code'),
              version: valueText(values, 'version'),
              type: valueNumber(values, 'type'),
              title: valueText(values, 'title'),
              category_id: valueNumber(values, 'category_id'),
              difficulty: valueNumber(values, 'difficulty'),
              tags: valueStringArray(values, 'tags'),
              knowledge_points: valueStringArray(values, 'knowledge_points'),
              visibility: valueNumber(values, 'visibility'),
              body: valueJson(values, 'body'),
              sensitive_fields: valueStringArray(values, 'sensitive_fields'),
            })
            return '内容草稿已创建'
          }),
          pageAction('publish-content', '发布内容', '发布内容后进入可用状态。', [textInput('item_id', '内容编号', true)], async (values) => {
            await api.content.publishItem(valueText(values, 'item_id'))
            return '内容已发布'
          }),
        ],
        rowActions: [
          navigateRowAction('open-content-edit', '编辑内容', '进入内容版本和附件维护页。', 'content-edit'),
          rowAction('share-content', '共享', '设为共享资源库可见。', async (row) => {
            await api.content.shareItem(row.id)
            return '内容已共享'
          }),
          rowAction('unshare-content', '取消共享', '取消共享资源库可见。', async (row) => {
            await api.content.unshareItem(row.id)
            return '内容已取消共享'
          }),
          rowAction('deprecate-content', '弃用', '弃用已发布内容。', async (row) => {
            await api.content.deprecateItem(row.id)
            return '内容已弃用'
          }),
          rowAction('delete-content', '删除', '删除草稿内容。', async (row) => {
            await api.content.deleteItem(row.id)
            return '内容已删除'
          }),
        ],
      }),
    },
    {
      path: 'papers',
      label: '试卷组卷',
      description: '维护试卷与自动组卷结果',
      icon: FileText,
      group: '资源',
      load: async (api) => ({
        ...teacherMetrics(listResult(await api.content.listPapers(defaultPageParams()), paperColumns(), '暂无试卷', '创建试卷后会在这里显示。'), '试卷数量', '组卷条件', '重生成'),
        actions: [
          pageAction('create-paper', '创建试卷', '按手工题目列表或组卷条件创建试卷。', [
            textInput('name', '试卷名称', true),
            numberInput('gen_mode', '组卷方式', true),
            textareaInput('gen_criteria', '组卷条件', false),
            textareaInput('items', '题目列表', false, '数组，每项包含 code、version、score。'),
          ], async (values) => {
            await api.content.createPaper({
              name: valueText(values, 'name'),
              gen_mode: valueNumber(values, 'gen_mode'),
              gen_criteria: valueJson(values, 'gen_criteria'),
              items: valueJsonArray(values, 'items') as never,
            })
            return '试卷已创建'
          }),
        ],
        rowActions: [
          navigateRowAction('open-paper-edit', '编辑试卷', '进入组卷条件和题目维护页。', 'paper-edit', 'paper_id'),
        ],
      }),
    },
    {
      path: 'judge',
      label: '判题任务',
      description: '查看判题进度、人工评分和重判状态',
      icon: ClipboardCheck,
      group: '教学',
      hidden: true,
      load: async (api) => ({
        ...listResult(await api.judge.getTasks(defaultPageParams()), judgeTaskColumns(), '暂无判题任务', '学生提交需要判题的作业后会在这里显示。'),
        actions: [
          pageAction('manual-score', '人工评分', '为需要人工评分的判题任务提交分数和评语。', [
            textInput('task_id', '判题任务编号', true),
            numberInput('score', '得分', true),
            numberInput('max_score', '满分', true),
            numberInput('passed', '是否通过', true, '1 表示通过，0 表示未通过。'),
            textareaInput('comment', '评语', true),
          ], async (values) => {
            await api.judge.manualScore(valueText(values, 'task_id'), {
              score: valueNumber(values, 'score'),
              max_score: valueNumber(values, 'max_score'),
              passed: valueFlag(values, 'passed'),
              comment: valueText(values, 'comment'),
            })
            return '人工评分已提交'
          }),
          pageAction('read-judge-task', '查看任务详情', '按任务编号读取判题任务详情。', [textInput('task_id', '判题任务编号', true)], async (values) => {
            await api.judge.getTask(valueText(values, 'task_id'))
            return '判题任务详情已读取'
          }),
          pageAction('prepare-judge-progress', '准备判题进度', '准备指定判题任务的实时进度信息。', [textInput('task_id', '判题任务编号', true)], async (values) => {
            api.judge.getProgressWsUrl(valueText(values, 'task_id'))
            return '判题进度已准备'
          }),
        ],
        rowActions: [
          rowAction('rejudge-task', '重判', '按原始快照触发重判。', async (row) => {
            await api.judge.rejudgeTask(row.id)
            return '重判任务已提交'
          }),
        ],
      }),
    },
    {
      path: 'grade-appeals',
      label: '成绩申诉',
      description: '处理学生成绩申诉与反馈',
      icon: ShieldAlert,
      group: '成绩',
      hidden: true,
      load: async (api) => ({
        ...listResult(await api.grade.listAppeals(defaultPageParams()), appealColumns(), '暂无申诉', '有学生提交申诉后会在这里显示。'),
        actions: [
          pageAction('accept-appeal', '受理申诉', '受理学生成绩申诉并写入处理说明。', [
            textInput('appeal_id', '申诉编号', true),
            textareaInput('comment', '处理说明', true),
          ], async (values) => {
            await api.grade.acceptAppeal(valueText(values, 'appeal_id'), { comment: valueText(values, 'comment') })
            return '成绩申诉已受理'
          }),
          pageAction('reject-appeal', '驳回申诉', '驳回学生成绩申诉并写入原因。', [
            textInput('appeal_id', '申诉编号', true),
            textareaInput('comment', '驳回原因', true),
          ], async (values) => {
            await api.grade.rejectAppeal(valueText(values, 'appeal_id'), { comment: valueText(values, 'comment') })
            return '成绩申诉已驳回'
          }),
        ],
      }),
    },
    {
      path: 'contests',
      label: '赛事组织',
      description: '创建竞赛、编排题目、查看榜单与违规线索',
      icon: Swords,
      group: '实践',
      load: async (api) => ({
        ...teacherMetrics(listResult(await api.contest.getContests(defaultPageParams()), contestColumns(), '暂无竞赛', '创建竞赛后会在这里显示。'), '赛事数量', '赛题配置', '状态流转'),
        actions: [
          navigatePageAction('open-vuln-sources-page', '漏洞源管理', '进入漏洞来源、同步和素材入库页面。', 'vuln-sources'),
          navigatePageAction('open-vuln-transform-page', '漏洞题转化', '进入漏洞素材转题和预验证页面。', 'vuln-transform'),
          ...contestManagementActions(api),
        ],
        rowActions: [
          navigateRowAction('open-contest-edit', '配置竞赛', '进入竞赛基础信息和赛制配置页。', 'contest-edit', 'contest_id'),
          navigateRowAction('open-contest-problems', '竞赛出题', '进入竞赛题目和对抗配置页。', 'contest-problems', 'contest_id'),
          navigateRowAction('open-cheat-review', '防作弊', '进入防作弊线索处理页。', 'cheat-review', 'contest_id'),
        ],
      }),
    },
    ...teacherDeepRoutes(),
    sharedNotificationRoute(),
    sharedAnnouncementRoute(),
    sharedTransferRoute(),
    sharedProfileRoute(),
  ],
}
/**
 * teacherDeepRoutes 补齐教师端课程、作业、实验、竞赛、监控、资源和报送子页。
 */
function teacherDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('course-edit', '课程编辑', '创建或更新课程基础信息', Pencil, async (api) => ({
      ...listResult(await api.teaching.getCourses({ role: 'teacher', ...defaultPageParams() }), courseColumns(), '暂无课程', '创建课程后可继续维护章节和成员。'),
      actions: teacherCourseActions(api),
    })),
    hiddenResourceRoute('chapters', '章节课时', '维护课程章节和课时内容', ClipboardList, async (api, params) => {
      const courseId = routeParam(params, 'course_id', 'id')
      return {
        ...(courseId ? arrayResult(await api.teaching.listChapters(courseId), chapterColumns(), '暂无章节', '创建章节后会显示。') : emptyResult(chapterColumns(), '请选择课程', '从课程编辑页进入章节维护。')),
        actions: [
          pageAction('create-chapter', '创建章节', '为课程创建章节。', [
            textInput('course_id', '课程编号', true),
            textInput('title', '章节标题', true),
            numberInput('sort', '顺序', true),
          ], async (values) => {
            await api.teaching.createChapter(valueText(values, 'course_id'), { title: valueText(values, 'title'), sort: valueNumber(values, 'sort') })
            return '章节已创建'
          }),
          pageAction('create-lesson', '创建课时', '在章节下创建课时。', [
            textInput('chapter_id', '章节编号', true),
            textInput('title', '课时标题', true),
            numberInput('sort', '顺序', true),
            numberInput('content_type', '内容类型', true),
            textareaInput('content_ref', '课时内容引用', true),
          ], async (values) => {
            await api.teaching.createLesson(valueText(values, 'chapter_id'), {
              title: valueText(values, 'title'),
              sort: valueNumber(values, 'sort'),
              content_type: valueNumber(values, 'content_type'),
              content_ref: valueJson(values, 'content_ref'),
            })
            return '课时已创建'
          }),
          pageAction('update-chapter', '更新章节', '更新课程章节标题和顺序。', [
            textInput('course_id', '课程编号', true),
            textInput('chapter_id', '章节编号', true),
            textInput('title', '章节标题', true),
            numberInput('sort', '顺序', true),
          ], async (values) => {
            await api.teaching.updateChapter(valueText(values, 'course_id'), valueText(values, 'chapter_id'), {
              title: valueText(values, 'title'),
              sort: valueNumber(values, 'sort'),
            })
            return '章节已更新'
          }),
          pageAction('delete-chapter', '删除章节', '删除未被引用的章节。', [
            textInput('course_id', '课程编号', true),
            textInput('chapter_id', '章节编号', true),
          ], async (values) => {
            await api.teaching.deleteChapter(valueText(values, 'course_id'), valueText(values, 'chapter_id'))
            return '章节已删除'
          }),
          pageAction('list-lessons', '读取课时', '读取章节下的课时列表。', [textInput('chapter_id', '章节编号', true)], async (values) => {
            await api.teaching.listLessons(valueText(values, 'chapter_id'))
            return '课时列表已读取'
          }),
          pageAction('update-lesson', '更新课时', '更新课时标题、内容引用和顺序。', [
            textInput('chapter_id', '章节编号', true),
            textInput('lesson_id', '课时编号', true),
            textInput('title', '课时标题', true),
            numberInput('content_type', '内容类型', true),
            textareaInput('content_ref', '课时内容引用', true),
            numberInput('sort', '顺序', true),
          ], async (values) => {
            await api.teaching.updateLesson(valueText(values, 'chapter_id'), valueText(values, 'lesson_id'), {
              title: valueText(values, 'title'),
              content_type: valueNumber(values, 'content_type'),
              content_ref: valueJson(values, 'content_ref'),
              sort: valueNumber(values, 'sort'),
            })
            return '课时已更新'
          }),
          pageAction('set-lesson-content', '设置课时内容', '把课时关联到内容、实验或仿真资源。', [
            textInput('lesson_id', '课时编号', true),
            numberInput('content_type', '内容类型', true),
            textareaInput('content_ref', '内容引用', true),
          ], async (values) => {
            await api.teaching.setLessonContent(valueText(values, 'lesson_id'), {
              content_type: valueNumber(values, 'content_type'),
              content_ref: valueJson(values, 'content_ref'),
            })
            return '课时内容已设置'
          }),
          pageAction('delete-lesson', '删除课时', '删除未被引用的课时。', [
            textInput('chapter_id', '章节编号', true),
            textInput('lesson_id', '课时编号', true),
          ], async (values) => {
            await api.teaching.deleteLesson(valueText(values, 'chapter_id'), valueText(values, 'lesson_id'))
            return '课时已删除'
          }),
        ],
      }
    }),
    hiddenResourceRoute('members', '选课成员', '管理课程成员和选课名单', Users, async (api, params) => {
      const courseId = routeParam(params, 'course_id', 'id')
      return {
        ...(courseId ? listResult(await api.teaching.listMembers(courseId, defaultPageParams()), memberColumns(), '暂无成员', '添加学生后会显示。') : emptyResult(memberColumns(), '请选择课程', '从课程编辑页进入成员管理。')),
        actions: [
          pageAction('add-members', '添加成员', '按学生编号批量加入课程。', [
            textInput('course_id', '课程编号', true),
            textInput('student_ids', '学生编号', true, '多个编号用英文逗号分隔。'),
          ], async (values) => {
            await api.teaching.addMembers(valueText(values, 'course_id'), { student_ids: valueNumberArray(values, 'student_ids') })
            return '课程成员已添加'
          }),
        ],
        rowActions: [
          rowAction('remove-member', '移除', '移除课程成员。', async (row) => {
            await api.teaching.removeMember(valueFromRow(row, 'course_id'), valueFromRow(row, 'student_id'))
            return '课程成员已移除'
          }),
        ],
      }
    }),
    hiddenResourceRoute('course-community', '课程讨论', '维护课程讨论、公告和课程评价', FileText, async (api, params) => {
      const courseId = routeParam(params, 'course_id', 'id')
      return {
        ...(courseId
          ? listResult(await api.teaching.listPosts(courseId, defaultPageParams()), teachingPostColumns(), '暂无讨论', '发布讨论或公告后会显示课程互动。')
          : emptyResult(teachingPostColumns(), '请选择课程', '从课程管理进入具体课程后可维护讨论和公告。')),
        actions: [
          pageAction('list-posts', '读取讨论', '读取课程讨论列表。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.listPosts(valueText(values, 'course_id'), defaultPageParams())
            return '课程讨论已读取'
          }),
          pageAction('create-post', '发布讨论', '发布课程讨论内容。', [
            textInput('course_id', '课程编号', true),
            numberInput('parent_id', '回复对象编号'),
            textareaInput('content', '讨论内容', true),
          ], async (values) => {
            await api.teaching.createPost(valueText(values, 'course_id'), {
              parent_id: optionalNumber(values, 'parent_id'),
              content: valueText(values, 'content'),
            })
            return '讨论已发布'
          }),
          pageAction('create-course-announcement', '发布课程公告', '发布课程内公告。', [
            textInput('course_id', '课程编号', true),
            textInput('title', '公告标题', true),
            textareaInput('content', '公告内容', true),
            numberInput('is_pinned', '是否置顶', true),
          ], async (values) => {
            await api.teaching.createAnnouncement(valueText(values, 'course_id'), {
              title: valueText(values, 'title'),
              content: valueText(values, 'content'),
              is_pinned: valueFlag(values, 'is_pinned'),
            })
            return '课程公告已发布'
          }),
          pageAction('list-course-announcements', '读取课程公告', '读取课程公告列表。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.listAnnouncements(valueText(values, 'course_id'))
            return '课程公告已读取'
          }),
          pageAction('pin-course-announcement', '置顶课程公告', '按公告编号置顶课程公告。', [textInput('announcement_id', '公告编号', true)], async (values) => {
            await api.teaching.pinAnnouncement(valueText(values, 'announcement_id'))
            return '课程公告已置顶'
          }),
        ],
        rowActions: [
          rowAction('like-post', '点赞', '为讨论点赞。', async (row) => {
            await api.teaching.likePost(row.id)
            return '讨论已点赞'
          }),
          rowAction('pin-post', '置顶', '置顶课程讨论。', async (row) => {
            await api.teaching.pinPost(row.id)
            return '讨论已置顶'
          }),
          rowAction('delete-post', '删除', '删除课程讨论。', async (row) => {
            await api.teaching.deletePost(row.id)
            return '讨论已删除'
          }),
        ],
      }
    }, '教学'),
    hiddenResourceRoute('assignments', '作业管理', '管理课程作业、提交和批改入口', ClipboardCheck, async (api, params) => {
      const assignmentId = routeParam(params, 'assignment_id')
      return assignmentId
        ? objectResult(await api.teaching.getAssignment(assignmentId), assignmentColumns(), '作业详情')
        : emptyResult(assignmentColumns(), '请选择课程作业', '从课程详情进入作业管理。')
    }),
    hiddenResourceRoute('assignment-edit', '作业编辑', '创建、更新和发布课程作业', FilePenLine, async (api) => ({
      ...emptyResult(assignmentColumns(), '作业编辑', '填写表单后保存作业。'),
      actions: assignmentActions(api),
    })),
    resourceRoute('grading', '批改中心', '查看提交、人工评分和查重线索', ShieldCheck, async (api, params) => {
      const assignmentId = routeParam(params, 'assignment_id', 'aid')
      return {
        ...teacherMetrics(assignmentId ? listResult(await api.teaching.getSubmissions(assignmentId, defaultPageParams()), submissionColumns(), '暂无提交', '学生提交作业后会显示。') : listResult(await api.judge.getTasks(defaultPageParams()), judgeTaskColumns(), '暂无批改任务', '作业提交或判题任务会显示。'), '待处理项', '人工评分', '重判进度'),
        actions: [
          pageAction('grade-submission', '人工批改', '为提交记录写入分数和评语。', [
            textInput('submission_id', '提交编号', true),
            numberInput('score', '分数', true),
            textareaInput('comment', '评语', true),
          ], async (values) => {
            await api.teaching.gradeSubmission(valueText(values, 'submission_id'), {
              score: valueNumber(values, 'score'),
              comment: valueText(values, 'comment'),
            })
            return '人工批改已保存'
          }),
          pageAction('manual-score', '人工评分', '为需要人工评分的判题任务提交分数和评语。', [
            textInput('task_id', '判题任务编号', true),
            numberInput('score', '得分', true),
            numberInput('max_score', '满分', true),
            numberInput('passed', '是否通过', true, '1 表示通过，0 表示未通过。'),
            textareaInput('comment', '评语', true),
          ], async (values) => {
            await api.judge.manualScore(valueText(values, 'task_id'), {
              score: valueNumber(values, 'score'),
              max_score: valueNumber(values, 'max_score'),
              passed: valueFlag(values, 'passed'),
              comment: valueText(values, 'comment'),
            })
            return '人工评分已提交'
          }),
          pageAction('read-judge-task', '查看判题任务', '按任务编号读取判题进度和结果。', [textInput('task_id', '判题任务编号', true)], async (values) => {
            await api.judge.getTask(valueText(values, 'task_id'))
            return '判题任务已读取'
          }),
          pageAction('prepare-judge-progress', '准备判题进度', '准备指定判题任务的实时进度信息。', [textInput('task_id', '判题任务编号', true)], async (values) => {
            api.judge.getProgressWsUrl(valueText(values, 'task_id'))
            return '判题进度已准备'
          }),
        ],
        rowActions: [
          rowAction('rejudge-task', '重判', '按原始快照触发重判。', async (row) => {
            await api.judge.rejudgeTask(row.id)
            return '重判任务已提交'
          }),
        ],
      }
    }, '教学'),
    hiddenResourceRoute('exp-wizard', '实验编排向导', '分步配置实验组件、协作、检查点和发布校验', Layers, async (api) => ({
      ...listResult(await api.experiment.getExperiments(defaultPageParams()), experimentColumns(), '暂无实验草稿', '创建实验后会显示编排进度。'),
      actions: experimentAuthorActions(api),
    })),
    hiddenResourceRoute('contest-edit', '竞赛配置', '创建或更新竞赛基础配置和赛制规则', Swords, async (api) => ({
      ...listResult(await api.contest.getContests(defaultPageParams()), contestColumns(), '暂无竞赛', '创建竞赛后会显示。'),
      actions: contestManagementActions(api),
    })),
    hiddenResourceRoute('contest-problems', '竞赛出题', '维护竞赛题目、分值和对抗配置', ListChecks, async (api, params) => {
      const contestId = routeParam(params, 'contest_id', 'id')
      return {
        ...(contestId ? arrayResult(await api.contest.getProblems(contestId), contestProblemColumns(), '暂无赛题', '添加赛题后会显示。') : emptyResult(contestProblemColumns(), '请选择竞赛', '从竞赛配置进入出题页面。')),
        actions: contestManagementActions(api),
      }
    }),
    resourceRoute('monitor', '实时监控', '查看实验运行、竞赛对抗和异常学生状态', MonitorCog, async (api) => ({
      ...teacherMetrics(listResult(await api.judge.getTasks(defaultPageParams()), judgeTaskColumns(), '暂无运行任务', '实验或竞赛运行后会显示实时状态。'), '运行任务', '学习统计', '异常处理'),
      actions: [
        pageAction('read-progress-stats', '读取学习统计', '按课程编号读取学习进度统计。', [textInput('course_id', '课程编号', true)], async (values) => {
          await api.teaching.getProgressStats(valueText(values, 'course_id'))
          return '学习统计已读取'
        }),
      ],
    }), '实践'),
    hiddenResourceRoute('cheat-review', '防作弊审查', '查看可疑提交并形成处理记录', ShieldAlert, async (api, params) => {
      const contestId = routeParam(params, 'contest_id')
      return {
        ...(contestId ? listResult(await api.contest.listCheatRecords(contestId, defaultPageParams()), cheatColumns(), '暂无违规记录', '处理防作弊线索后会显示记录。') : emptyResult(cheatColumns(), '请选择竞赛', '从实时监控进入具体竞赛后处理防作弊线索。')),
        actions: [
          pageAction('create-cheat-record', '登记处理记录', '记录防作弊处理结论。', [
            textInput('contest_id', '竞赛编号', true),
            numberInput('team_id', '队伍编号', true),
            numberInput('type', '类型', true),
            textareaInput('evidence', '证据内容', true),
            numberInput('action', '处理动作', true),
          ], async (values) => {
            await api.contest.createCheatRecord(valueText(values, 'contest_id'), {
              team_id: valueNumber(values, 'team_id'),
              type: valueNumber(values, 'type'),
              evidence: valueJson(values, 'evidence'),
              action: valueNumber(values, 'action'),
            })
            return '防作弊处理记录已保存'
          }),
        ],
      }
    }),
    hiddenResourceRoute('vuln-sources', '漏洞源管理', '维护漏洞来源、同步和素材入库', Flag, async (api) => ({
      ...arrayResult(await api.contest.listVulnSources(), vulnSourceColumns(), '暂无漏洞源', '接入漏洞源后会显示。'),
      actions: vulnSourceActions(api),
    })),
    hiddenResourceRoute('vuln-transform', '漏洞题转化', '把漏洞素材转化为可判题内容并完成预验证', ShieldCheck, async (api) => ({
      ...listResult(await api.contest.listVulnProblems(defaultPageParams()), vulnProblemColumns(), '暂无漏洞题草稿', '导入漏洞素材后会显示。'),
      actions: vulnProblemActions(api),
    })),
    hiddenResourceRoute('content-edit', '内容编辑', '创建题库内容、版本和附件引用', FilePenLine, async (api) => ({
      ...listResult(await api.content.getItems(defaultPageParams()), contentColumns(), '暂无内容', '创建内容后会显示。'),
      actions: contentActions(api),
    })),
    hiddenResourceRoute('paper-edit', '组卷编辑', '创建试卷并维护题目列表', FileText, async (api, params) => {
      const paperId = routeParam(params, 'paper_id', 'id')
      return {
        ...(paperId
          ? objectResult(await api.content.getPaper(paperId), paperColumns(), '试卷详情')
          : listResult(await api.content.listPapers(defaultPageParams()), paperColumns(), '暂无试卷', '创建试卷后会显示。')),
        actions: paperActions(api),
      }
    }),
    resourceRoute('sim-packages', '仿真场景', '上传、预览和提交仿真包审核', Network, async (api) => ({
      ...teacherMetrics(listResult(await api.sim.getPackages(defaultPageParams()), simPackageColumns(), '暂无仿真包', '上传仿真包后会显示。'), '仿真包', '预览校验', '提交审核'),
      actions: simPackageActions(api),
      rowActions: [
        rowAction('open-sim-workspace', '预览工作台', '进入仿真工作台查看包状态。', async (row) => {
          window.location.hash = routeHref('sim-workspace', { code: valueFromRow(row, 'code'), version: valueFromRow(row, 'version') }).slice(1)
          return '正在进入仿真工作台'
        }),
      ],
    }), '资源'),
    resourceRoute('shared-lib', '共享资源库', '查看跨课程共享内容和复用素材', Library, async (api) => teacherMetrics(listResult(await api.content.listShared(defaultPageParams()), contentColumns(), '暂无共享资源', '共享题库内容后会显示。'), '共享资源', '跨课复用', '题库引用'), '资源'),
    hiddenResourceRoute('content-categories', '题库分类', '维护题库分类树和展示顺序', Library, async (api) => ({
      ...arrayResult(await api.content.listCategories(), contentCategoryColumns(), '暂无分类', '创建分类后会显示。'),
      actions: [
        pageAction('update-category', '更新分类', '更新题库分类名称和顺序。', [
          textInput('category_id', '分类编号', true),
          numberInput('parent_id', '上级分类编号', true),
          textInput('name', '分类名称', true),
          numberInput('sort', '顺序', true),
        ], async (values) => {
          await api.content.updateCategory(valueText(values, 'category_id'), {
            parent_id: valueNumber(values, 'parent_id'),
            name: valueText(values, 'name'),
            sort: valueNumber(values, 'sort'),
          })
          return '分类已更新'
        }),
      ],
      rowActions: [
        rowAction('delete-category', '删除', '删除未被引用的分类。', async (row) => {
          await api.content.deleteCategory(row.id)
          return '分类已删除'
        }),
      ],
    }), '资源'),
    resourceRoute('grade-submit', '成绩报送', '计算课程成绩并提交学校审核', ScrollText, async (api) => ({
      ...teacherMetrics(listResult(await api.grade.listReviews(defaultPageParams()), gradeReviewColumns(), '暂无报送记录', '提交成绩审核后会显示。'), '报送记录', '学校审核', '成绩锁定'),
      actions: [
        navigatePageAction('open-grade-appeals-page', '成绩申诉', '进入学生成绩申诉处理页面。', 'grade-appeals'),
        pageAction('submit-grade-review', '提交成绩审核', '将课程成绩提交学校管理员审核锁定。', [
          textInput('course_id', '课程编号', true),
          textInput('semester_id', '学期编号'),
          textareaInput('comment', '报送说明'),
        ], async (values) => {
          await api.grade.submitReview({
            course_id: valueText(values, 'course_id'),
            semester_id: optionalText(values, 'semester_id'),
            comment: optionalText(values, 'comment'),
          })
          return '成绩审核已提交'
        }),
      ],
    }), '成绩'),
    hiddenResourceRoute('course-grades', '课程成绩', '维护课程成绩权重、计算结果和成绩导出', ScrollText, async (api, params) => {
      const courseId = routeParam(params, 'course_id', 'id')
      return {
        ...(courseId
          ? listResult(await api.teaching.listGrades(courseId, defaultPageParams()), teachingGradeColumns(), '暂无课程成绩', '计算或调整成绩后会显示。')
          : emptyResult(teachingGradeColumns(), '请选择课程', '从成绩报送或课程管理进入具体课程后维护成绩。')),
        actions: [
          pageAction('list-course-grades', '读取课程成绩', '读取课程成绩列表。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.listGrades(valueText(values, 'course_id'), defaultPageParams())
            return '课程成绩已读取'
          }),
          pageAction('compute-course-grades', '计算课程成绩', '按当前成绩权重重新计算课程成绩。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.computeGrades(valueText(values, 'course_id'))
            return '课程成绩已计算'
          }),
          pageAction('set-grade-weights', '设置成绩权重', '设置课程成绩来源和权重。', [
            textInput('course_id', '课程编号', true),
            textareaInput('items', '权重列表', true),
          ], async (values) => {
            await api.teaching.setGradeWeights(valueText(values, 'course_id'), {
              items: valueJsonArray(values, 'items') as never,
            })
            return '成绩权重已保存'
          }),
          pageAction('list-grade-weights', '读取成绩权重', '读取课程成绩权重配置。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.listGradeWeights(valueText(values, 'course_id'))
            return '成绩权重已读取'
          }),
          pageAction('override-course-grade', '调整课程成绩', '按学生编号调整课程最终成绩。', [
            textInput('course_id', '课程编号', true),
            textInput('student_id', '学生编号', true),
            numberInput('total', '调整后总分', true),
          ], async (values) => {
            await api.teaching.overrideGrade(valueText(values, 'course_id'), valueText(values, 'student_id'), { total: valueNumber(values, 'total') })
            return '课程成绩已调整'
          }),
          pageAction('export-course-grades', '导出课程成绩', '创建课程成绩导出任务，下载走任务与下载页。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.exportGrades(valueText(values, 'course_id'))
            return '成绩导出任务已创建'
          }),
        ],
      }
    }, '成绩'),
    resourceRoute('org', '组织查看', '查看学校组织结构，教师侧只读', Building2, async (api) => teacherMetrics(arrayResult(await api.identity.listDepartments(), orgColumns(), '暂无组织', '学校管理员维护组织后会显示。'), '组织节点', '只读查看', '课程成员'), '组织'),
  ]
}

/**
 * teacherMetrics 为教师主任务页提供生产状态指标，数值只来自当前资源结果。
 */
function teacherMetrics(result: ResourceResult, primaryLabel: string, processLabel: string, actionLabel: string): ResourceResult {
  const metrics: MetricItem[] = [
    { label: primaryLabel, value: String(result.rows.length), tone: 'primary' },
    { label: processLabel, value: result.rows.length > 0 ? '可继续处理' : '等待创建', tone: result.rows.length > 0 ? 'success' : 'secondary' },
    { label: '下一步', value: actionLabel, tone: 'warning' },
  ]
  return { ...result, metrics }
}
