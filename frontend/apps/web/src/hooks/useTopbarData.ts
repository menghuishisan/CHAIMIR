// useTopbarData 从后端读取顶栏身份与未读通知数据，避免各角色壳重复拼接展示字段。

import { useEffect, useState } from 'react'
import { api } from '../app/api'
import { subscribeAppResource } from '../app/resourceInvalidation'
import { accountRoleLabel } from '../utils/labels'

export interface TopbarDataOptions {
  loadUnread?: boolean
}

export interface TopbarData {
  name: string
  meta: string
  avatar: string
  unreadCount: number | null
}

const EMPTY_PROFILE = {
  name: '账号信息读取中',
  meta: '正在连接服务端',
  avatar: '用',
}

/**
 * firstAvatarChar 取真实姓名的首字符作为头像文案。
 */
function firstAvatarChar(name: string): string {
  const trimmed = name?.trim()
  return trimmed.slice(0, 1)
}

/**
 * useTopbarData 聚合 /me 和未读通知数，并把不可用数据转换为明确的用户向提示。
 */
export function useTopbarData(options: TopbarDataOptions): TopbarData {
  const [data, setData] = useState<TopbarData>({
    ...EMPTY_PROFILE,
    unreadCount: null,
  })

  useEffect(() => {
    let active = true

    async function loadTopbarData(): Promise<void> {
      const [meResult, unreadResult] = await Promise.allSettled([
        api.identity.getMe(),
        options.loadUnread ? api.notify.getUnreadCount() : Promise.resolve({ unread: 0 }),
      ])

      let nextProfile = EMPTY_PROFILE
      if (meResult.status === 'fulfilled') {
        const account = meResult.value.account
        if (account.name.trim()) {
          nextProfile = {
            name: account.name,
            meta: account.phone_masked || accountRoleLabel(account.roles),
            avatar: firstAvatarChar(account.name),
          }
        } else {
          nextProfile = {
            name: '账号信息不完整',
            meta: '请联系管理员补全账号资料',
            avatar: '!',
          }
        }
      } else {
        nextProfile = {
          name: '账号信息不可用',
          meta: '请重新登录后查看',
          avatar: '用',
        }
      }

      let unreadCount: number | null = null
      if (unreadResult.status === 'fulfilled') {
        unreadCount = options.loadUnread ? unreadResult.value.unread : null
      }

      if (active) {
        setData({
          ...nextProfile,
          unreadCount,
        })
      }
    }

    void loadTopbarData()
    const unsubscribeProfile = subscribeAppResource('profile', () => void loadTopbarData())
    const unsubscribeUnread = subscribeAppResource('notification-unread', () => void loadTopbarData())
    return () => {
      active = false
      unsubscribeProfile()
      unsubscribeUnread()
    }
  }, [options.loadUnread])

  return data
}
