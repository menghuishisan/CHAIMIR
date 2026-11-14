// AccountPicker 在本校账号里按姓名、学工号或完整手机号挑一个人(审计的操作人筛选、学业预警的学生筛选与扫描范围)。
// 查找交给 SearchPicker:输入停顿后由服务端按关键词查账号目录,不把全校账号一次取回;
// 第一项「不限」用于清除选择。地址里恢复的选择按编号单独读取一次,用来在触发器上显示是谁。

import { API_ERROR_CODES, ApiError, type Account, type BaseIdentity } from '@chaimir/api-client'
import { api } from '../../../app/api'
import { SearchPicker } from '../../../components/SearchPicker'
import { useAsyncResource, type AsyncResourceState } from '../../../hooks'

export interface AccountPickerProps {
  /** 当前选中的账号编号;空串表示不限 */
  value: string
  onChange: (accountId: string) => void
  /** 只在这类基础身份里找(如只找学生);不传为全部账号 */
  role?: BaseIdentity
  /** 「不限」那一项的文字,如「全部操作人」 */
  allLabel: string
  /** 触发器与结果列表的无障碍名称,如「按操作人筛选」 */
  label: string
  /** 触发器的 id,供表单标签关联 */
  id?: string
  className?: string
}

/** pickedText 触发器上显示的当前选择;读不到时给出说法,不显示编号 */
function pickedText(
  value: string,
  picked: AsyncResourceState<Account | null>,
  allLabel: string
): string {
  if (!value) return allLabel
  if (picked.status === 'error') {
    return picked.error instanceof ApiError &&
      picked.error.code === API_ERROR_CODES.RESOURCE_NOT_FOUND
      ? '已移除的账号'
      : '姓名暂时读不到'
  }
  if (picked.status === 'loading' || !picked.data) return '正在读取…'
  return picked.data.no ? `${picked.data.name} · ${picked.data.no}` : picked.data.name
}

/** AccountPicker 触发器显示当前选中的人;点开后输入姓名、学工号或完整手机号查找并点选 */
export function AccountPicker({
  value,
  onChange,
  role,
  allLabel,
  label,
  id,
  className,
}: AccountPickerProps) {
  // 当前选中的人按编号读一次:从地址恢复的筛选也能显示是谁
  const picked = useAsyncResource<Account | null>(
    () => (value ? api.identity.getAccount(value) : Promise.resolve(null)),
    [value],
    () => false
  )

  return (
    <SearchPicker<Account>
      id={id}
      label={label}
      triggerText={pickedText(value, picked, allLabel)}
      empty={!value}
      searchPlaceholder="输入姓名、学工号或完整手机号"
      search={(keyword, limit) =>
        api.identity.getAccounts({ keyword: keyword || undefined, role, page: 1, size: limit })
      }
      searchDeps={[role]}
      itemKey={(account) => account.id}
      renderItem={(account) => (
        <>
          <span className="min-w-0 truncate">{account.name}</span>
          {account.no ? (
            <span className="ml-2 shrink-0 font-mono text-xs text-ink-sub">{account.no}</span>
          ) : null}
        </>
      )}
      isSelected={(account) => account.id === value}
      onSelect={(account) => onChange(account.id)}
      clearLabel={allLabel}
      onClear={() => onChange('')}
      emptyText={(keyword) =>
        keyword ? '没有找到这个人,换个姓名或学工号试试。' : '还没有可选的账号。'
      }
      className={className}
    />
  )
}
