/** 共用的三态筛选值。0 表示不限，1 表示是，2 表示否。 */
export const BOOL_FILTER = {
  ANY: 0,
  YES: 1,
  NO: 2,
} as const

export type BoolFilter = (typeof BOOL_FILTER)[keyof typeof BOOL_FILTER]
