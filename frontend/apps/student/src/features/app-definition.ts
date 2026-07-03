// 学生端路由：课程、实验、竞赛、仿真、成绩与账户页面定义。

import { Activity, Award, BookOpen, FileCheck2, FileClock, FilePenLine, FileText, Flag, Gavel, GraduationCap, Network, ShieldAlert, TerminalSquare, Trophy, UserCog } from 'lucide-react'
import type { AppDefinition } from '@chaimir/shared'
import {
  appealColumns,
  arrayResult,
  assignmentColumns,
  battleReplayRoute,
  contestColumns,
  contestProblemColumns,
  contestRecordColumns,
  courseColumns,
  emptyResult,
  experimentColumns,
  gradeSummaryColumns,
  hiddenResourceRoute,
  lessonColumns,
  listResult,
  numberInput,
  objectResult,
  optionalNumberPayload,
  optionalText,
  outlineColumns,
  pageAction,
  reportColumns,
  routeHref,
  routeParam,
  rowAction,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  simPackageColumns,
  simWorkspaceRoute,
  solveWorkspaceRoute,
  submissionColumns,
  textInput,
  textareaInput,
  transcriptColumns,
  valueFromRow,
  valueJson,
  valueNumber,
  valueText,
  warningColumns,
  workspaceInfo,
} from '@chaimir/shared'

export const studentApp: AppDefinition = {
  role: 'student',
  title: '学生端',
  subtitle: '课程学习、实验实训、竞赛参赛与个人成绩',
  homePath: 'courses',
  routes: [
    {
      path: 'courses',
      label: '我的课程',
      description: '查看已加入课程、学习进度和课程时间安排',
      icon: BookOpen,
      load: async (api) => ({
        ...listResult(await api.teaching.getCourses({ role: 'student', page: 1, size: 20 }), courseColumns(), '暂无课程', '加入课程后会在这里显示学习安排。'),
        actions: [
          pageAction('join-course', '加入课程', '输入教师提供的邀请码加入课程，加入状态由服务端保存。', [textInput('invite_code', '课程邀请码', true)], async (values) => {
            await api.teaching.joinCourse({ invite_code: valueText(values, 'invite_code') })
            return '已提交加入课程请求'
          }),
        ],
        rowActions: [
          rowAction('course-outline', '读取大纲', '读取课程章节、课时和学习进度。', async (row) => {
            await api.teaching.getCourseOutline(row.id)
            return '课程大纲已读取'
          }),
        ],
      }),
    },
    {
      path: 'experiments',
      label: '实验实训',
      description: '进入链上实验、查看实验配置和完成进度',
      icon: TerminalSquare,
      load: async (api) => ({
        ...listResult(await api.experiment.getExperiments({ status: 1, page: 1, size: 20 }), experimentColumns(), '暂无实验', '课程发布实验后会在这里显示。'),
        actions: [
          pageAction('start-experiment', '创建实验实例', '输入实验编号创建个人或小组实例，实例资源由后端编排。', [
            textInput('experiment_id', '实验编号', true),
            numberInput('group_id', '小组编号'),
          ], async (values) => {
            const instance = await api.experiment.createInstance(valueText(values, 'experiment_id'), optionalNumberPayload(values, 'group_id'))
            window.location.hash = routeHref('experiment-workspace', { instance_id: instance.instance_id }).slice(1)
            return '实验实例已创建'
          }),
        ],
        rowActions: [
          rowAction('start-row-experiment', '进入实验', '为当前实验创建实例并进入工作台。', async (row) => {
            const instance = await api.experiment.createInstance(row.id, {})
            window.location.hash = routeHref('experiment-workspace', { instance_id: instance.instance_id }).slice(1)
            return '实验实例已创建'
          }),
        ],
      }),
    },
    {
      path: 'contests',
      label: '竞赛参赛',
      description: '报名赛事、查看赛程和个人竞赛记录',
      icon: Trophy,
      load: async (api) => ({
        ...listResult(await api.contest.getContests({ page: 1, size: 20 }), contestColumns(), '暂无竞赛', '有可报名或进行中的竞赛时会在这里显示。'),
        actions: [
          pageAction('contest-signup', '报名竞赛', '输入竞赛编号和队伍名称完成报名，队伍状态由后端保存。', [
            textInput('contest_id', '竞赛编号', true),
            textInput('team_name', '队伍名称', true),
          ], async (values) => {
            await api.contest.signup(valueText(values, 'contest_id'), { team_name: valueText(values, 'team_name') })
            return '竞赛报名已提交'
          }),
          pageAction('join-team', '加入队伍', '输入队伍编号和邀请码加入已有队伍。', [
            textInput('team_id', '队伍编号', true),
            textInput('invite_code', '队伍邀请码', true),
          ], async (values) => {
            await api.contest.joinTeam(valueText(values, 'team_id'), { invite_code: valueText(values, 'invite_code') })
            return '已提交加入队伍请求'
          }),
        ],
      }),
    },
    {
      path: 'contest-records',
      label: '竞赛战绩',
      description: '查看个人历史竞赛成绩和排名',
      icon: Flag,
      load: async (api) => arrayResult(await api.contest.getMyContestRecords(), contestRecordColumns(), '暂无竞赛战绩', '完成竞赛后会在这里显示成绩。'),
    },
    {
      path: 'simulation',
      label: '仿真库',
      description: '使用已发布仿真包进行协议推演和复盘',
      icon: Network,
      load: async (api) => ({
        ...listResult(await api.sim.getPackages({ status: 'published', page: 1, size: 20 }), simPackageColumns(), '暂无仿真包', '教师或管理员发布仿真包后会在这里显示。'),
        actions: [
          pageAction('read-shared-replay', '读取分享回放', '输入分享码读取可复现实验回放。', [textInput('code', '分享码', true)], async (values) => {
            await api.sim.getSharedReplay(valueText(values, 'code'))
            return '分享回放已读取'
          }),
        ],
        rowActions: [
          rowAction('package-versions', '查看版本', '读取该仿真包的可用版本。', async (row) => {
            await api.sim.getPackageVersions(valueFromRow(row, 'code'))
            return '仿真包版本已读取'
          }),
        ],
      }),
    },
    {
      path: 'grades',
      label: '成绩中心',
      description: '查看个人课程成绩、绩点和学业预警',
      icon: GraduationCap,
      load: async (api) => {
        const me = await api.identity.getMe()
        return {
          ...arrayResult(await api.grade.studentGPA(me.id), gradeSummaryColumns(), '暂无成绩', '课程成绩完成归档后会在这里显示。'),
          actions: [
            pageAction('submit-grade-appeal', '提交成绩申诉', '对课程成绩有疑问时提交申诉，处理结果由教师或管理员反馈。', [
              textInput('course_id', '课程编号', true),
              textareaInput('reason', '申诉说明', true),
            ], async (values) => {
              await api.grade.submitAppeal({ course_id: valueText(values, 'course_id'), reason: valueText(values, 'reason') })
              return '成绩申诉已提交'
            }),
            pageAction('generate-transcript', '生成成绩单', '生成个人成绩单记录，文件下载授权由后端签发。', [
              numberInput('scope', '成绩单范围', true),
              textInput('semester_id', '学期编号'),
            ], async (values) => {
              const transcript = await api.grade.generateTranscript({
                scope: valueNumber(values, 'scope'),
                semester_id: optionalText(values, 'semester_id'),
              })
              await api.grade.downloadTranscript(transcript.id)
              return '成绩单已生成'
            }),
          ],
        }
      },
    },
    {
      path: 'warnings',
      label: '学业预警',
      description: '查看并确认本人学业预警',
      icon: ShieldAlert,
      load: async (api) => ({
        ...listResult(await api.grade.listWarnings({ page: 1, size: 20 }), warningColumns(), '暂无学业预警', '有需要关注的学业状态时会在这里显示。'),
        rowActions: [
          rowAction('ack-warning', '确认预警', '确认已知晓这条学业预警。', async (row) => {
            await api.grade.ackWarning(row.id)
            return '已确认学业预警'
          }),
        ],
      }),
    },
    ...studentDeepRoutes(),
    sharedNotificationRoute(),
    sharedAnnouncementRoute(),
    sharedProfileRoute(),
    studentWorkspaceRoute(),
    simWorkspaceRoute(),
    solveWorkspaceRoute(),
    battleReplayRoute(),
  ],
}


function studentWorkspaceRoute(): AppDefinition['routes'][number] {
  return {
    path: 'experiment-workspace',
    label: '实验工作台',
    description: '沉浸式查看实验实例、检查点和资源状态',
    icon: Activity,
    immersive: true,
    load: async (api, params) => {
      const instanceId = params.get('instance_id')
      if (!instanceId) {
        return workspaceInfo('实验工作台', '从实验列表进入具体实例后，可在这里查看沙箱、仿真和检查点状态。', [
          { label: '资源创建', value: '由实验实例触发', tone: 'primary' },
          { label: '进度推送', value: '使用后端订阅信息', tone: 'secondary' },
          { label: '安全边界', value: '不在前端暴露答案', tone: 'success' },
        ])
      }
      const instance = await api.experiment.getInstance(instanceId)
      return workspaceInfo('实验工作台', `实例 ${instance.instance_id} 的实验资源状态。`, [
        { label: '沙箱数量', value: String(instance.sandboxes.length), tone: 'primary' },
        { label: '仿真会话', value: String(instance.sims.length), tone: 'secondary' },
        { label: '当前得分', value: String(instance.score), tone: 'success' },
      ])
    },
  }
}

/**
 * studentDeepRoutes 补齐学生端文档要求的详情、作答、报名、战绩和沉浸入口。
 */
function studentDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('course-detail', '课程详情', '查看课程大纲、学习进度、讨论和课程作业', BookOpen, async (api, params) => {
      const courseId = routeParam(params, 'id', 'course_id')
      const result = courseId
        ? objectResult(await api.teaching.getCourseOutline(courseId), outlineColumns(), '课程大纲')
        : listResult(await api.teaching.getCourses({ role: 'student', page: 1, size: 20 }), courseColumns(), '暂无课程', '加入课程后会显示课程详情。')
      return {
        ...result,
        actions: [
          pageAction('read-progress', '读取学习进度', '按课程编号读取本人学习进度。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.getMyProgress(valueText(values, 'course_id'))
            return '学习进度已读取'
          }),
        ],
      }
    }),
    hiddenResourceRoute('lesson', '课时学习', '查看课时内容并上报学习进度', FileText, async (api, params) => {
      const lessonId = routeParam(params, 'lesson_id', 'id')
      const result = lessonId
        ? objectResult(await api.teaching.getLesson(lessonId), lessonColumns(), '课时内容')
        : emptyResult(lessonColumns(), '请选择课时', '从课程详情进入具体课时后会显示学习内容。')
      return {
        ...result,
        actions: [
          pageAction('report-progress', '保存学习进度', '向服务端保存当前课时学习进度，刷新或换设备后不丢失。', [
            textInput('lesson_id', '课时编号', true),
            numberInput('video_pos', '学习位置秒数', true),
            numberInput('duration_sec', '总时长秒数', true),
            numberInput('status', '学习状态', true),
          ], async (values) => {
            await api.teaching.reportProgress(valueText(values, 'lesson_id'), {
              status: valueNumber(values, 'status'),
              video_pos: valueNumber(values, 'video_pos'),
              duration_sec: valueNumber(values, 'duration_sec'),
            })
            return '学习进度已保存'
          }),
        ],
      }
    }),
    hiddenResourceRoute('assignment', '作业作答', '查看作业题目、保存草稿并提交作答', FilePenLine, async (api, params) => {
      const assignmentId = routeParam(params, 'assignment_id', 'id')
      const result = assignmentId
        ? objectResult(await api.teaching.getAssignment(assignmentId), assignmentColumns(), '作业详情')
        : emptyResult(assignmentColumns(), '请选择作业', '从课程详情进入作业后可保存草稿并提交。')
      return {
        ...result,
        actions: [
          pageAction('save-draft', '保存作答草稿', '草稿保存到服务端，刷新或换设备后可继续作答。', [
            textInput('assignment_id', '作业编号', true),
            textareaInput('content', '作答内容', true),
          ], async (values) => {
            await api.teaching.saveDraft(valueText(values, 'assignment_id'), { content: valueJson(values, 'content') })
            return '作答草稿已保存'
          }),
          pageAction('submit-assignment', '提交作业', '提交后进入评测或教师批改流程。', [
            textInput('assignment_id', '作业编号', true),
            textareaInput('content_ref', '作答引用', true),
          ], async (values) => {
            await api.teaching.submitAssignment(valueText(values, 'assignment_id'), { content_ref: valueJson(values, 'content_ref') })
            return '作业已提交'
          }),
        ],
      }
    }),
    hiddenResourceRoute('submission', '作业结果', '查看作业提交、评测任务和教师反馈', FileCheck2, async (api, params) => {
      const submissionId = routeParam(params, 'submission_id', 'id')
      const assignmentId = routeParam(params, 'assignment_id')
      if (submissionId) return objectResult(await api.teaching.getSubmission(submissionId), submissionColumns(), '提交详情')
      if (assignmentId) return listResult(await api.teaching.getSubmissions(assignmentId, { page: 1, size: 20 }), submissionColumns(), '暂无提交', '提交作业后会显示结果。')
      return emptyResult(submissionColumns(), '请选择提交记录', '从作业或提交列表进入后会显示结果。')
    }),
    hiddenResourceRoute('experiment-detail', '实验详情', '查看实验组件、协作配置、报告和实例入口', TerminalSquare, async (api, params) => ({
      ...listResult(await api.experiment.listReports(routeParam(params, 'id', 'experiment_id') || '0', { page: 1, size: 20 }), reportColumns(), '暂无实验报告', '进入实验并提交报告后会显示记录。'),
      actions: [
        pageAction('create-instance', '进入实验工作台', '创建个人或小组实验实例并进入沉浸工作台。', [
          textInput('experiment_id', '实验编号', true),
          numberInput('group_id', '小组编号'),
        ], async (values) => {
          const instance = await api.experiment.createInstance(valueText(values, 'experiment_id'), optionalNumberPayload(values, 'group_id'))
          window.location.hash = routeHref('experiment-workspace', { instance_id: instance.instance_id }).slice(1)
          return '实验实例已创建'
        }),
      ],
    })),
    hiddenResourceRoute('contest-detail', '竞赛详情', '查看赛题、榜单、赛程和结果快照', Trophy, async (api, params) => {
      const contestId = routeParam(params, 'id', 'contest_id')
      return contestId
        ? arrayResult(await api.contest.getProblems(contestId), contestProblemColumns(), '暂无赛题', '竞赛发布赛题后会显示题目列表。')
        : listResult(await api.contest.getContests({ page: 1, size: 20 }), contestColumns(), '暂无竞赛', '有可报名或进行中的竞赛时会显示。')
    }),
    hiddenResourceRoute('contest-signup', '竞赛报名', '完成个人或团队报名，队伍状态由服务端保存', UserCog, async (api) => ({
      ...listResult(await api.contest.getContests({ page: 1, size: 20 }), contestColumns(), '暂无可报名竞赛', '报名开放后会在这里显示。'),
      actions: [
        pageAction('signup', '报名竞赛', '创建队伍或个人报名记录。', [
          textInput('contest_id', '竞赛编号', true),
          textInput('team_name', '队伍名称', true),
        ], async (values) => {
          await api.contest.signup(valueText(values, 'contest_id'), { team_name: valueText(values, 'team_name') })
          return '竞赛报名已提交'
        }),
        pageAction('lock-team', '锁定队伍', '确认队伍成员后锁定报名信息。', [textInput('team_id', '队伍编号', true)], async (values) => {
          await api.contest.lockTeam(valueText(values, 'team_id'))
          return '队伍已锁定'
        }),
      ],
    })),
    hiddenResourceRoute('sim-lib', '仿真实验室', '检索仿真包、读取回放并进入仿真工作台', Network, async (api) => ({
      ...listResult(await api.sim.getPackages({ status: 'published', page: 1, size: 20 }), simPackageColumns(), '暂无仿真包', '有仿真包发布后会显示。'),
      rowActions: [
        rowAction('open-sim', '进入仿真', '打开沉浸式仿真工作台。', async (row) => {
          window.location.hash = routeHref('sim-workspace', { code: valueFromRow(row, 'code'), version: valueFromRow(row, 'version') }).slice(1)
          return '正在进入仿真工作台'
        }),
      ],
    })),
    hiddenResourceRoute('my-records', '我的战绩', '查看跨竞赛名次、得分和历史记录', Award, async (api) => arrayResult(await api.contest.getMyContestRecords(), contestRecordColumns(), '暂无竞赛战绩', '完成竞赛后会显示战绩。')),
    hiddenResourceRoute('appeals', '成绩申诉', '提交成绩申诉并查看处理进度', Gavel, async (api) => ({
      ...listResult(await api.grade.listAppeals({ page: 1, size: 20 }), appealColumns(), '暂无申诉', '提交申诉后会显示处理进度。'),
      actions: [
        pageAction('submit-appeal', '提交申诉', '对课程成绩有疑问时提交说明。', [
          textInput('course_id', '课程编号', true),
          textareaInput('reason', '申诉说明', true),
        ], async (values) => {
          await api.grade.submitAppeal({ course_id: valueText(values, 'course_id'), reason: valueText(values, 'reason') })
          return '成绩申诉已提交'
        }),
      ],
    })),
    hiddenResourceRoute('transcripts', '成绩单', '生成并获取成绩单下载授权', FileClock, async (api) => ({
      ...emptyResult(transcriptColumns(), '暂无成绩单', '生成成绩单后可获取下载授权。'),
      actions: [
        pageAction('generate-transcript', '生成成绩单', '按范围生成成绩单记录。', [
          numberInput('scope', '成绩单范围', true),
          textInput('semester_id', '学期编号'),
        ], async (values) => {
          const transcript = await api.grade.generateTranscript({
            scope: valueNumber(values, 'scope'),
            semester_id: optionalText(values, 'semester_id'),
          })
          await api.grade.downloadTranscript(transcript.id)
          return '成绩单已生成并获取下载授权'
        }),
      ],
    })),
  ]
}

/**
 * teacherDeepRoutes 补齐教师端课程、作业、实验、竞赛、监控、资源和报送子页。
 */
