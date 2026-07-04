// 共享路由：通知、个人中心、公告和沉浸式工作台入口。

import { Bell, Download, Flag, Megaphone, PlayCircle, Swords, UserCog } from 'lucide-react'
import type { AppDefinition } from '../types'
import { getRefreshToken, saveSession, saveStoredUser } from '../storage'
import { accountStatusText, dateText, idOf, listResult, sessionStatusText, text, toRows, workspaceInfo } from './results'
import { announcementColumns, notificationColumns, sessionColumns, transferTaskColumns } from './columns'
import { defaultPageParams } from './pagination'
import { routeParam } from './support'
import { numberInput, pageAction, passwordInput, rowAction, textInput, textareaInput, valueFlag, valueJson, valueNumber, valueText } from './actions'

export function sharedNotificationRoute(): AppDefinition['routes'][number] {
  return {
    path: 'notifications',
    label: '通知公告',
    description: '查看通知、公告和未读状态',
    icon: Bell,
    hidden: true,
    load: async (api) => ({
      ...listResult(await api.notify.getNotifications(defaultPageParams()), notificationColumns(), '暂无通知', '新的通知和公告会在这里显示。'),
      actions: [
        pageAction('mark-all-read', '全部标记已读', '将当前账号的未读通知全部标记为已读。', [], async () => {
          await api.notify.markAllAsRead()
          return '通知已全部标记为已读'
        }),
        pageAction('update-notify-preference', '更新通知偏好', '按通知类型启用或关闭接收偏好。', [
          textInput('type', '通知类型', true),
          numberInput('enabled', '是否启用', true, '1 表示启用，0 表示关闭'),
        ], async (values) => {
          await api.notify.updatePreference(valueText(values, 'type'), { enabled: valueFlag(values, 'enabled') })
          return '通知偏好已更新'
        }),
        pageAction('read-notify-preferences', '读取通知偏好', '读取当前账号通知接收偏好。', [], async () => {
          await api.notify.getPreferences()
          return '通知偏好已读取'
        }),
        pageAction('prepare-notify-realtime', '准备实时提醒', '准备当前账号的实时提醒通道。', [], async () => {
          api.notify.getWebSocketUrl()
          return '实时提醒已准备'
        }),
      ],
      rowActions: [
        rowAction('mark-read', '标记已读', '将这条通知标记为已读。', async (row) => {
          await api.notify.markAsRead(row.id)
          return '通知已标记为已读'
        }),
        rowAction('delete-notification', '删除', '删除这条站内通知。', async (row) => {
          await api.notify.deleteNotification(row.id)
          return '通知已删除'
        }),
      ],
    }),
  }
}

/**
 * sharedProfileRoute 四端统一经 M1 当前用户接口读取个人信息。
 */
export function sharedProfileRoute(): AppDefinition['routes'][number] {
  return {
    path: 'profile',
    label: '个人中心',
    description: '查看个人资料和当前登录会话',
    icon: UserCog,
    group: '账户',
    load: async (api) => {
      const me = await api.identity.getMe()
      const sessions = await api.identity.getSessions()
      return {
        metrics: [
          { label: '姓名', value: text(me.account.name), tone: 'primary' },
          { label: '账号状态', value: accountStatusText(me.account.status), tone: 'success' },
          { label: '登录会话', value: String(sessions.length), tone: 'secondary' },
        ],
        columns: sessionColumns(),
        rows: toRows(sessions, (item, index) => ({
          id: idOf(item, index),
          device_info: text(item.device_info || '当前设备'),
          ip: text(item.ip || '未记录'),
          status: sessionStatusText(item.status),
          expire_at: dateText(item.expire_at),
          created_at: dateText(item.created_at),
        })),
        emptyTitle: '暂无会话',
        emptyDescription: '登录会话会在这里显示。',
        actions: [
          pageAction('change-password', '修改密码', '修改当前账号密码，提交后更新登录凭据。', [
            passwordInput('old_password', '当前密码', true),
            passwordInput('new_password', '新密码', true),
          ], async (values) => {
            await api.identity.changePassword({
              old_password: valueText(values, 'old_password'),
              new_password: valueText(values, 'new_password'),
            })
            return '密码已修改'
          }),
          pageAction('change-phone', '修改手机号', '使用验证码修改当前账号手机号。', [
            textInput('phone', '新手机号', true),
            textInput('code', '验证码', true),
          ], async (values) => {
            await api.identity.changePhone({
              phone: valueText(values, 'phone'),
              code: valueText(values, 'code'),
            })
            return '手机号已修改'
          }),
          pageAction('refresh-session', '刷新登录状态', '使用刷新令牌延长当前浏览器登录态。', [], async () => {
            const refreshToken = getRefreshToken()
            if (!refreshToken) {
              throw new Error('登录状态已失效，请重新登录')
            }
            const response = await api.identity.refreshToken({ refresh_token: refreshToken })
            saveSession(response.access_token, response.refresh_token)
            saveStoredUser(response.account)
            return '登录状态已刷新'
          }),
        ],
      }
    },
  }
}

/**
 * studentWorkspaceRoute 通过实例编号读取真实实验实例，不自动创建沙箱资源。
 */

export function sharedAnnouncementRoute(): AppDefinition['routes'][number] {
  return {
    path: 'announcements',
    label: '系统公告',
    description: '查看平台或学校发布的系统公告',
    icon: Megaphone,
    hidden: true,
    load: async (api) => ({
      ...listResult(await api.notify.getAnnouncements(defaultPageParams()), announcementColumns(), '暂无公告', '有公告发布后会在这里显示。'),
      actions: [
        pageAction('create-announcement', '发布系统公告', '发布平台或学校公告，发布范围按当前账号权限生效。', [
          textInput('title', '公告标题', true),
          textareaInput('content', '公告内容', true),
          numberInput('scope', '公告范围', true),
          textInput('target_roles', '目标角色', true, '多个角色编号用英文逗号分隔。'),
        ], async (values) => {
          await api.notify.createAnnouncement({
            title: valueText(values, 'title'),
            content: valueText(values, 'content'),
            scope: valueNumber(values, 'scope'),
            target_roles: valueText(values, 'target_roles').split(',').map((item) => Number(item.trim())).filter(Number.isFinite),
          })
          return '系统公告已发布'
        }),
      ],
      rowActions: [
        rowAction('read-announcement', '标记已读', '将公告标记为已读。', async (row) => {
          await api.notify.markAnnouncementRead(row.id)
          return '公告已标记为已读'
        }),
      ],
    }),
  }
}

/**
 * simWorkspaceRoute 渲染仿真沉浸工作台入口。
 */
export function simWorkspaceRoute(): AppDefinition['routes'][number] {
  return {
    path: 'sim-workspace',
    label: '仿真工作台',
    description: '沉浸式运行协议仿真、单步播放和回放复盘',
    icon: PlayCircle,
    immersive: true,
    hidden: true,
    load: async (api, params) => {
      const code = routeParam(params, 'code')
      const version = routeParam(params, 'version')
      const sessionId = routeParam(params, 'session_id')
      if (code && version) {
        await api.sim.getBundleGrant(code, version)
      }
      return workspaceInfo('仿真工作台', '仿真包加载授权由平台签发，页面只运行已发布或已审核通过的仿真包。', [
        { label: '播放控制', value: '单步与回放', tone: 'primary' },
        { label: '状态来源', value: code || '未选择', tone: 'secondary' },
        { label: '无障碍', value: '支持减少动态', tone: 'success' },
      ], sessionId ? [{
        key: `sim-stream-${sessionId}`,
        label: '仿真实时状态',
        description: '接收当前仿真会话的实时状态更新。',
        kind: 'status',
        href: api.sim.getStreamWsUrl(sessionId),
      }] : undefined, [
        pageAction('report-sim-action', '记录仿真操作', '将一次仿真交互操作记录到当前会话。', [
          textInput('session_id', '仿真会话编号', true),
          numberInput('seq', '操作序号', true),
          numberInput('at_tick', '仿真时刻', true),
          textInput('event_type', '事件类型', true),
          textareaInput('payload', '操作参数'),
        ], async (values) => {
          await api.sim.reportAction(valueText(values, 'session_id'), {
            seq: valueNumber(values, 'seq'),
            at_tick: valueNumber(values, 'at_tick'),
            event_type: valueText(values, 'event_type'),
            payload: valueJson(values, 'payload'),
          })
          return '仿真操作已上报'
        }),
        pageAction('read-sim-replay', '读取仿真回放', '按会话编号读取可复现实验回放。', [textInput('session_id', '仿真会话编号', true)], async (values) => {
          await api.sim.getReplay(valueText(values, 'session_id'))
          return '仿真回放已读取'
        }),
        pageAction('share-sim-session', '分享仿真回放', '为当前仿真会话生成分享码。', [textInput('session_id', '仿真会话编号', true)], async (values) => {
          await api.sim.shareSession(valueText(values, 'session_id'))
          return '仿真分享码已生成'
        }),
      ])
    },
  }
}

/**
 * solveWorkspaceRoute 渲染解题赛答题沉浸入口。
 */
export function solveWorkspaceRoute(): AppDefinition['routes'][number] {
  return {
    path: 'contest-solve',
    label: '竞赛答题',
    description: '沉浸式读取题面、创建环境并提交答案',
    icon: Flag,
    immersive: true,
    hidden: true,
    load: async (api, params) => {
      const contestId = routeParam(params, 'contest_id')
      const problemId = routeParam(params, 'problem_id')
      if (contestId && problemId) {
        await api.contest.getProblems(contestId)
      }
      return workspaceInfo('竞赛答题', '题面读取和提交判定均走竞赛模块，学生端不接触答案或判题配置。', [
        { label: '题面', value: problemId || '未选择', tone: 'primary' },
        { label: '环境', value: '按需创建', tone: 'secondary' },
        { label: '安全', value: '答案黑盒', tone: 'success' },
      ], undefined, [
        pageAction('create-contest-env', '创建答题环境', '为指定赛题创建受控答题环境。', [
          textInput('contest_id', '竞赛编号', true),
          textInput('problem_id', '题目编号', true),
          textInput('runtime_code', '运行时编码', true),
          textInput('runtime_image_version', '运行时镜像版本', true),
          textInput('tool_codes', '工具编码', true, '多个编码用英文逗号分隔。'),
          textInput('init_code_ref', '初始化代码引用'),
          textInput('init_script_ref', '初始化脚本引用'),
        ], async (values) => {
          await api.contest.createEnv(valueText(values, 'contest_id'), valueText(values, 'problem_id'), {
            runtime_code: valueText(values, 'runtime_code'),
            runtime_image_version: valueText(values, 'runtime_image_version'),
            tool_codes: valueText(values, 'tool_codes').split(',').map((item) => item.trim()).filter(Boolean),
            init_code_ref: valueText(values, 'init_code_ref') || undefined,
            init_script_ref: valueText(values, 'init_script_ref') || undefined,
          })
          return '答题环境已创建'
        }),
        pageAction('submit-contest-solve', '提交答案', '提交解题内容并等待判定结果。', [
          textInput('contest_id', '竞赛编号', true),
          textInput('problem_id', '题目编号', true),
          textareaInput('content_ref', '答案引用', true),
        ], async (values) => {
          await api.contest.submitSolve(valueText(values, 'contest_id'), valueText(values, 'problem_id'), { content_ref: valueJson(values, 'content_ref') })
          return '答案已提交'
        }),
        pageAction('read-contest-submission', '查看提交结果', '按提交编号读取判定结果。', [textInput('submission_id', '提交编号', true)], async (values) => {
          await api.contest.getSubmission(valueText(values, 'submission_id'))
          return '提交结果已读取'
        }),
        pageAction('submit-battle-entry', '提交对抗作品', '提交对抗赛参战作品引用。', [
          textInput('contest_id', '竞赛编号', true),
          numberInput('problem_id', '题目编号', true),
          numberInput('role', '参战角色', true),
          textInput('artifact_ref', '作品引用', true),
          textInput('code_hash', '代码哈希', true),
        ], async (values) => {
          await api.contest.submitBattleEntry(valueText(values, 'contest_id'), {
            problem_id: valueNumber(values, 'problem_id'),
            role: valueNumber(values, 'role'),
            artifact_ref: valueText(values, 'artifact_ref'),
            code_hash: valueText(values, 'code_hash'),
          })
          return '对抗作品已提交'
        }),
        pageAction('read-contest-ladder', '查看天梯榜', '读取当前竞赛天梯排名。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
          await api.contest.getLadder(valueText(values, 'contest_id'), defaultPageParams())
          return '天梯榜已读取'
        }),
        pageAction('prepare-leaderboard-realtime', '准备榜单实时更新', '准备指定竞赛的榜单实时更新信息。', [
          textInput('tenant_id', '学校编号', true),
          textInput('contest_id', '竞赛编号', true),
        ], async (values) => {
          api.contest.getLeaderboardTopic(valueText(values, 'tenant_id'), valueText(values, 'contest_id'))
          return '榜单实时更新已准备'
        }),
      ])
    },
  }
}

/**
 * battleReplayRoute 渲染对抗赛回放沉浸入口。
 */
export function battleReplayRoute(): AppDefinition['routes'][number] {
  return {
    path: 'battle-replay',
    label: '对抗回放',
    description: '按对局录制和执行轨迹复盘攻防过程',
    icon: Swords,
    immersive: true,
    hidden: true,
    load: async (api, params) => {
      const matchId = routeParam(params, 'match_id')
      if (matchId) {
        await api.contest.getBattleReplay(matchId)
      }
      return workspaceInfo('对抗回放', '回放入口读取对局录制内容，按真实执行轨迹复盘。', [
        { label: '对局', value: matchId || '未选择', tone: 'primary' },
        { label: '时间轴', value: '真实录制', tone: 'secondary' },
        { label: '复盘', value: '可暂停', tone: 'success' },
      ], undefined, [
        pageAction('read-battle-matches', '读取对局列表', '按竞赛编号读取对抗赛对局。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
          await api.contest.listBattleMatches(valueText(values, 'contest_id'), defaultPageParams())
          return '对局列表已读取'
        }),
        pageAction('read-battle-entries', '读取参战作品', '按竞赛编号读取对抗赛参战作品。', [textInput('contest_id', '竞赛编号', true)], async (values) => {
          await api.contest.listBattleEntries(valueText(values, 'contest_id'))
          return '参战作品已读取'
        }),
      ])
    },
  }
}

/**
 * sharedTransferRoute 四端统一承载导入导出任务和短时下载授权。
 */
export function sharedTransferRoute(): AppDefinition['routes'][number] {
  return {
    path: 'transfer-tasks',
    label: '任务与下载',
    description: '查看导入导出任务，并为已完成任务签发下载授权',
    icon: Download,
    group: '账户',
    load: async (api) => {
      const response = await api.transfer.listTasks(defaultPageParams())
      return {
        ...listResult({ list: response.items, total: response.items.length }, transferTaskColumns(), '暂无任务', '导入、导出或批量生成任务会在这里显示。'),
        actions: [
        pageAction('read-transfer-task', '读取任务详情', '按任务编号读取当前账号可见的任务快照。', [textInput('task_id', '任务编号', true)], async (values) => {
          await api.transfer.getTask(valueText(values, 'task_id'))
          return '任务详情已读取'
        }),
        pageAction('issue-download-grant', '获取下载授权', '为已完成任务签发短时下载授权。', [textInput('task_id', '任务编号', true)], async (values) => {
          await api.transfer.downloadGrant(valueText(values, 'task_id'))
          return '下载授权已生成'
        }),
        ],
        rowActions: [
        rowAction('download-grant-row', '下载授权', '为这条任务生成短时下载授权。', async (row) => {
          await api.transfer.downloadGrant(row.id)
          return '下载授权已生成'
        }),
        ],
      }
    },
  }
}
