// Pagination 组件：分页

import React from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { clsx } from 'clsx'
import './Pagination.css'

export interface PaginationProps {
  /** 当前页 */
  current: number
  /** 总页数 */
  total: number
  /** 每页条数（用于计算，可选） */
  pageSize?: number
  /** 总条数 */
  totalItems?: number
  /** 变化回调 */
  onChange?: (page: number) => void
  /** 是否显示快速跳转 */
  showQuickJumper?: boolean
  /** 是否显示总数 */
  showTotal?: boolean
  /** 自定义类名 */
  className?: string
  /** 分页导航说明 */
  ariaLabel?: string
}

export const Pagination: React.FC<PaginationProps> = ({
  current,
  total,
  pageSize: _pageSize = 20,
  totalItems,
  onChange,
  showQuickJumper = false,
  showTotal = true,
  ariaLabel = '分页',
  className,
}) => {
  const [jumpValue, setJumpValue] = React.useState('')

  const handlePageChange = (page: number) => {
    if (page < 1 || page > total || page === current) return
    onChange?.(page)
  }

  const handleJump = () => {
    const page = parseInt(jumpValue, 10)
    if (!isNaN(page)) {
      handlePageChange(page)
      setJumpValue('')
    }
  }

  /**
   * renderPageNumbers 根据当前页折叠页码，避免分页在窄屏挤压内容。
   */
  const renderPageNumbers = () => {
    const pages: (number | string)[] = []
    const showPages = 7 // 最多显示7个页码

    if (total <= showPages) {
      for (let i = 1; i <= total; i++) {
        pages.push(i)
      }
    } else {
      pages.push(1)

      if (current <= 3) {
        for (let i = 2; i <= 4; i++) {
          pages.push(i)
        }
        pages.push('...')
        pages.push(total)
      } else if (current >= total - 2) {
        pages.push('...')
        for (let i = total - 3; i <= total; i++) {
          pages.push(i)
        }
      } else {
        pages.push('...')
        for (let i = current - 1; i <= current + 1; i++) {
          pages.push(i)
        }
        pages.push('...')
        pages.push(total)
      }
    }

    return pages
  }

  const classes = clsx('chaimir-pagination', className)

  return (
    <nav className={classes} aria-label={ariaLabel}>
      {showTotal && totalItems && (
        <div className="chaimir-pagination__total">
          共 {totalItems} 条
        </div>
      )}

      <div className="chaimir-pagination__list">
        <button
          type="button"
          className="chaimir-pagination__btn"
          onClick={() => handlePageChange(current - 1)}
          disabled={current === 1}
          aria-label="上一页"
        >
          <ChevronLeft size={16} />
        </button>

        {renderPageNumbers().map((page, index) => {
          if (page === '...') {
            return (
              <span key={`ellipsis-${index}`} className="chaimir-pagination__ellipsis">
                •••
              </span>
            )
          }

          return (
            <button
              key={page}
              type="button"
              className={clsx(
                'chaimir-pagination__item',
                page === current && 'chaimir-pagination__item--active',
                !isCompactVisible(page as number, current, total) && 'chaimir-pagination__item--compact-hidden'
              )}
              aria-current={page === current ? 'page' : undefined}
              aria-label={`第 ${page} 页`}
              onClick={() => handlePageChange(page as number)}
            >
              {page}
            </button>
          )
        })}

        <button
          type="button"
          className="chaimir-pagination__btn"
          onClick={() => handlePageChange(current + 1)}
          disabled={current === total}
          aria-label="下一页"
        >
          <ChevronRight size={16} />
        </button>
      </div>

      {showQuickJumper && (
        <div className="chaimir-pagination__jumper">
          跳至
          <input
            type="number"
            className="chaimir-pagination__input"
            value={jumpValue}
            onChange={(e) => setJumpValue(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleJump()}
            aria-label="输入要跳转的页码"
            min={1}
            max={total}
          />
          页
        </div>
      )}
    </nav>
  )
}

Pagination.displayName = 'Pagination'

/**
 * isCompactVisible 保留首页、尾页、当前页和相邻页，小屏隐藏其余页码。
 */
function isCompactVisible(page: number, current: number, total: number): boolean {
  return page === 1 || page === total || Math.abs(page - current) <= 1
}
