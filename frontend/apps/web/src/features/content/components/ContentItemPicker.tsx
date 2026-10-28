// 内容域统一题库选题列表片段,由试卷页面或课程作业内容区承载。
//
// 试卷与课程作业都从 M5 题库选择已发布版本,查询、列表展示和选择交互属于同一内容能力;
// 调用方只负责把选中题目写入自己的业务表单,避免跨模块重复实现题库读取逻辑。

import { useId, useState } from 'react'
import { FileText, Search } from 'lucide-react'
import { ContentStatus, type ContentItem } from '@chaimir/api-client'
import {
  Button,
  DataPanel,
  FilterBar,
  FilterField,
  Input,
  Pagination,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { ContentItemClassificationCell, ContentItemIdentityCell } from './ContentItemCells'

export interface ContentItemPickerProps {
  /** 已在目标业务对象中的题目编号,这些行不可重复选择。 */
  selectedCodes: ReadonlySet<string>
  /** 用于向用户说明题目已被哪个对象占用。 */
  targetName: '试卷' | '作业'
  onClose: () => void
  onPick: (items: ContentItem[]) => void
}

/**
 * ContentItemPicker 读取题库并返回本次选择的题目版本。
 * 题目版本由调用方按公开 DTO 写入目标表单,本组件不承担试卷或作业的状态管理。
 */
export function ContentItemPicker({
  selectedCodes,
  targetName,
  onClose,
  onPick,
}: ContentItemPickerProps) {
  const [picked, setPicked] = useState<ContentItem[]>([])
  const keywordId = useId()
  const [keyword, setKeyword] = useState('')
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const items = usePagedResource(
    (params) =>
      api.content.getItems({
        ...params,
        status: ContentStatus.PUBLISHED,
        keyword: submittedKeyword || undefined,
      }),
    [submittedKeyword]
  )

  /** handleSearch 由服务端查询完整已发布库,同条件再次查询时也刷新当前结果。 */
  const handleSearch = () => {
    const next = keyword.trim()
    if (next === submittedKeyword) items.reload()
    else setSubmittedKeyword(next)
  }

  const columns: TableColumn<ContentItem>[] = [
    {
      key: 'title',
      header: '题目',
      render: (item) => <ContentItemIdentityCell item={item} />,
    },
    {
      key: 'type',
      header: '类型',
      render: (item) => <ContentItemClassificationCell item={item} />,
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => {
        const alreadyInTarget = selectedCodes.has(item.code)
        const isPicked = picked.some((candidate) => candidate.code === item.code)
        return (
          <Button
            type="button"
            variant={isPicked ? 'outline' : 'ghost'}
            size="sm"
            disabled={alreadyInTarget}
            onClick={() =>
              setPicked((current) =>
                isPicked
                  ? current.filter((candidate) => candidate.code !== item.code)
                  : [...current, item]
              )
            }
          >
            {alreadyInTarget ? `已在${targetName}中` : isPicked ? '取消选择' : '选择'}
          </Button>
        )
      },
    },
  ]

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-ink-sub">
        仅可选择已发布题目。选中的题目按当前版本加入{targetName},之后题库改动不影响已添加内容。
      </p>
      <DataPanel
        label="已发布题目"
        filter={
          <FilterBar label="选题筛选" onSubmit={handleSearch} submitLabel="搜索">
            <FilterField label="题目关键词" htmlFor={keywordId}>
              <Input
                id={keywordId}
                value={keyword}
                leftIcon={Search}
                placeholder="输入关键词"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={items.page}
            pageSize={items.pageSize}
            total={items.total}
            onPageChange={items.setPage}
          />
        }
      >
        <ResourceState
          resource={items}
          emptyIcon={FileText}
          emptyTitle={submittedKeyword ? '没有匹配的已发布题目' : '还没有可选择的已发布题目'}
          emptyDescription={
            submittedKeyword
              ? '换个关键词再试,或清空关键词查看全部。'
              : '先在题库中发布题目,再回来选择。'
          }
          skeleton={
            <Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />
          }
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):题名一行、分类一行,选择按钮在右
              mobileCard={(item) => {
                const alreadyInTarget = selectedCodes.has(item.code)
                const isPicked = picked.some((candidate) => candidate.code === item.code)
                return {
                  title: item.title,
                  meta: <ContentItemClassificationCell item={item} />,
                  action: (
                    <Button
                      type="button"
                      variant={isPicked ? 'outline' : 'ghost'}
                      size="sm"
                      disabled={alreadyInTarget}
                      onClick={() =>
                        setPicked((current) =>
                          isPicked
                            ? current.filter((candidate) => candidate.code !== item.code)
                            : [...current, item]
                        )
                      }
                    >
                      {alreadyInTarget ? `已在${targetName}中` : isPicked ? '取消选择' : '选择'}
                    </Button>
                  ),
                }
              }}
            />
          )}
        </ResourceState>
      </DataPanel>
      <div className="flex flex-wrap justify-end gap-3">
        <Button type="button" variant="outline" onClick={onClose}>
          取消
        </Button>
        <Button
          type="button"
          variant="primary"
          disabled={picked.length === 0}
          onClick={() => onPick(picked)}
        >
          添加 {picked.length > 0 ? `${picked.length} 道` : ''}题目
        </Button>
      </div>
    </div>
  )
}
