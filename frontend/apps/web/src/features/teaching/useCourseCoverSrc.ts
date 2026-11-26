// 课程封面地址解析(教学各页共用):把受保护的对象引用换成短时文件流地址。

import { useEffect, useState } from 'react'
import { api } from '../../app/api'
import { errorDiagnostics } from '../../utils/userFacingError'

/** 返回课程封面可显示地址;没有封面或授权失败时返回空串。 */
export function useCourseCoverSrc(courseId: string | undefined, coverRef: string | undefined): string {
  const currentKey = `${courseId ?? ''}:${coverRef ?? ''}`
  const [result, setResult] = useState<{ key: string; src: string }>({ key: '', src: '' })

  useEffect(() => {
    if (!courseId || !coverRef) return
    const requestKey = `${courseId}:${coverRef}`
    let active = true
    void api.teaching
      .issueCourseCoverAccess(courseId)
      .then((grant) => {
        if (active) setResult({ key: requestKey, src: api.storage.streamUrl(grant.token) })
      })
      .catch((error: unknown) => {
        console.error('课程封面授权换取失败', { courseId, error: errorDiagnostics(error) })
        if (active) setResult({ key: requestKey, src: '' })
      })
    return () => {
      active = false
    }
  }, [courseId, coverRef])

  return result.key === currentKey && Boolean(courseId && coverRef) ? result.src : ''
}
