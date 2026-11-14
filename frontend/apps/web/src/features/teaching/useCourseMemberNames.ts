// useCourseMemberNames 把课程里的学生编号换成姓名与学号(批改队列、课程成绩表:这里要分清「已退课学生」,
// 所以按该课成员名单解析,而不是按账号;申诉、预警等跨课程列表的姓名由接口随记录给出;
// 实时监控里的实验环境横跨教师的几门课,所以接收若干门课)。
// 成员接口已返回姓名与学号,教师有权读;每页最多 100 条,成员超过 100 人时按页取完。
// 解析不到(已退课)的显示「已退课学生」,不显示编号;读取中由调用方显示占位,失败时显示「姓名暂时读不到」。

import { useCallback, useMemo } from 'react'
import { PAGINATION_MAX_SIZE, type CourseMember } from '@chaimir/api-client'
import { api } from '../../app/api'
import { useAsyncResource } from '../../hooks'
import { collectAllPages } from '../../utils/allPages'

export type MemberName =
  | { state: 'loading' }
  | { state: 'failed' }
  | { state: 'left' }
  | { state: 'found'; name: string; no?: string }

/**
 * useCourseMemberNames 读取给定几门课的全部成员,返回按学生编号查姓名的函数;没有课程时不读。
 * 空值与重复的课程编号会被去掉;按编号内容记忆,调用方每次传新数组也不会重复请求。
 */
export function useCourseMemberNames(courseIds: readonly (string | undefined)[]) {
  const key = [...new Set(courseIds.filter((id): id is string => Boolean(id)))].sort().join(',')
  const members = useAsyncResource<CourseMember[]>(
    () =>
      key
        ? Promise.all(
            key
              .split(',')
              .map((courseId) => collectAllPages((params) => api.teaching.listMembers(courseId, params), PAGINATION_MAX_SIZE)),
          ).then((lists) => lists.flat())
        : Promise.resolve([]),
    [key],
    () => false,
  )
  const byStudent = useMemo(
    () => new Map((members.data ?? []).map((member) => [member.student_id, member])),
    [members.data],
  )
  const status = members.status
  const lookup = useCallback(
    (studentId: string): MemberName => {
      if (status === 'loading') return { state: 'loading' }
      if (status === 'error') return { state: 'failed' }
      const member = byStudent.get(studentId)
      return member ? { state: 'found', name: member.student_name, no: member.student_no } : { state: 'left' }
    },
    [byStudent, status],
  )
  return { lookup, reload: members.reload }
}
