// filters constants 文件维护跨模块共用的三态布尔筛选契约。

/**
 * BOOL_FILTER 是「布尔列」的服务端筛选取值,全平台同一套语义。
 *
 * 为什么需要它:布尔列没法用「缺省即不过滤」表达三种意图 —— `false` 与「不传」在查询串上
 * 长得一样。后端统一约定 `0=不限 / 1=是 / 2=否`,故前端也只用这一套取值,
 * 不给 `is_locked`、`is_late`、`is_shared` 各写一份(CLAUDE.md §4「同类问题全平台用同一种方案」)。
 *
 * 用它的接口(取值与后端 `httpx.QueryInt16(… Min:0, Max:2)` 一致):
 *   `grade.listReviews` 的 `is_locked`、`teaching.getSubmissions` 的 `is_late`、
 *   `teaching.getCourses` 的 `is_shared`。
 */
export const BOOL_FILTER = {
  /** 不按这一列过滤 */
  ANY: 0,
  YES: 1,
  NO: 2,
} as const

export type BoolFilter = (typeof BOOL_FILTER)[keyof typeof BOOL_FILTER]
