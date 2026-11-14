// SearchPicker 在大集合里挑一项:触发器外观与下拉框一致,点开是「搜索框 + 至多 20 条结果」,
// 输入停顿后由服务端按关键词查找,不把整个集合一次取回(账号目录、题库、仿真场景都可能有成百上千条)。
// 可选的第一项用于清除选择(筛选里的「不限」);当前选中项的文字由调用方给出 —— 它可能不在当前结果里。
// 浮层与下拉框一样在弹窗之上,表单弹窗里也能用;浮层不窄于触发器,整宽字段展开后与输入框对齐。
// 无障碍语义与下拉框一致:触发器是组合框,读屏会读出当前选择;搜索框里用上下键在结果间移动、回车选中。

import {
  useEffect,
  useId,
  useState,
  type DependencyList,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import { Check, ChevronDown, Search } from 'lucide-react'
import {
  cn,
  Icon,
  Input,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Skeleton,
  selectTriggerVariants,
} from '@chaimir/ui'
import { useAsyncResource } from '../../hooks'

/** 一次最多列出的条数:更多时提示继续输入 */
const RESULT_LIMIT = 20
/** 输入停顿多久后再查找(毫秒):逐字查找会让每次按键都发一次请求 */
const SEARCH_DELAY_MS = 300
/** 清除选择那一项的键;查到的条目一律带 item: 前缀,不会与它撞上 */
const CLEAR_KEY = 'clear'

/** 一次查找的结果:本次列出的条目与符合条件的总数 */
export interface SearchPickerPage<T> {
  list: T[]
  total: number
}

export interface SearchPickerProps<T> {
  /** 触发器的 id:表单里传它,由表单标签给触发器命名(读屏连「必填」一起读);筛选栏里不传,改用 label 命名 */
  id?: string
  /** 结果列表与浮层的无障碍名称;没有 id(没有表单标签)时也用它给触发器命名 */
  label: string
  /** 触发器上的文字:当前选中项的说法,或没有选中时的占位 */
  triggerText: string
  /** 当前没有选中任何项:触发器按占位样式显示 */
  empty: boolean
  /** 搜索框的提示文字 */
  searchPlaceholder: string
  /** 按关键词查找(空串为不带关键词),最多取 limit 条 */
  search: (keyword: string, limit: number) => Promise<SearchPickerPage<T>>
  /** 关键词以外、变化时要重新查找的条件(如身份、题型) */
  searchDeps?: DependencyList
  itemKey: (item: T) => string
  renderItem: (item: T) => ReactNode
  isSelected: (item: T) => boolean
  onSelect: (item: T) => void
  /** 清除选择那一项的文字(如「全部学生」);不传则没有这一项 */
  clearLabel?: string
  onClear?: () => void
  /** 没有结果时的说法;keyword 为空表示集合本身是空的 */
  emptyText: (keyword: string) => string
  invalid?: boolean
  disabled?: boolean
  className?: string
}

/** 结果列表里的一项:清除选择与查到的条目合成同一种形状,上下键按这个顺序移动 */
interface PickerOption {
  key: string
  selected: boolean
  content: ReactNode
  choose: () => void
}

/** SearchPicker 触发器显示当前选择;点开后输入关键词,由服务端查找并点选一项 */
export function SearchPicker<T>({
  id,
  label,
  triggerText,
  empty,
  searchPlaceholder,
  search,
  searchDeps = [],
  itemKey,
  renderItem,
  isSelected,
  onSelect,
  clearLabel,
  onClear,
  emptyText,
  invalid = false,
  disabled = false,
  className,
}: SearchPickerProps<T>) {
  const listId = useId()
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [query, setQuery] = useState('')
  // 键盘或指针停在哪一项:按键记,结果刷新后找不到这一项就回到第一项
  const [activeKey, setActiveKey] = useState<string>()

  // 输入停顿后才查找
  useEffect(() => {
    const timer = window.setTimeout(() => setQuery(keyword.trim()), SEARCH_DELAY_MS)
    return () => window.clearTimeout(timer)
  }, [keyword])

  // 只在展开时查找;收起后不发请求
  const results = useAsyncResource<SearchPickerPage<T> | null>(
    () => (open ? search(query, RESULT_LIMIT) : Promise.resolve(null)),
    [open, query, ...searchDeps],
    () => false
  )

  /** close 收起并清空关键词,下次打开从空搜索、第一项开始 */
  const close = () => {
    setOpen(false)
    setKeyword('')
    setActiveKey(undefined)
  }

  const ready = results.status !== 'loading' && results.status !== 'error'
  const list = ready ? (results.data?.list ?? []) : []
  const more = ready && results.data ? Math.max(0, results.data.total - list.length) : 0

  const options: PickerOption[] = []
  if (ready && clearLabel && onClear) {
    options.push({
      key: CLEAR_KEY,
      selected: empty,
      content: clearLabel,
      choose: () => {
        onClear()
        close()
      },
    })
  }
  for (const item of list) {
    options.push({
      key: `item:${itemKey(item)}`,
      selected: isSelected(item),
      content: renderItem(item),
      choose: () => {
        onSelect(item)
        close()
      },
    })
  }
  const activeIndex = Math.max(
    0,
    options.findIndex((option) => option.key === activeKey)
  )
  const optionId = (index: number) => `${listId}-${index}`

  /** moveActive 上下键在结果间移动一格,并把移到的一项滚进可见范围 */
  const moveActive = (step: number) => {
    if (options.length === 0) return
    const next = Math.min(options.length - 1, Math.max(0, activeIndex + step))
    setActiveKey(options[next].key)
    document.getElementById(optionId(next))?.scrollIntoView({ block: 'nearest' })
  }

  /** handleSearchKeyDown 搜索框里的键盘操作:上下键移动、回车选中;Esc 由浮层负责收起 */
  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      moveActive(event.key === 'ArrowDown' ? 1 : -1)
    } else if (event.key === 'Enter') {
      // 回车只用来选中,不提交外层表单
      event.preventDefault()
      options[activeIndex]?.choose()
    }
  }

  return (
    <Popover open={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
      <PopoverTrigger
        id={id}
        role="combobox"
        aria-label={id ? undefined : label}
        aria-invalid={invalid || undefined}
        disabled={disabled}
        data-placeholder={empty ? '' : undefined}
        className={cn(selectTriggerVariants({ invalid }), className)}
      >
        <span className="min-w-0 truncate">{triggerText}</span>
        <Icon icon={ChevronDown} size="sm" className="shrink-0 text-ink-sub" />
      </PopoverTrigger>
      <PopoverContent
        align="start"
        aria-label={label}
        className="z-modal w-picker-popover min-w-popover-trigger p-2"
      >
        <Input
          role="combobox"
          aria-label={searchPlaceholder}
          aria-expanded
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={options.length > 0 ? optionId(activeIndex) : undefined}
          placeholder={searchPlaceholder}
          leftIcon={Search}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onKeyDown={handleSearchKeyDown}
          autoFocus
        />
        <div className="mt-2 max-h-72 overflow-y-auto">
          <ul id={listId} role="listbox" aria-label={label} className="flex flex-col">
            {options.map((option, index) => (
              <li
                key={option.key}
                id={optionId(index)}
                role="option"
                aria-selected={index === activeIndex}
                onMouseMove={() => setActiveKey(option.key)}
                onClick={option.choose}
                className={cn(
                  'relative flex cursor-pointer items-center rounded-sm py-1.5 pl-2 pr-8 text-base text-ink transition-colors duration-fast',
                  index === activeIndex && 'bg-surface-hover'
                )}
              >
                {option.content}
                {option.selected ? (
                  <>
                    <Icon icon={Check} size="sm" className="absolute right-2 text-primary" />
                    <span className="sr-only">,当前选择</span>
                  </>
                ) : null}
              </li>
            ))}
          </ul>
          {results.status === 'loading' ? (
            <div className="p-2">
              <Skeleton variant="line" lines={4} />
            </div>
          ) : null}
          {results.status === 'error' ? (
            <p className="px-2 py-3 text-sm text-ink-sub">暂时读不到,请稍后重试。</p>
          ) : null}
          {ready && list.length === 0 ? (
            <p className="px-2 py-3 text-sm text-ink-sub">{emptyText(query)}</p>
          ) : null}
          {more > 0 ? (
            <p className="px-2 py-2 text-xs text-ink-sub">还有 {more} 条,继续输入以缩小范围。</p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  )
}
