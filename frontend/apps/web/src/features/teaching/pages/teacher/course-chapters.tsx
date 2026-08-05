// 章节课时管理(课程详情页内区块)。
// 章节与课时是两级结构:章节只有标题与排序,课时挂在章节下并按 content_type 设置内容。
// 课时内容设置分五种形态,视频与附件走上传接口,其余按结构化字段填写 ——
// 不把 content_ref 作为裸 JSON 交给用户。

import { useCallback, useMemo, useState } from 'react'
import { FilePlus2, Layers, Pencil, Plus, Trash2, Upload } from 'lucide-react'
import {
  LessonContentType,
  type Chapter,
  type CourseOutline,
  type Lesson,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageSection,
  Select,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { formatFileSize } from '../../../../utils/formatters'
import {
  LESSON_CONTENT_TYPES,
  isLessonMaterialType,
  lessonContentTypeLabel,
} from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** content_ref 各形态的键,与 docs/06-教学/02-数据模型.md 的形状表一致。 */
const MARKDOWN_KEY = 'markdown'
const EXPERIMENT_KEY = 'experiment_id'
const SIM_CODE_KEY = 'package_code'
const SIM_VERSION_KEY = 'version'
const FILE_NAME_KEY = 'file_name'
const FILE_SIZE_KEY = 'size'

export interface CourseChaptersProps {
  courseId: string
  outline: CourseOutline
  onChanged: () => void
}

/**
 * CourseChapters 管理章节与课时。
 */
export function CourseChapters({ courseId, outline, onChanged }: CourseChaptersProps) {
  const [chapterModal, setChapterModal] = useState<{ chapter?: Chapter } | undefined>()
  const [lessonModal, setLessonModal] = useState<{ chapterId: string; lesson?: Lesson } | undefined>()
  const [deleteTarget, setDeleteTarget] = useState<
    { kind: 'chapter'; chapter: Chapter } | { kind: 'lesson'; chapterId: string; lesson: Lesson }
  >()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const lessonsByChapter = useMemo(() => {
    const map = new Map<string, Lesson[]>()
    for (const lesson of outline.lessons) {
      const list = map.get(lesson.chapter_id) ?? []
      list.push(lesson)
      map.set(lesson.chapter_id, list)
    }
    for (const list of map.values()) list.sort((a, b) => a.sort - b.sort)
    return map
  }, [outline.lessons])

  /** removeTarget 删除章节或课时:删除章节会连带其下课时,故先确认。 */
  const removeTarget = useCallback(async () => {
    if (!deleteTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      if (deleteTarget.kind === 'chapter') {
        await api.teaching.deleteChapter(courseId, deleteTarget.chapter.id)
        toast.success('章节已删除')
      } else {
        await api.teaching.deleteLesson(deleteTarget.chapterId, deleteTarget.lesson.id)
        toast.success('课时已删除')
      }
      setDeleteTarget(undefined)
      onChanged()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, deleteTarget, onChanged])

  const sortedChapters = useMemo(
    () => [...outline.chapters].sort((a, b) => a.sort - b.sort),
    [outline.chapters],
  )

  return (
    <PageSection
      title="章节课时"
      description="先建章节,再往章节下添加课时并设置内容。"
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setChapterModal({})}>
          新建章节
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        {sortedChapters.length === 0 ? (
          <Empty
            icon={Layers}
            title="还没有章节"
            description="课程内容按章节组织。先建一个章节,再往里添加课时。"
            action={
              <Button variant="primary" leftIcon={Plus} onClick={() => setChapterModal({})}>
                新建章节
              </Button>
            }
          />
        ) : (
          sortedChapters.map((chapter) => (
            <ChapterCard
              key={chapter.id}
              chapter={chapter}
              lessons={lessonsByChapter.get(chapter.id) ?? []}
              onEditChapter={() => setChapterModal({ chapter })}
              onDeleteChapter={() => setDeleteTarget({ kind: 'chapter', chapter })}
              onAddLesson={() => setLessonModal({ chapterId: chapter.id })}
              onEditLesson={(lesson) => setLessonModal({ chapterId: chapter.id, lesson })}
              onDeleteLesson={(lesson) =>
                setDeleteTarget({ kind: 'lesson', chapterId: chapter.id, lesson })
              }
            />
          ))
        )}
      </div>

      {chapterModal ? (
        <ChapterFormModal
          courseId={courseId}
          chapter={chapterModal.chapter}
          nextSort={sortedChapters.length + 1}
          onClose={() => setChapterModal(undefined)}
          onSaved={() => {
            setChapterModal(undefined)
            onChanged()
          }}
        />
      ) : null}

      {lessonModal ? (
        <LessonFormModal
          chapterId={lessonModal.chapterId}
          lesson={lessonModal.lesson}
          nextSort={(lessonsByChapter.get(lessonModal.chapterId)?.length ?? 0) + 1}
          onClose={() => setLessonModal(undefined)}
          onSaved={() => {
            setLessonModal(undefined)
            onChanged()
          }}
        />
      ) : null}

      <Modal open={deleteTarget !== undefined} onOpenChange={(open) => !open && setDeleteTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>
              {deleteTarget?.kind === 'chapter' ? '确认删除章节' : '确认删除课时'}
            </ModalTitle>
            <ModalDescription>
              {deleteTarget?.kind === 'chapter'
                ? '删除章节会同时删除它下面的全部课时,学生的学习记录将无法再关联到这些课时。'
                : '删除后学生看不到这个课时,已有的学习记录不再显示。'}
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">
              {deleteTarget?.kind === 'chapter' ? deleteTarget.chapter.title : deleteTarget?.lesson.title}
            </p>
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(undefined)}>
              取消
            </Button>
            <Button variant="danger" loading={working} onClick={() => void removeTarget()}>
              确认删除
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}

interface ChapterCardProps {
  chapter: Chapter
  lessons: Lesson[]
  onEditChapter: () => void
  onDeleteChapter: () => void
  onAddLesson: () => void
  onEditLesson: (lesson: Lesson) => void
  onDeleteLesson: (lesson: Lesson) => void
}

/**
 * ChapterCard 渲染单个章节及其课时表。
 */
function ChapterCard({
  chapter,
  lessons,
  onEditChapter,
  onDeleteChapter,
  onAddLesson,
  onEditLesson,
  onDeleteLesson,
}: ChapterCardProps) {
  const columns: TableColumn<Lesson>[] = [
    { key: 'sort', header: '序号', align: 'right', mono: true },
    {
      key: 'title',
      header: '课时',
      render: (lesson) => <span className="font-medium text-ink">{lesson.title}</span>,
    },
    {
      key: 'content_type',
      header: '内容形态',
      render: (lesson) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="neutral">{lessonContentTypeLabel(lesson.content_type)}</Badge>
          <LessonContentSummary lesson={lesson} />
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (lesson) => (
        <div className="flex justify-end gap-1">
          <IconButton
            variant="ghost"
            size="sm"
            icon={Pencil}
            aria-label={`编辑课时 ${lesson.title}`}
            onClick={() => onEditLesson(lesson)}
          />
          <IconButton
            variant="ghost"
            size="sm"
            icon={Trash2}
            aria-label={`删除课时 ${lesson.title}`}
            onClick={() => onDeleteLesson(lesson)}
          />
        </div>
      ),
    },
  ]

  return (
    <Card>
      <CardHeader
        title={chapter.title}
        description={`第 ${chapter.sort} 章 · ${lessons.length} 个课时`}
        actions={
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" leftIcon={FilePlus2} onClick={onAddLesson}>
              添加课时
            </Button>
            <IconButton
              variant="ghost"
              size="sm"
              icon={Pencil}
              aria-label={`编辑章节 ${chapter.title}`}
              onClick={onEditChapter}
            />
            <IconButton
              variant="ghost"
              size="sm"
              icon={Trash2}
              aria-label={`删除章节 ${chapter.title}`}
              onClick={onDeleteChapter}
            />
          </div>
        }
      />
      <CardBody>
        <Table
          columns={columns}
          data={lessons}
          rowKey={(lesson) => lesson.id}
          empty={
            <Empty
              icon={FilePlus2}
              title="这一章还没有课时"
              description="添加课时后可以设置视频、图文、资料、实验或仿真内容。"
              action={
                <Button variant="outline" size="sm" leftIcon={FilePlus2} onClick={onAddLesson}>
                  添加课时
                </Button>
              }
            />
          }
        />
      </CardBody>
    </Card>
  )
}

/**
 * LessonContentSummary 用一句话说明课时内容当前是什么。
 * 未设置内容的课时明确提示,避免教师以为已经配好了。
 */
function LessonContentSummary({ lesson }: { lesson: Lesson }) {
  const ref = lesson.content_ref

  if (isLessonMaterialType(lesson.content_type)) {
    const fileName = ref[FILE_NAME_KEY]
    const size = ref[FILE_SIZE_KEY]
    if (typeof fileName === 'string' && fileName !== '') {
      return (
        <span className="text-xs text-ink-sub">
          {fileName}
          {typeof size === 'number' ? ` · ${formatFileSize(size)}` : ''}
        </span>
      )
    }
    return <Badge tone="warning">未上传文件</Badge>
  }

  if (lesson.content_type === LessonContentType.MARKDOWN) {
    const markdown = ref[MARKDOWN_KEY]
    return typeof markdown === 'string' && markdown.trim() !== '' ? (
      <span className="text-xs text-ink-sub">已填写正文</span>
    ) : (
      <Badge tone="warning">未填写正文</Badge>
    )
  }

  if (lesson.content_type === LessonContentType.EXPERIMENT) {
    const experimentId = ref[EXPERIMENT_KEY]
    return typeof experimentId === 'string' && experimentId !== '' ? (
      <span className="text-xs text-ink-sub">已关联实验</span>
    ) : (
      <Badge tone="warning">未关联实验</Badge>
    )
  }

  const packageCode = ref[SIM_CODE_KEY]
  const version = ref[SIM_VERSION_KEY]
  return typeof packageCode === 'string' && packageCode !== '' ? (
    <span className="text-xs text-ink-sub">
      {packageCode}
      {typeof version === 'string' ? ` · ${version}` : ''}
    </span>
  ) : (
    <Badge tone="warning">未关联仿真场景</Badge>
  )
}

interface ChapterFormModalProps {
  courseId: string
  chapter?: Chapter
  nextSort: number
  onClose: () => void
  onSaved: () => void
}

/**
 * ChapterFormModal 承载章节创建与编辑。
 */
function ChapterFormModal({ courseId, chapter, nextSort, onClose, onSaved }: ChapterFormModalProps) {
  const [title, setTitle] = useState(chapter?.title ?? '')
  const [sort, setSort] = useState(String(chapter?.sort ?? nextSort))
  const [fieldError, setFieldError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (title.trim() === '') {
        setFieldError('请输入章节标题')
        return
      }
      setFieldError(undefined)
      setWorking(true)
      try {
        const payload = { title: title.trim(), sort: Number(sort) || nextSort }
        if (chapter) {
          await api.teaching.updateChapter(courseId, chapter.id, payload)
          toast.success('章节已更新')
        } else {
          await api.teaching.createChapter(courseId, payload)
          toast.success('章节已创建')
        }
        onSaved()
      } catch (error) {
        setFieldError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [chapter, courseId, nextSort, onSaved, sort, title],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>{chapter ? '编辑章节' : '新建章节'}</ModalTitle>
          <ModalDescription>章节是课时的分组,按序号排列展示给学生。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="章节标题" htmlFor="chapter-title" required error={fieldError}>
              <Input
                id="chapter-title"
                value={title}
                invalid={Boolean(fieldError)}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>
            <FormField label="排列序号" htmlFor="chapter-sort" helper="数字越小越靠前">
              <Input
                id="chapter-sort"
                type="number"
                min="1"
                value={sort}
                onChange={(event) => setSort(event.target.value)}
              />
            </FormField>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              {chapter ? '保存修改' : '创建章节'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface LessonFormModalProps {
  chapterId: string
  lesson?: Lesson
  nextSort: number
  onClose: () => void
  onSaved: () => void
}

/**
 * LessonFormModal 承载课时创建、编辑与内容设置。
 * 内容按形态渲染显式字段:视频/资料走文件上传,图文填正文,实验/仿真填引用。
 */
function LessonFormModal({ chapterId, lesson, nextSort, onClose, onSaved }: LessonFormModalProps) {
  const [title, setTitle] = useState(lesson?.title ?? '')
  const [sort, setSort] = useState(String(lesson?.sort ?? nextSort))
  const [contentType, setContentType] = useState(
    String(lesson?.content_type ?? LessonContentType.MARKDOWN),
  )
  const [markdown, setMarkdown] = useState(readString(lesson, MARKDOWN_KEY))
  const [experimentId, setExperimentId] = useState(readString(lesson, EXPERIMENT_KEY))
  const [simCode, setSimCode] = useState(readString(lesson, SIM_CODE_KEY))
  const [simVersion, setSimVersion] = useState(readString(lesson, SIM_VERSION_KEY))
  const [file, setFile] = useState<File>()
  const [uploadProgress, setUploadProgress] = useState<number>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const typeValue = Number(contentType) as LessonContentType
  const isMaterial = isLessonMaterialType(typeValue)

  const contentTypeOptions = useMemo(
    () =>
      LESSON_CONTENT_TYPES.map((value) => ({
        value: String(value),
        label: lessonContentTypeLabel(value),
      })),
    [],
  )

  /** buildContentRef 按形态组装 content_ref,视频与资料由上传接口写入,不在此构造。 */
  const buildContentRef = useCallback((): Record<string, unknown> => {
    if (typeValue === LessonContentType.MARKDOWN) return { [MARKDOWN_KEY]: markdown }
    if (typeValue === LessonContentType.EXPERIMENT) return { [EXPERIMENT_KEY]: experimentId.trim() }
    if (typeValue === LessonContentType.SIMULATION) {
      return { [SIM_CODE_KEY]: simCode.trim(), [SIM_VERSION_KEY]: simVersion.trim() }
    }
    // 视频/资料:保留服务端已写入的引用,由上传接口更新
    return lesson?.content_ref ?? {}
  }, [experimentId, lesson?.content_ref, markdown, simCode, simVersion, typeValue])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (title.trim() === '') {
        setFormError('请输入课时标题')
        return
      }
      if (typeValue === LessonContentType.SIMULATION && simCode.trim() === '') {
        setFormError('请填写仿真场景标识')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload = {
          title: title.trim(),
          content_type: typeValue,
          content_ref: buildContentRef(),
          sort: Number(sort) || nextSort,
        }
        const saved = lesson
          ? await api.teaching.updateLesson(chapterId, lesson.id, payload)
          : await api.teaching.createLesson(chapterId, payload)

        // 选了文件就接着上传:上传接口会按文件类型置 content_type 并写入引用
        if (isMaterial && file) {
          await api.teaching.uploadLessonMaterial(saved.id, file, setUploadProgress)
        }
        toast.success(lesson ? '课时已更新' : '课时已创建')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
        setUploadProgress(undefined)
      }
    },
    [buildContentRef, chapterId, file, isMaterial, lesson, nextSort, onSaved, simCode, sort, title, typeValue],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{lesson ? '编辑课时' : '添加课时'}</ModalTitle>
          <ModalDescription>选择内容形态后填写对应内容,学生按这个形态学习。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="课时标题" htmlFor="lesson-title" required>
              <Input
                id="lesson-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="内容形态" htmlFor="lesson-type" required>
                <Select
                  id="lesson-type"
                  options={contentTypeOptions}
                  value={contentType}
                  onValueChange={setContentType}
                />
              </FormField>
              <FormField label="排列序号" htmlFor="lesson-sort" helper="数字越小越靠前">
                <Input
                  id="lesson-sort"
                  type="number"
                  min="1"
                  value={sort}
                  onChange={(event) => setSort(event.target.value)}
                />
              </FormField>
            </div>

            {typeValue === LessonContentType.MARKDOWN ? (
              <FormField label="图文正文" htmlFor="lesson-markdown" helper="支持分段,按段落展示给学生">
                <Textarea
                  id="lesson-markdown"
                  value={markdown}
                  rows={8}
                  onChange={(event) => setMarkdown(event.target.value)}
                />
              </FormField>
            ) : null}

            {typeValue === LessonContentType.EXPERIMENT ? (
              <FormField
                label="关联实验"
                htmlFor="lesson-experiment"
                helper="填写实验编排里的实验标识,学生从课时跳转到实验实训"
              >
                <Input
                  id="lesson-experiment"
                  value={experimentId}
                  onChange={(event) => setExperimentId(event.target.value)}
                />
              </FormField>
            ) : null}

            {typeValue === LessonContentType.SIMULATION ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField label="仿真场景标识" htmlFor="lesson-sim-code" required>
                  <Input
                    id="lesson-sim-code"
                    value={simCode}
                    onChange={(event) => setSimCode(event.target.value)}
                  />
                </FormField>
                <FormField label="场景版本" htmlFor="lesson-sim-version">
                  <Input
                    id="lesson-sim-version"
                    value={simVersion}
                    onChange={(event) => setSimVersion(event.target.value)}
                  />
                </FormField>
              </div>
            ) : null}

            {isMaterial ? (
              <FormField
                label={typeValue === LessonContentType.VIDEO ? '视频文件' : '资料文件'}
                htmlFor="lesson-file"
                helper={
                  typeValue === LessonContentType.VIDEO
                    ? '支持 mp4、webm;学生可在页面内播放并自动续播'
                    : '支持 pdf、图片、文档等;学生可直接下载'
                }
              >
                <Input
                  id="lesson-file"
                  type="file"
                  accept={typeValue === LessonContentType.VIDEO ? 'video/mp4,video/webm' : undefined}
                  onChange={(event) => setFile(event.target.files?.[0])}
                />
              </FormField>
            ) : null}

            {uploadProgress !== undefined ? (
              <Callout tone="info">文件上传中 {uploadProgress}%,请不要关闭窗口。</Callout>
            ) : null}

            {isMaterial && !file && lesson ? (
              <Callout tone="info">
                <span className="flex items-start gap-2">
                  <Upload aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
                  <span>不选文件则保留原有文件,只更新标题与排序。</span>
                </span>
              </Callout>
            ) : null}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" loading={working}>
              {lesson ? '保存修改' : '创建课时'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** readString 从课时内容引用里读字符串字段;非字符串回空串(不把对象塞进文本控件)。 */
function readString(lesson: Lesson | undefined, key: string): string {
  const value = lesson?.content_ref?.[key]
  return typeof value === 'string' ? value : ''
}
