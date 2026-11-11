// reviewQueue.ts 审阅队列(批改、成绩审核、申诉、实验报告)的移动规则,四处队列共用这一份(设计 §6 QueueDetailLayout):
// 上下方向键在队列内移动且不越界;处理完一条后进「下一条待处理」—— 先往后找,后面没有再从头找。
// 纯函数,供 node:test 直接运行。

/** stepQueue 从当前条按 step(+1 下移 / -1 上移)移动,到头到尾停住;当前没有选中时取第一条 */
export function stepQueue<T extends { id: string }>(items: readonly T[], currentId: string | undefined, step: 1 | -1): string | undefined {
  if (items.length === 0) return undefined
  const index = items.findIndex((item) => item.id === currentId)
  if (index < 0) return items[0].id
  return items[Math.min(Math.max(index + step, 0), items.length - 1)].id
}

/** nextPendingId 当前条之后的第一条待处理;后面没有就从头找;都没有(全部处理完)返回 undefined */
export function nextPendingId<T extends { id: string }>(
  items: readonly T[],
  currentId: string | undefined,
  isPending: (item: T) => boolean,
): string | undefined {
  const index = items.findIndex((item) => item.id === currentId)
  const after = items.slice(index + 1).find((item) => isPending(item))
  if (after) return after.id
  return items.slice(0, Math.max(index, 0)).find((item) => isPending(item))?.id
}
