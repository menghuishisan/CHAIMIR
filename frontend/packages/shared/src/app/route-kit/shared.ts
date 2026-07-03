// 共享路由：通知、个人中心、公告和沉浸式工作台入口。

import { Bell, Flag, Megaphone, PlayCircle, Swords, UserCog } from 'lucide-react'
import type { AppDefinition } from '../types'
import { dateText, idOf, listResult, statusText, text, toRows, workspaceInfo } from './results'
import { announcementColumns, notificationColumns, sessionColumns } from './columns'
import { routeParam } from './support'
import { numberInput, pageAction, passwordInput, rowAction, textInput, valueNumber, valueText } from './actions'

export function sharedNotificationRoute(): AppDefinition['routes'][number] {
  return {
    path: 'notifications',
    label: '通知公告',
    description: '查看通知、公告和未读状态',
    icon: Bell,
    load: async (api) => ({
      ...listResult(await api.notify.getNotifications({ page: 1, size: 20 }), notificationColumns(), '暂无通知', '新的通知和公告会在这里显示。'),
      actions: [
        pageAction('mark-all-read', '全部标记已读', '将当前账号的未读通知全部标记为已读。', [], async () => {
          await api.notify.markAllAsRead()
          return '通知已全部标记为已读'
        }),
        pageAction('update-notify-preference', '更新通知偏好', '按通知类型启用或关闭接收偏好。', [
          textInput('type', '通知类型', true),
          numberInput('enabled', '是否启用', true, '1 表示启用，0 表示关闭'),
        ], async (values) => {
          await api.notify.updatePreference(valueText(values, 'type'), { enabled: valueNumber(values, 'enabled') === 1 })
          return '通知偏好已更新'
        }),
      ],
      rowActions: [
        rowAction('mark-read', '标记已读', '将这条通知标记为已读。', async (row) => {
          await api.notify.markAsRead(row.id)
          return '通知已标记为已读'
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
    load: async (api) => {
      const me = await api.identity.getMe()
      const sessions = await api.identity.getSessions()
      return {
        metrics: [
          { label: '姓名', value: text(me.name), tone: 'primary' },
          { label: '账号状态', value: statusText(me.status), tone: 'success' },
          { label: '登录会话', value: String(sessions.length), tone: 'secondary' },
        ],
        columns: sessionColumns(),
        rows: toRows(sessions, (item, index) => ({
          id: idOf(item, index),
          device_info: text(item.device_info || '当前设备'),
          ip: text(item.ip || '未记录'),
          status: statusText(item.status),
          expire_at: dateText(item.expire_at),
          created_at: dateText(item.created_at),
        })),
        emptyTitle: '暂无会话',
        emptyDescription: '登录会话会在这里显示。',
        actions: [
          pageAction('change-password', '修改密码', '修改当前账号密码，提交后由后端更新登录凭据。', [
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
      ...listResult(await api.notify.getAnnouncements({ page: 1, size: 20 }), announcementColumns(), '暂无公告', '有公告发布后会在这里显示。'),
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
      if (code && version) {
        await api.sim.getBundleGrant(code, version)
      }
      return workspaceInfo('仿真工作台', '仿真包加载授权由后端签发，前端只运行已发布或已审核通过的 bundle。', [
        { label: '播放控制', value: '单步与回放', tone: 'primary' },
        { label: '状态来源', value: code || '未选择', tone: 'secondary' },
        { label: '无障碍', value: '支持减少动态', tone: 'success' },
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
      return workspaceInfo('对抗回放', '回放入口读取后端对局录制引用，按真实执行轨迹复盘。', [
        { label: '对局', value: matchId || '未选择', tone: 'primary' },
        { label: '时间轴', value: '真实录制', tone: 'secondary' },
        { label: '复盘', value: '可暂停', tone: 'success' },
      ])
    },
  }
}
