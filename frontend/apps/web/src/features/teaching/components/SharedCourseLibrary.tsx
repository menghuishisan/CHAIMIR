// SharedCourseLibrary提供教师跨校课程目录，克隆复用M6现有表单与权限入口。
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Copy, Search, Share2 } from 'lucide-react'
import type { SharedCourse } from '@chaimir/api-client'
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
import { formatShortDateTime } from '../../../utils/formatters'
import { courseTypeLabel, teachingDifficultyLabel } from '../../../utils/labels/teaching'
import { CloneCourseModal } from './CloneCourseModal'

/** SharedCourseLibrary以服务端共享目录为权威，失败态不显示为合法空目录。 */
export function SharedCourseLibrary() {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const [cloneTarget, setCloneTarget] = useState<SharedCourse>()
  const shared = usePagedResource<SharedCourse>(
    (params) =>
      api.teaching.getSharedCourses({ ...params, keyword: submittedKeyword || undefined }),
    [submittedKeyword]
  )
  const columns: TableColumn<SharedCourse>[] = [
    {
      key: 'name',
      header: '课程',
      render: (course) => (
        <div>
          <p className="font-medium text-ink">{course.name}</p>
          <p className="text-sm text-ink-sub">{course.description}</p>
        </div>
      ),
    },
    { key: 'type', header: '课程类型', render: (course) => courseTypeLabel(course.type) },
    {
      key: 'difficulty',
      header: '难度',
      render: (course) => teachingDifficultyLabel(course.difficulty),
    },
    { key: 'semester', header: '学期' },
    { key: 'credits', header: '学分', align: 'right', mono: true },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (course) => formatShortDateTime(course.updated_at),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (course) => (
        <Button variant="ghost" size="sm" leftIcon={Copy} onClick={() => setCloneTarget(course)}>
          克隆课程
        </Button>
      ),
    },
  ]
  return (
    <>
      <DataPanel
        label="共享课程"
        filter={
          <FilterBar
            label="共享课程筛选"
            onSubmit={() => setSubmittedKeyword(keyword.trim())}
            submitLabel="搜索"
            onReset={
              keyword || submittedKeyword
                ? () => {
                    setKeyword('')
                    setSubmittedKeyword('')
                  }
                : undefined
            }
          >
            <FilterField label="课程关键词" htmlFor="shared-course-keyword">
              <Input
                id="shared-course-keyword"
                value={keyword}
                leftIcon={Search}
                placeholder="输入课程名称或说明"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={shared.page}
            pageSize={shared.pageSize}
            total={shared.total}
            onPageChange={shared.setPage}
          />
        }
      >
        <ResourceState
          resource={shared}
          emptyIcon={Share2}
          emptyTitle={submittedKeyword ? '没有匹配的共享课程' : '还没有共享课程'}
          emptyDescription={
            submittedKeyword
              ? '换个关键词再试,或清空筛选查看全部。'
              : '其他学校共享的课程会显示在这里。'
          }
          skeleton={
            <Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />
          }
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(course) => course.id}
              elevated={false}
              mobileCard={(course) => ({
                title: course.name,
                meta: `${courseTypeLabel(course.type)} · ${course.semester} · ${course.credits}学分`,
                action: (
                  <Button
                    variant="ghost"
                    size="sm"
                    leftIcon={Copy}
                    onClick={() => setCloneTarget(course)}
                  >
                    克隆课程
                  </Button>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>
      {cloneTarget ? (
        <CloneCourseModal
          key={cloneTarget.id}
          course={cloneTarget}
          onClose={() => setCloneTarget(undefined)}
          onCloned={(created) => navigate(`/teacher/courses/${created.id}`)}
        />
      ) : null}
    </>
  )
}
