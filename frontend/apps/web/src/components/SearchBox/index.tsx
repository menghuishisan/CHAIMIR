// SearchBox 列表页的搜索框(设计 §6.4:筛选与搜索写进地址)。
// 输入时只改本地草稿,回车或点放大镜才写进地址并触发读取 —— 每敲一个字就请求一次会把后端打碎;
// 有内容时右侧出现清空按钮(用 type=text:type=search 会再带一个浏览器自己的清空叉)。地址里的搜索词变了(后退、点「清除筛选」)时草稿跟着变。

import { useEffect, useState, type FormEvent } from 'react'
import { Search, X } from 'lucide-react'
import { cn, IconButton, Input } from '@chaimir/ui'

export interface SearchBoxProps {
  /** 地址里当前生效的搜索词 */
  value: string
  /** 提交搜索词(已去掉首尾空白);清空时传空串 */
  onSearch: (value: string) => void
  placeholder: string
  /** 无障碍名称,如「搜索实验」 */
  label: string
  className?: string
}

/** SearchBox 渲染一个回车提交的搜索框 */
export function SearchBox({ value, onSearch, placeholder, label, className }: SearchBoxProps) {
  const [draft, setDraft] = useState(value)
  useEffect(() => setDraft(value), [value])

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    onSearch(draft.trim())
  }

  return (
    <form role="search" onSubmit={submit} className={cn('relative w-full sm:w-72', className)}>
      <Input
        type="text"
        aria-label={label}
        leftIcon={Search}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        placeholder={placeholder}
        enterKeyHint="search"
        className={draft ? 'pr-10' : undefined}
      />
      {draft ? (
        <IconButton
          icon={X}
          size="sm"
          variant="ghost"
          aria-label="清空搜索"
          className="absolute right-1 top-1/2 -translate-y-1/2"
          onClick={() => {
            setDraft('')
            onSearch('')
          }}
        />
      ) : null}
    </form>
  )
}
