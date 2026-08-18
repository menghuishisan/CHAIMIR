// 课程封面地址解析(教学各页共用)。
//
// 封面存的是对象引用而不是可显示的地址:要显示得先换一张短时投放授权,再交给统一文件服务。
// 封面只对能看到该课程的人可见,所以没有公开路径,每次进页面都重新换授权。
//
// 只在「一门课程」的语境里用(详情页、编辑表单)。课程列表的缩略图仍用四张纸材质加题识:
// 一页 20 门课就要换 20 次授权,而列表接口早已按可见性筛过课程,逐行再换一次授权
// 不产生任何新的鉴权判断,只是把一次列表渲染变成二十多个请求。

import { useEffect, useState } from 'react'
import { api } from '../../app/api'
import { errorDiagnostics } from '../../utils/userFacingError'

/**
 * useCourseCoverSrc 把课程封面引用换成可直接作为 img src 的地址。
 * 课程没有封面时返回空串,由 CoverImage 回落到纸材质。
 */
export function useCourseCoverSrc(courseId: string | undefined, coverRef: string | undefined): string {
  const [src, setSrc] = useState('')

  useEffect(() => {
    if (courseId === undefined || courseId === '' || coverRef === undefined || coverRef === '') {
      setSrc('')
      return
    }
    let active = true
    void (async () => {
      try {
        const grant = await api.teaching.issueCourseCoverAccess(courseId)
        if (active) setSrc(api.storage.streamUrl(grant.token))
      } catch (error) {
        // 封面取不到不该影响课程内容:留空即回落纸材质,失败原因只进控制台供排障。
        console.error('课程封面授权换取失败', { courseId, error: errorDiagnostics(error) })
        if (active) setSrc('')
      }
    })()
    return () => {
      active = false
    }
  }, [courseId, coverRef])

  return src
}
