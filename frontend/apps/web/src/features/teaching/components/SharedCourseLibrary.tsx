// SharedCourseLibrary 共享资源库「课程」标签(设计 §7.4 /teacher/shared):搜索 + 卡片网格,整卡打开预览抽屉
// (简介、类型、难度、学期与学分),抽屉底部「克隆到本校」;克隆成功后在抽屉里给「去编辑」链接。
// 共享目录是有限投影(不含邀请码与学生数据);预览写章节数,章节内容要克隆后才看得到。
// 搜索词与页码由页面写进地址。克隆表单复用 CloneCourseModal(与「我的课程」共用)。

import { useState } from 'react'
import { Link } from 'react-router'
import { Copy, Share2 } from 'lucide-react'
import type { Course, SharedCourse } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  DescriptionList,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  Icon,
  Pagination,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceError } from '../../../components/ResourceState'
import { SearchBox } from '../../../components/SearchBox'
import { usePagedResource } from '../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../utils/formatters'
import { courseTypeLabel, teachingDifficultyLabel } from '../../../utils/labels/teaching'
import { CloneCourseModal } from './CloneCourseModal'

/** 每页卡片数 */
const CARD_PAGE_SIZE = 12

export interface SharedCourseLibraryProps {
  /** 地址里当前生效的搜索词 */
  keyword: string
  onSearch: (value: string) => void
}

/** SharedCourseLibrary 以服务端共享目录为权威,失败态不显示为合法空目录。 */
export function SharedCourseLibrary({ keyword, onSearch }: SharedCourseLibraryProps) {
  const [previewTarget, setPreviewTarget] = useState<SharedCourse>()
  const shared = usePagedResource<SharedCourse>(
    (params) => api.teaching.getSharedCourses({ ...params, keyword: keyword || undefined }),
    [keyword],
    { urlPageKey: 'page', pageSize: CARD_PAGE_SIZE },
  )

  return (
    <>
      <div className="mb-5 flex justify-end">
        <SearchBox label="搜索共享课程" placeholder="搜索课程名称或说明" value={keyword} onSearch={onSearch} />
      </div>

      {shared.status === 'error' ? (
        <ResourceError
          message={shared.error?.message ?? '共享课程暂时读取不到'}
          traceId={shared.error?.traceId}
          onRetry={shared.reload}
        />
      ) : shared.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Share2} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{keyword ? '没有匹配的共享课程' : '还没有共享课程'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {keyword ? '换个关键词再试,或清空搜索查看全部。' : '其他学校共享的课程会显示在这里。'}
          </p>
          {keyword ? (
            <Button variant="outline" className="mt-5" onClick={() => onSearch('')}>
              清空搜索
            </Button>
          ) : null}
        </div>
      ) : shared.status === 'loading' && !shared.data ? (
        <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取共享课程">
          {Array.from({ length: 3 }, (_, index) => (
            <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
              <Skeleton variant="title" width="half" />
              <Skeleton variant="line" lines={2} />
            </li>
          ))}
        </ul>
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={shared.status === 'loading' || undefined}>
            {(shared.data?.list ?? []).map((course) => (
              <li key={course.id}>
                <SharedCourseCard course={course} onOpen={() => setPreviewTarget(course)} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={shared.page} pageSize={shared.pageSize} total={shared.total} onPageChange={shared.setPage} />
          </div>
        </>
      )}

      {previewTarget ? <SharedCourseDrawer course={previewTarget} onClose={() => setPreviewTarget(undefined)} /> : null}
    </>
  )
}

/** SharedCourseCard 一门共享课程:课程名(整卡按钮)、类型与难度、简介、学期与学分 */
function SharedCourseCard({ course, onOpen }: { course: SharedCourse; onOpen: () => void }) {
  return (
    <Card interactive className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-3">
        <button type="button" onClick={onOpen} className="card-link line-clamp-2 text-left text-lg font-bold text-ink">
          {course.name}
        </button>
        <Badge tone="neutral" className="shrink-0">
          {courseTypeLabel(course.type)}
        </Badge>
      </div>
      {course.description ? <p className="line-clamp-2 text-sm text-ink-sub">{course.description}</p> : null}
      <p className="mt-auto pt-3 text-sm text-ink-sub">
        {teachingDifficultyLabel(course.difficulty)} · {course.semester} · {course.credits} 学分
      </p>
    </Card>
  )
}

/**
 * SharedCourseDrawer 预览一门共享课程,底部「克隆到本校」;克隆成功后给「去编辑」链接(进入新课程详情),
 * 留在资源库里可以继续挑。
 */
function SharedCourseDrawer({ course, onClose }: { course: SharedCourse; onClose: () => void }) {
  const [cloneOpen, setCloneOpen] = useState(false)
  const [cloned, setCloned] = useState<Course>()

  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>{course.name}</DrawerTitle>
          <DrawerDescription>克隆会复制章节课时与作业设置,不复制学生、提交与成绩。</DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-5">
          {cloned ? (
            <Callout tone="success" title="已克隆为本校课程草稿">
              <p>
                「{cloned.name}」。{' '}
                <Link to={`/teacher/courses/${cloned.id}`} className="font-medium text-primary underline-offset-2 hover:underline">
                  去编辑
                </Link>
              </p>
            </Callout>
          ) : null}
          <section aria-label="课程简介">
            <h3 className="text-sm font-semibold text-ink">简介</h3>
            <p className="mt-1.5 whitespace-pre-wrap break-words text-sm text-ink">{course.description || '这门课没有填写简介。'}</p>
          </section>
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '课程类型', description: courseTypeLabel(course.type) },
              { term: '难度', description: teachingDifficultyLabel(course.difficulty) },
              { term: '学期', description: course.semester },
              { term: '学分', description: `${course.credits} 学分` },
              { term: '章节', description: course.chapter_count > 0 ? `共 ${course.chapter_count} 章` : '还没有章节' },
              {
                term: '更新于',
                description: <span title={formatDateTime(course.updated_at)}>{formatRelativeOccurrence(course.updated_at)}</span>,
              },
            ]}
          />
        </DrawerBody>
        <DrawerFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          <Button variant="primary" leftIcon={Copy} onClick={() => setCloneOpen(true)}>
            {cloned ? '再克隆一份' : '克隆到本校'}
          </Button>
        </DrawerFooter>
      </DrawerContent>
      {cloneOpen ? (
        <CloneCourseModal
          key={course.id}
          course={course}
          onClose={() => setCloneOpen(false)}
          onCloned={(created) => {
            setCloneOpen(false)
            setCloned(created)
          }}
        />
      ) : null}
    </Drawer>
  )
}
