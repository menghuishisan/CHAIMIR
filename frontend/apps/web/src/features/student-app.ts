// 学生端路由：课程、实验、竞赛、仿真、成绩与账户页面定义。

import { Activity, Award, BookOpen, FileCheck2, FileClock, FilePenLine, FileText, Flag, Gavel, GraduationCap, Network, ShieldAlert, TerminalSquare, Trophy, UserCog } from 'lucide-react'
import { ExperimentStatus } from '@chaimir/api-client'
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
  defaultPageParams,
  emptyResult,
  experimentColumns,
  gradeSummaryColumns,
  hiddenResourceRoute,
  lessonColumns,
  listResult,
  numberInput,
  optionalNumber,
  objectResult,
  optionalNumberPayload,
  optionalText,
  outlineColumns,
  pageAction,
  reportColumns,
  resourceRoute,
  routeHref,
  routeParam,
  rowAction,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  sharedTransferRoute,
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
  valueStringArray,
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
      group: '学习',
      load: async (api) => ({
        ...listResult(await api.teaching.getCourses({ role: 'student', ...defaultPageParams() }), courseColumns(), '暂无课程', '加入课程后会在这里显示学习安排。'),
        actions: [
          pageAction('join-course', '加入课程', '输入教师提供的邀请码加入课程，加入状态会自动保存。', [textInput('invite_code', '课程邀请码', true)], async (values) => {
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
      group: '学习',
      load: async (api) => ({
        ...listResult(await api.experiment.getExperiments({ status: ExperimentStatus.PUBLISHED, ...defaultPageParams() }), experimentColumns(), '暂无实验', '课程发布实验后会在这里显示。'),
        actions: [
          pageAction('start-experiment', '创建实验实例', '输入实验编号创建个人或小组实例，平台会准备所需实验资源。', [
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
      group: '学习',
      load: async (api) => ({
        ...listResult(await api.contest.getContests(defaultPageParams()), contestColumns(), '暂无竞赛', '有可报名或进行中的竞赛时会在这里显示。'),
        actions: [
          pageAction('contest-signup', '报名竞赛', '输入竞赛编号和队伍名称完成报名，队伍状态会自动保存。', [
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
      group: '学习',
      load: async (api) => arrayResult(await api.contest.getMyContestRecords(), contestRecordColumns(), '暂无竞赛战绩', '完成竞赛后会在这里显示成绩。'),
    },
    {
      path: 'grades',
      label: '成绩中心',
      description: '查看个人课程成绩、绩点和学业预警',
      icon: GraduationCap,
      group: '成绩',
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
            pageAction('generate-transcript', '生成成绩单', '生成个人成绩单记录，并获取下载授权。', [
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
      group: '成绩',
      load: async (api) => ({
        ...listResult(await api.grade.listWarnings(defaultPageParams()), warningColumns(), '暂无学业预警', '有需要关注的学业状态时会在这里显示。'),
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
    sharedTransferRoute(),
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
          { label: '进度推送', value: '实时同步', tone: 'secondary' },
          { label: '安全边界', value: '不在前端暴露答案', tone: 'success' },
        ])
      }
      const instance = await api.experiment.getInstance(instanceId)
      const primarySandbox = instance.sandboxes[0]
      return workspaceInfo('实验工作台', `实例 ${instance.instance_id} 的实验资源状态。`, [
        { label: '沙箱数量', value: String(instance.sandboxes.length), tone: 'primary' },
        { label: '仿真会话', value: String(instance.sims.length), tone: 'secondary' },
        { label: '当前得分', value: String(instance.score), tone: 'success' },
      ], [
        ...instance.sandboxes.flatMap((sandbox) => [
          {
            key: `terminal-${sandbox.sandbox_id}`,
            label: `${sandbox.runtime_code} 终端`,
            description: '打开当前实验沙箱终端。',
            kind: 'terminal' as const,
            href: api.sandbox.getTerminalWsUrl(sandbox.sandbox_id),
          },
          {
            key: `progress-${sandbox.sandbox_id}`,
            label: `${sandbox.runtime_code} 状态`,
            description: '查看当前实验沙箱准备和运行状态。',
            kind: 'status' as const,
            href: api.sandbox.getProgressWsUrl(sandbox.sandbox_id),
          },
          ...sandbox.tools.map((tool) => ({
            key: `${sandbox.sandbox_id}-${tool.code}`,
            label: toolLabel(tool.code, tool.kind),
            description: toolDescription(tool.code, tool.kind),
            kind: toolKind(tool.kind),
            href: tool.kind === 3 ? api.sandbox.getToolProxyUrl(sandbox.sandbox_id, tool.code) : undefined,
          })),
        ]),
        ...instance.sims.map((sim) => ({
          key: `sim-${sim.session_id}`,
          label: `${sim.package_code} 仿真`,
          description: '进入与实验实例关联的仿真工作台。',
          kind: 'sim' as const,
          href: routeHref('sim-workspace', { code: sim.package_code, version: sim.version, session_id: sim.session_id }),
        })),
      ], primarySandbox ? [
        pageAction('save-workspace-files', '保存工作区', '立即持久化当前实验沙箱工作区。', [], async () => {
          await api.sandbox.saveFiles(primarySandbox.sandbox_id)
          return '工作区已保存'
        }),
        pageAction('read-sandbox-detail', '读取沙箱详情', '读取指定实验沙箱资源状态和工具入口。', [textInput('sandbox_id', '沙箱编号', true)], async (values) => {
          await api.sandbox.getInstance(valueText(values, 'sandbox_id'))
          return '沙箱详情已读取'
        }),
        pageAction('list-workspace-files', '查看文件列表', '列出沙箱工作区目录。', [
          textInput('sandbox_id', '沙箱编号', true),
          textInput('path', '目录路径', true),
        ], async (values) => {
          await api.sandbox.listFiles(valueText(values, 'sandbox_id'), valueText(values, 'path'))
          return '文件列表已读取'
        }),
        pageAction('read-workspace-file', '读取文件', '读取沙箱工作区文件内容。', [
          textInput('sandbox_id', '沙箱编号', true),
          textInput('path', '文件路径', true),
        ], async (values) => {
          await api.sandbox.readFile(valueText(values, 'sandbox_id'), valueText(values, 'path'))
          return '文件内容已读取'
        }),
        pageAction('write-workspace-file', '写入文件', '写入沙箱工作区文件，内容使用 Base64。', [
          textInput('sandbox_id', '沙箱编号', true),
          textInput('relative_path', '文件路径', true),
          textareaInput('content_base64', '文件内容', true),
        ], async (values) => {
          await api.sandbox.writeFile(valueText(values, 'sandbox_id'), {
            relative_path: valueText(values, 'relative_path'),
            content_base64: valueText(values, 'content_base64'),
          })
          return '文件已写入'
        }),
        pageAction('run-command-tool', '执行命令工具', '执行实验声明的受控命令工具。', [
          textInput('sandbox_id', '沙箱编号', true),
          textInput('tool_code', '工具编码', true),
          textInput('command', '命令参数', true, '多个参数用英文逗号分隔，例如 forge,test。'),
          numberInput('timeout_sec', '等待秒数'),
        ], async (values) => {
          await api.sandbox.runCommandTool(valueText(values, 'sandbox_id'), valueText(values, 'tool_code'), {
            command: valueStringArray(values, 'command'),
            timeout_sec: optionalNumber(values, 'timeout_sec'),
          })
          return '命令工具已执行'
        }),
        pageAction('chain-query', '查询链上状态', '调用当前沙箱运行时的链查询能力。', [
          textInput('sandbox_id', '沙箱编号', true),
          textInput('target', '查询目标', true),
        ], async (values) => {
          await api.sandbox.chainQuery(valueText(values, 'sandbox_id'), valueText(values, 'target'))
          return '链上状态已查询'
        }),
        pageAction('chain-deploy', '部署链上合约', '调用当前沙箱运行时的部署能力。', [
          textInput('sandbox_id', '沙箱编号', true),
          textareaInput('payload', '部署参数', true),
        ], async (values) => {
          await api.sandbox.chainDeploy(valueText(values, 'sandbox_id'), { payload: valueJson(values, 'payload') })
          return '链上部署已提交'
        }),
        pageAction('chain-send-tx', '发送链上交易', '调用当前沙箱运行时的交易能力。', [
          textInput('sandbox_id', '沙箱编号', true),
          textareaInput('payload', '交易参数', true),
        ], async (values) => {
          await api.sandbox.chainSendTx(valueText(values, 'sandbox_id'), { payload: valueJson(values, 'payload') })
          return '链上交易已提交'
        }),
        pageAction('activate-stage', '激活实验阶段', '按阶段创建后续资源，阶段状态会自动保存。', [
          numberInput('stage', '阶段序号', true),
        ], async (values) => {
          await api.experiment.activateStage(instance.instance_id, valueNumber(values, 'stage'))
          return '实验阶段已激活'
        }),
        pageAction('judge-checkpoint', '判定检查点', '提交检查点判分，答案和判题规则对学生不可见。', [
          textInput('checkpoint_id', '检查点编号', true),
          textareaInput('payload', '判分参数'),
        ], async (values) => {
          await api.experiment.judgeCheckpoint(instance.instance_id, valueText(values, 'checkpoint_id'), valueJson(values, 'payload'))
          return '检查点判分已提交'
        }),
        pageAction('submit-report', '提交实验报告', '提交实验报告引用，报告文件由平台管理。', [
          textareaInput('content_ref', '报告引用', true),
        ], async (values) => {
          await api.experiment.submitReport(instance.instance_id, { content_ref: JSON.stringify(valueJson(values, 'content_ref')) })
          return '实验报告已提交'
        }),
        pageAction('pause-instance', '暂停实验', '暂停当前实验实例。', [], async () => {
          await api.experiment.pauseInstance(instance.instance_id)
          return '实验已暂停'
        }),
        pageAction('resume-instance', '恢复实验', '恢复当前实验实例。', [], async () => {
          await api.experiment.resumeInstance(instance.instance_id)
          return '实验已恢复'
        }),
        pageAction('finish-instance', '完成实验', '结束当前实验实例并归档进度。', [], async () => {
          await api.experiment.finishInstance(instance.instance_id)
          return '实验已完成'
        }),
        pageAction('recycle-instance', '回收实验资源', '回收当前实验实例占用的沙箱和仿真资源。', [], async () => {
          await api.experiment.recycleInstance(instance.instance_id)
          return '实验资源已回收'
        }),
      ] : [])
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
        : listResult(await api.teaching.getCourses({ role: 'student', ...defaultPageParams() }), courseColumns(), '暂无课程', '加入课程后会显示课程详情。')
      return {
        ...result,
        actions: [
          pageAction('read-progress', '读取学习进度', '按课程编号读取本人学习进度。', [textInput('course_id', '课程编号', true)], async (values) => {
            await api.teaching.getMyProgress(valueText(values, 'course_id'))
            return '学习进度已读取'
          }),
          pageAction('review-course', '评价课程', '提交课程学习评价。', [
            textInput('course_id', '课程编号', true),
            numberInput('rating', '评分', true),
            textareaInput('comment', '评价内容', true),
          ], async (values) => {
            await api.teaching.reviewCourse(valueText(values, 'course_id'), {
              rating: valueNumber(values, 'rating'),
              comment: valueText(values, 'comment'),
            })
            return '课程评价已提交'
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
          pageAction('report-progress', '保存学习进度', '保存当前课时学习进度，刷新或换设备后不丢失。', [
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
          pageAction('save-draft', '保存作答草稿', '草稿会自动保存，刷新或换设备后可继续作答。', [
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
          pageAction('read-assignment-draft', '读取作答草稿', '读取已保存的作答草稿。', [textInput('assignment_id', '作业编号', true)], async (values) => {
            await api.teaching.getDraft(valueText(values, 'assignment_id'))
            return '作答草稿已读取'
          }),
        ],
      }
    }),
    hiddenResourceRoute('submission', '作业结果', '查看作业提交、评测任务和教师反馈', FileCheck2, async (api, params) => {
      const submissionId = routeParam(params, 'submission_id', 'id')
      const assignmentId = routeParam(params, 'assignment_id')
      if (submissionId) return objectResult(await api.teaching.getSubmission(submissionId), submissionColumns(), '提交详情')
      if (assignmentId) return listResult(await api.teaching.getSubmissions(assignmentId, defaultPageParams()), submissionColumns(), '暂无提交', '提交作业后会显示结果。')
      return emptyResult(submissionColumns(), '请选择提交记录', '从作业或提交列表进入后会显示结果。')
    }),
    hiddenResourceRoute('experiment-detail', '实验详情', '查看实验组件、协作配置、报告和实例入口', TerminalSquare, async (api, params) => ({
      ...listResult(await api.experiment.listReports(routeParam(params, 'id', 'experiment_id') || '0', defaultPageParams()), reportColumns(), '暂无实验报告', '进入实验并提交报告后会显示记录。'),
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
        : listResult(await api.contest.getContests(defaultPageParams()), contestColumns(), '暂无竞赛', '有可报名或进行中的竞赛时会显示。')
    }),
    hiddenResourceRoute('contest-signup', '竞赛报名', '完成个人或团队报名，队伍状态会自动保存', UserCog, async (api) => ({
      ...listResult(await api.contest.getContests(defaultPageParams()), contestColumns(), '暂无可报名竞赛', '报名开放后会在这里显示。'),
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
    resourceRoute('sim-lib', '仿真实验室', '检索仿真包、读取回放并进入仿真工作台', Network, async (api) => ({
      ...listResult(await api.sim.getPackages({ status: 'published', ...defaultPageParams() }), simPackageColumns(), '暂无仿真包', '有仿真包发布后会显示。'),
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
        rowAction('open-sim', '进入仿真', '打开沉浸式仿真工作台。', async (row) => {
          window.location.hash = routeHref('sim-workspace', { code: valueFromRow(row, 'code'), version: valueFromRow(row, 'version') }).slice(1)
          return '正在进入仿真工作台'
        }),
      ],
    }), '学习'),
    hiddenResourceRoute('my-records', '我的战绩', '查看跨竞赛名次、得分和历史记录', Award, async (api) => arrayResult(await api.contest.getMyContestRecords(), contestRecordColumns(), '暂无竞赛战绩', '完成竞赛后会显示战绩。')),
    hiddenResourceRoute('appeals', '成绩申诉', '提交成绩申诉并查看处理进度', Gavel, async (api) => ({
      ...listResult(await api.grade.listAppeals(defaultPageParams()), appealColumns(), '暂无申诉', '提交申诉后会显示处理进度。'),
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
 * toolKind 把后端沙箱工具类型映射为工作台可理解的展示类型。
 */
function toolKind(kind: number): 'terminal' | 'web' | 'command' | 'file' | 'chain' | 'sim' | 'status' {
  if (kind === 1) return 'file'
  if (kind === 2) return 'terminal'
  if (kind === 3) return 'web'
  if (kind === 4) return 'command'
  return 'status'
}

/**
 * toolLabel 生成用户向工具名称，避免直接展示技术枚举。
 */
function toolLabel(code: string, kind: number): string {
  if (kind === 1) return `${code} 内建工具`
  if (kind === 2) return `${code} 终端`
  if (kind === 3) return `${code} 工具`
  if (kind === 4) return `${code} 命令工具`
  return `${code} 工具`
}

/**
 * toolDescription 说明工具用途和打开方式。
 */
function toolDescription(code: string, kind: number): string {
  if (kind === 1) return '由平台提供文件、状态或日志能力。'
  if (kind === 2) return '连接到实验沙箱终端。'
  if (kind === 3) return '打开沙箱中的 Web 工具。'
  if (kind === 4) return '在右侧操作区执行受控命令。'
  return `查看 ${code} 的可用状态。`
}
