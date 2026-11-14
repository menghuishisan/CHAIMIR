// allPages.ts 把分页接口按每页最大条数取完(设计 §10:成员、账号超过一页时要分页取完)。
// 只用于按设计本来就小的集合(一门课的成员与作业、一个班的学生、一个人的课程与申诉、一场竞赛的队伍):
// 下拉要列出完整名单、或要在前端跨页排序时才取完。账号目录、题库、仿真场景这类大集合不在此列,
// 它们由 SearchPicker 交给服务端按关键词查找;列表里的姓名与课程名由接口随记录给出。
// 取到总数、遇到不满一页或空页即停 —— 总数与实际不符时也不会无限请求。
// 纯函数,供 node:test 直接运行,不在运行时导入 @chaimir/*。

/** 一次分页请求的参数 */
export interface PageRequest {
  page: number
  size: number
}

/** 一页结果里用到的字段(与接口的分页响应同形) */
export interface PageResult<T> {
  list: T[]
  total: number
}

/** collectAllPages 从第 1 页起按 pageSize 逐页读取,拼成完整列表 */
export async function collectAllPages<T>(
  loader: (params: PageRequest) => Promise<PageResult<T>>,
  pageSize: number,
): Promise<T[]> {
  const all: T[] = []
  for (let page = 1; ; page += 1) {
    const result = await loader({ page, size: pageSize })
    all.push(...result.list)
    if (result.list.length < pageSize || all.length >= result.total) return all
  }
}
