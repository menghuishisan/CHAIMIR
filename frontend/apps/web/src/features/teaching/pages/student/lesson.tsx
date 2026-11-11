// 课时学习页(深页,/student/courses/:courseId/lessons/:lessonId)。设计 §7.2:
// 阅读形态 —— ≥xl 左侧课程目录(当前课时高亮、每节带状态图标、可收起)+ 中间限宽正文;
// 更窄时目录收进正文顶部「课程目录」按钮弹出的抽屉。正文顶部一行章节名与课时类型,
// 底部是「上一节 / 下一节」两张可点卡片(由课程目录计算)。
//
// 学习进度以服务端为准:进入页面先读服务端进度,上报后重新读取(FE-7)。
// 「学习中」自动上报:非视频课时在首次打开且服务端状态为未开始时,用现有上报接口报一次「学习中」
// (视频课时本来就由播放器上报);「标记为学完」对图文、资料、实验、仿真四类课时保留手动
// (实验完成不会自动更新课时进度,docs/06-教学/01-需求规格.md E1)。
//
// 五种课时形态各有确定链路(docs/06-教学/02-数据模型.md 的 content_ref 形状表):
//   图文   → content_ref.markdown 直接渲染
//   视频   → material/access 换 mode=stream 授权,播放器内播放并按 video_pos 续播,播完自动标记学完
//   附件   → 同一入口换 mode=download 授权,经统一文件服务取件
//   实验   → 直接链到 content_ref.experiment_id 对应的实验详情
//   仿真   → 直接链到 content_ref.package_code + version 的仿真工作台(课时仿真只作演示,不计分);
//            学校自建场景不能从浏览器直接进入,说明请在课程的实验任务里打开
// 材料地址一律经统一文件服务,页面不拼接对象存储地址。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  BookOpen,
  CheckCheck,
  ChevronLeft,
  ChevronRight,
  Download,
  FileText,
  FlaskConical,
  ListTree,
  PanelLeftClose,
  PanelLeftOpen,
  Radar,
} from 'lucide-react'
import {
  LessonContentType,
  ProgressStatus,
  type CourseOutline,
  type Lesson,
  type Progress,
} from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  Callout,
  Card,
  CardBody,
  cn,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  Icon,
  IconButton,
  PageScaffold,
  ReadingLayout,
  Skeleton,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePageLocation, locationLabel } from '../../../../hooks'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { fileKindLabel, formatDuration, formatFileSize } from '../../../../utils/formatters'
import { lessonContentTypeLabel, progressStatusLabel } from '../../../../utils/labels/teaching'
import { errorDiagnostics, userFacingErrorMessage } from '../../../../utils/userFacingError'
import { LESSON_TYPE_ICON, LessonStatusIcon } from '../../components/LessonIcons'
import { adjacentLessons, orderedLessons } from '../../learningPath'
import { isLessonMaterialType } from '../../rules'

/** 平台内置仿真场景的 code 前缀:内置场景在浏览器里运行,可从课时直接进入;
 *  学校自建场景在服务端隔离容器里运行,只能从课程的实验任务进入(与仿真工作台同一判定) */
const BUILTIN_SIM_PREFIX = 'builtin__'

/** 目录收起偏好:只是界面偏好,全站一个键 */
const TOC_COLLAPSED_KEY = 'chaimir.lesson_toc_collapsed'

/** readTocCollapsed 读取目录收起偏好;存储不可用时默认展开 */
function readTocCollapsed(): boolean {
  try {
    return window.localStorage.getItem(TOC_COLLAPSED_KEY) === 'true'
  } catch (error) {
    console.warn('lesson_toc_preference_read_failed', {
      operation: 'read_toc_collapsed',
      error: errorDiagnostics(error),
    })
    return false
  }
}

/** writeTocCollapsed 保存目录收起偏好;失败不影响当前交互 */
function writeTocCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(TOC_COLLAPSED_KEY, String(collapsed))
  } catch (error) {
    console.warn('lesson_toc_preference_write_failed', {
      operation: 'write_toc_collapsed',
      error: errorDiagnostics(error),
    })
  }
}

/** LessonView 是课时页的静态部分:课时本体 + 课程大纲(供目录)。进度另取,见下。 */
interface LessonView {
  lesson: Lesson
  outline: CourseOutline
}

/** 进度列表尚未读到时的占位:用同一个实例,自动上报据此判断「还不知道」而不是「未开始」 */
const EMPTY_PROGRESS: Progress[] = []

/** 视频播放位置回写节流:每 15 秒最多写一次,避免播放中高频打服务端。 */
const VIDEO_POSITION_REPORT_INTERVAL_MS = 15_000

/**
 * StudentLessonPage 呈现课时内容并上报学习进度。
 *
 * 取数分两份是有意的:
 *   静态部分(课时本体 + 大纲)进入页面读一次 —— 学习阅读族的骨架要一列目录(§6.5.3 第 ⑦),
 *   而大纲里的章节课时在页面停留期间不会变;
 *   进度单独走 `getMyProgress`,上报后只重取它 —— 一次「标记学完」不该把整份大纲再拉一遍。
 */
export default function StudentLessonPage() {
  const { courseId = '', lessonId = '' } = useParams<{ courseId: string; lessonId: string }>()

  const view = useAsyncResource<LessonView>(
    () =>
      Promise.all([api.teaching.getLesson(lessonId), api.teaching.getCourseOutline(courseId)]).then(
        ([lesson, outline]) => ({ lesson, outline })
      ),
    [courseId, lessonId],
    () => false
  )

  const progressList = useAsyncResource(
    () => api.teaching.getMyProgress(courseId),
    [courseId],
    () => false
  )

  usePageLocation([
    {
      label: locationLabel(view, (data) => data.outline.course.name, '课程详情'),
      to: `/student/courses/${courseId}`,
    },
    { label: locationLabel(view, (data) => data.lesson.title, '课时') },
  ])

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={BookOpen}
        emptyTitle="课时内容暂未开放"
        emptyDescription="老师还没有为这个课时设置内容。"
        skeleton={
          <div className="mx-auto flex w-full max-w-160 flex-col gap-4" aria-busy="true">
            <Skeleton variant="line" width="third" />
            <Skeleton variant="title" width="half" />
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => (
          <LessonContent
            courseId={courseId}
            view={data}
            progressList={progressList.data ?? EMPTY_PROGRESS}
            onProgressChanged={progressList.reload}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

interface LessonContentProps {
  courseId: string
  view: LessonView
  /** 本人在该课程各课时的进度;由页面单独取,上报后只重取这一份 */
  progressList: Progress[]
  onProgressChanged: () => void
}

/**
 * LessonContent 渲染目录、章节行、课时标题、正文、完成动作与上一节 / 下一节。
 */
function LessonContent({ courseId, view, progressList, onProgressChanged }: LessonContentProps) {
  const { lesson, outline } = view
  const progress = progressList.find((item) => item.lesson_id === lesson.id)
  const status = progress ? progress.status : ProgressStatus.NOT_STARTED
  const chapter = outline.chapters.find((item) => item.id === lesson.chapter_id)
  const [tocCollapsed, setTocCollapsed] = useState(readTocCollapsed)
  const [drawerOpen, setDrawerOpen] = useState(false)

  const path = useMemo(
    () => orderedLessons({ ...outline, progress: progressList }),
    [outline, progressList]
  )
  const adjacent = adjacentLessons(path, lesson.id)
  const lessonById = useMemo(
    () => new Map(outline.lessons.map((item) => [item.id, item])),
    [outline.lessons]
  )

  useAutoStartProgress(lesson, progress, progressList, onProgressChanged)

  const toggleToc = () => {
    setTocCollapsed((previous) => {
      writeTocCollapsed(!previous)
      return !previous
    })
  }

  return (
    <ReadingLayout
      toc={
        tocCollapsed ? undefined : (
          <div className="rounded-lg bg-surface p-3 shadow-xs">
            <div className="mb-2 flex items-center justify-between gap-2 px-2">
              <span className="text-sm font-semibold text-ink">课程目录</span>
              <IconButton
                icon={PanelLeftClose}
                size="sm"
                aria-label="收起课程目录"
                onClick={toggleToc}
              />
            </div>
            <LessonToc
              courseId={courseId}
              outline={outline}
              progressList={progressList}
              currentLessonId={lesson.id}
            />
          </div>
        )
      }
    >
      <div className="flex flex-col gap-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="flex min-w-0 items-center gap-2 text-sm text-ink-sub">
            <span className="truncate">{chapter?.title ?? '未分章'}</span>
            <span aria-hidden="true" className="text-ink-faint">
              ·
            </span>
            <span className="inline-flex shrink-0 items-center gap-1">
              <Icon icon={LESSON_TYPE_ICON[lesson.content_type]} size="xs" />
              {lessonContentTypeLabel(lesson.content_type)}
            </span>
          </p>
          <div className="flex items-center gap-1">
            {/* ≥xl 目录收起后的展开入口;更窄时一律走抽屉 */}
            {tocCollapsed ? (
              <Button
                variant="ghost"
                size="sm"
                leftIcon={PanelLeftOpen}
                onClick={toggleToc}
                className="hidden xl:inline-flex"
              >
                课程目录
              </Button>
            ) : null}
            <Button
              variant="ghost"
              size="sm"
              leftIcon={ListTree}
              onClick={() => setDrawerOpen(true)}
              className="xl:hidden"
            >
              课程目录
            </Button>
          </div>
        </div>

        <h1 className="text-2xl font-bold leading-tight text-ink md:text-3xl">{lesson.title}</h1>

        <LessonBody lesson={lesson} progress={progress} onProgressChanged={onProgressChanged} />

        <LessonCompletion
          lesson={lesson}
          status={status}
          progress={progress}
          nextHref={
            adjacent.next ? `/student/courses/${courseId}/lessons/${adjacent.next}` : undefined
          }
          onProgressChanged={onProgressChanged}
        />

        <nav aria-label="上一节与下一节" className="grid gap-3 sm:grid-cols-2">
          <AdjacentLessonCard
            courseId={courseId}
            direction="previous"
            lesson={adjacent.previous ? lessonById.get(adjacent.previous) : undefined}
          />
          <AdjacentLessonCard
            courseId={courseId}
            direction="next"
            lesson={adjacent.next ? lessonById.get(adjacent.next) : undefined}
          />
        </nav>
      </div>

      <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
        <DrawerContent side="bottom">
          <DrawerHeader>
            <DrawerTitle>课程目录</DrawerTitle>
            <DrawerDescription>选一节继续学习,已学完的课时带对勾。</DrawerDescription>
          </DrawerHeader>
          <DrawerBody>
            <LessonToc
              courseId={courseId}
              outline={outline}
              progressList={progressList}
              currentLessonId={lesson.id}
              onNavigate={() => setDrawerOpen(false)}
            />
          </DrawerBody>
        </DrawerContent>
      </Drawer>
    </ReadingLayout>
  )
}

/**
 * useAutoStartProgress 非视频课时首次打开且服务端状态为未开始时,报一次「学习中」(设计 §7.2)。
 * 只在进度列表已经读到之后判断(读取中不知道是不是未开始),同一节课只报一次;失败只记日志不打扰阅读。
 */
function useAutoStartProgress(
  lesson: Lesson,
  progress: Progress | undefined,
  progressList: Progress[],
  onProgressChanged: () => void
): void {
  const reportedFor = useRef<string | null>(null)
  const progressLoaded = progressList !== EMPTY_PROGRESS
  useEffect(() => {
    if (!progressLoaded || lesson.content_type === LessonContentType.VIDEO) return
    if (progress && progress.status !== ProgressStatus.NOT_STARTED) return
    if (reportedFor.current === lesson.id) return
    reportedFor.current = lesson.id
    api.teaching
      .reportProgress(lesson.id, {
        status: ProgressStatus.IN_PROGRESS,
        video_pos: 0,
        duration_sec: progress?.duration_sec ?? 0,
      })
      .then(onProgressChanged)
      .catch((error) => {
        console.error('课时开始学习状态上报失败', {
          operation: 'teaching.lesson.autoStart',
          error: errorDiagnostics(error),
        })
      })
  }, [lesson.content_type, lesson.id, onProgressChanged, progress, progressLoaded])
}

interface LessonTocProps {
  courseId: string
  outline: CourseOutline
  /** 各课时进度:与正文区同一份,上报后一起刷新 */
  progressList: Progress[]
  currentLessonId: string
  /** 抽屉里点了某节后关闭抽屉 */
  onNavigate?: () => void
}

/**
 * LessonToc 课程目录:按章分组,每节带状态图标,当前课时高亮;每一项是真实链接(可在新标签页打开)。
 * 课时顺序与课程详情的学习路径同一口径(章序 + 章内 sort)。
 */
function LessonToc({
  courseId,
  outline,
  progressList,
  currentLessonId,
  onNavigate,
}: LessonTocProps) {
  const progressByLesson = useMemo(
    () => new Map(progressList.map((item) => [item.lesson_id, item.status])),
    [progressList]
  )
  const chapters = useMemo(
    () =>
      [...outline.chapters]
        .sort((a, b) => a.sort - b.sort)
        .map((chapter) => ({
          chapter,
          lessons: outline.lessons
            .filter((item) => item.chapter_id === chapter.id)
            .sort((a, b) => a.sort - b.sort),
        })),
    [outline.chapters, outline.lessons]
  )

  return (
    <nav aria-label="课程目录" className="flex flex-col gap-3">
      {chapters.map((entry) =>
        entry.lessons.length === 0 ? null : (
          <div key={entry.chapter.id} className="flex flex-col gap-0.5">
            <p className="px-2 pb-1 text-xs font-medium text-ink-sub">{entry.chapter.title}</p>
            {entry.lessons.map((item) => {
              const isCurrent = item.id === currentLessonId
              const itemStatus = progressByLesson.get(item.id) ?? ProgressStatus.NOT_STARTED
              return (
                <Link
                  key={item.id}
                  to={`/student/courses/${courseId}/lessons/${item.id}`}
                  onClick={onNavigate}
                  aria-current={isCurrent ? 'page' : undefined}
                  className={cn(
                    'flex min-h-9 items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors duration-fast pointer-coarse:min-h-11',
                    'focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2',
                    isCurrent
                      ? 'bg-primary-soft font-semibold text-primary'
                      : 'text-ink-sub hover:bg-surface-hover hover:text-ink'
                  )}
                >
                  <LessonStatusIcon status={itemStatus} />
                  <span className="min-w-0 truncate">{item.title}</span>
                </Link>
              )
            })}
          </div>
        )
      )}
    </nav>
  )
}

/** AdjacentLessonCard 上一节 / 下一节可点卡片;没有时留一个同尺寸的空位,保持两栏对齐 */
function AdjacentLessonCard({
  courseId,
  direction,
  lesson,
}: {
  courseId: string
  direction: 'previous' | 'next'
  lesson: Lesson | undefined
}) {
  if (!lesson) return <div aria-hidden="true" className="hidden sm:block" />
  const next = direction === 'next'
  return (
    <Link
      to={`/student/courses/${courseId}/lessons/${lesson.id}`}
      className={cn(
        'pressable flex items-center gap-3 rounded-lg bg-surface px-4 py-3.5 shadow-xs transition-shadow duration-fast hover:shadow-sm',
        'focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
        next && 'sm:flex-row-reverse sm:text-right'
      )}
    >
      <Icon icon={next ? ChevronRight : ChevronLeft} size="md" className="shrink-0 text-ink-sub" />
      <span className="min-w-0 flex-1">
        <span className="block text-xs text-ink-sub">{next ? '下一节' : '上一节'}</span>
        <span className="block truncate text-base font-semibold text-ink">{lesson.title}</span>
      </span>
    </Link>
  )
}

interface LessonBodyProps {
  lesson: Lesson
  progress: Progress | undefined
  onProgressChanged: () => void
}

/**
 * LessonBody 按课时形态呈现内容。
 */
function LessonBody({ lesson, progress, onProgressChanged }: LessonBodyProps) {
  if (lesson.content_type === LessonContentType.MARKDOWN) {
    const markdown = lesson.content_ref.markdown
    if (markdown?.trim()) {
      return (
        <Card>
          <CardBody>
            {/* 正文按段落渲染:文本节点本身就是 XSS 边界(全站零 dangerouslySetInnerHTML),
                后端按原文入库、不做 HTML 实体转义,故这里拿到的就是老师写下的原文 */}
            <div className="flex flex-col gap-3 text-base leading-relaxed text-ink">
              {markdown
                .split('\n')
                .map((paragraph, index) =>
                  paragraph.trim() === '' ? null : <p key={index}>{paragraph}</p>
                )}
            </div>
          </CardBody>
        </Card>
      )
    }
    return (
      <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-12 text-center shadow-xs">
        <Icon icon={FileText} size="lg" className="text-ink-faint" />
        <p className="mt-3 text-md font-semibold text-ink">这一节还没有正文</p>
        <p className="mt-1 text-sm text-ink-sub">老师填写图文内容后会显示在这里。</p>
      </div>
    )
  }

  if (isLessonMaterialType(lesson.content_type)) {
    return (
      <LessonMaterial lesson={lesson} progress={progress} onProgressChanged={onProgressChanged} />
    )
  }

  if (lesson.content_type === LessonContentType.EXPERIMENT) {
    const experimentId = lesson.content_ref.experiment_id
    return (
      <Card>
        <CardBody className="flex flex-col gap-3">
          <p className="flex items-center gap-2 text-md font-semibold text-ink">
            <Icon icon={FlaskConical} size="md" className="text-primary" />
            这一节要动手做实验
          </p>
          <p className="text-sm text-ink-sub">
            实验环境与检查点在实验里统一开启和记录。做完后回到这里把这一节标记为学完。
          </p>
          <div>
            <Link
              to={experimentId ? `/student/experiments/${experimentId}` : '/student/experiments'}
              className={buttonVariants({ variant: 'primary' })}
            >
              <Icon icon={FlaskConical} size="sm" />
              去做实验
            </Link>
          </div>
        </CardBody>
      </Card>
    )
  }

  const packageCode = lesson.content_ref.package_code ?? ''
  const version = lesson.content_ref.version ?? ''
  const browserRunnable = packageCode.startsWith(BUILTIN_SIM_PREFIX)
  return (
    <Card>
      <CardBody className="flex flex-col gap-3">
        <p className="flex items-center gap-2 text-md font-semibold text-ink">
          <Icon icon={Radar} size="md" className="text-primary" />
          这一节要看仿真推演
        </p>
        {browserRunnable ? (
          <>
            <p className="text-sm text-ink-sub">
              课时里的推演只作演示,不计分。看完后回到这里把这一节标记为学完。
            </p>
            <div>
              <Link
                to={`/student/simulations/${encodeURIComponent(packageCode)}/workspace${version ? `?version=${encodeURIComponent(version)}` : ''}`}
                className={buttonVariants({ variant: 'primary' })}
              >
                <Icon icon={Radar} size="sm" />
                进入推演
              </Link>
            </div>
          </>
        ) : (
          <Callout tone="info" title="这个场景要从课程的实验任务里打开">
            它由学校自建,在学校服务器上运行,不能从浏览器直接进入。请在课程的实验任务里打开它。
          </Callout>
        )}
      </CardBody>
    </Card>
  )
}

interface LessonMaterialProps {
  lesson: Lesson
  progress: Progress | undefined
  onProgressChanged: () => void
}

/**
 * LessonMaterial 换取投放授权并按形态呈现视频或附件。
 * 授权是短时的,进入页面时换取一次;过期后由用户重新加载(不做后台静默续期,
 * 那会在页面停留时持续打服务端)。
 */
function LessonMaterial({ lesson, progress, onProgressChanged }: LessonMaterialProps) {
  const access = useAsyncResource(
    () => api.teaching.issueLessonMaterialAccess(lesson.id),
    [lesson.id],
    () => false
  )

  return (
    <ResourceState
      resource={access}
      emptyIcon={BookOpen}
      emptyTitle="这一节还没有上传材料"
      emptyDescription="老师上传视频或资料后就能在这里查看。"
      skeleton={<Skeleton variant="block" />}
    >
      {(grant) =>
        grant.mode === 'stream' ? (
          <LessonVideo
            lessonId={lesson.id}
            token={grant.token}
            fileName={grant.file_name}
            size={grant.size}
            startPosition={progress ? progress.video_pos : 0}
            onProgressChanged={onProgressChanged}
          />
        ) : (
          <LessonAttachment
            lessonId={lesson.id}
            fileName={grant.file_name}
            size={grant.size}
          />
        )
      }
    </ResourceState>
  )
}

interface LessonVideoProps {
  lessonId: string
  token: string
  fileName: string
  size: number
  startPosition: number
  onProgressChanged: () => void
}

/**
 * LessonVideo 播放课时视频并续播。
 * 播放位置按节流回写服务端(video_pos),播完自动标记学完 —— 这两件事都以服务端为权威,
 * 刷新后从服务端记录的位置继续,而不是从头开始。
 */
function LessonVideo({
  lessonId,
  token,
  fileName,
  size,
  startPosition,
  onProgressChanged,
}: LessonVideoProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const lastReportedRef = useRef(0)
  const [playbackError, setPlaybackError] = useState<string>()
  // 视频由浏览器自身发起请求,拿不到 Axios 拦截器,故用绝对地址 + 投放授权
  const src = useMemo(() => api.storage.streamUrl(token), [token])

  /** reportPosition 把当前播放位置写回服务端;完成态一并上报,让课时状态与观看进度一致。 */
  const reportPosition = useCallback(
    async (positionSec: number, status: ProgressStatus) => {
      try {
        await api.teaching.reportProgress(lessonId, {
          status,
          video_pos: Math.floor(positionSec),
          duration_sec: Math.floor(positionSec),
        })
        if (status === ProgressStatus.DONE) onProgressChanged()
      } catch (error) {
        // 位置回写失败不打断观看:下一次节流窗口或播完时会再写一次。
        // 技术原因进结构化日志(规范 §6.7 B),不弹窗打扰正在看视频的用户。
        console.error('课时视频播放位置回写失败', {
          operation: 'teaching.lesson.reportVideoPosition',
          reason: 'progress-report-failed',
          error: errorDiagnostics(error),
        })
      }
    },
    [lessonId, onProgressChanged]
  )

  /** 续播:元数据就绪后跳到服务端记录的位置(超出时长则从头播)。 */
  const handleLoadedMetadata = useCallback(() => {
    const video = videoRef.current
    if (!video || startPosition <= 0) return
    if (startPosition < video.duration) video.currentTime = startPosition
  }, [startPosition])

  /** 播放中按固定间隔回写位置,避免每一帧都打服务端。 */
  const handleTimeUpdate = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    const now = Date.now()
    if (now - lastReportedRef.current < VIDEO_POSITION_REPORT_INTERVAL_MS) return
    lastReportedRef.current = now
    void reportPosition(video.currentTime, ProgressStatus.IN_PROGRESS)
  }, [reportPosition])

  /** 播完即视为学完:这是最可靠的完成信号,不需要用户再点一次。 */
  const handleEnded = useCallback(() => {
    const video = videoRef.current
    void reportPosition(video ? video.duration : 0, ProgressStatus.DONE)
    toast.success('已看完这一节')
  }, [reportPosition])

  // 离开页面时写一次当前位置:否则最后一个节流窗口内的观看会丢
  useEffect(() => {
    const video = videoRef.current
    return () => {
      if (video && video.currentTime > 0 && !video.ended) {
        void reportPosition(video.currentTime, ProgressStatus.IN_PROGRESS)
      }
    }
  }, [reportPosition])

  return (
    <Card>
      <CardBody className="flex flex-col gap-3">
        {playbackError ? <Callout tone="danger">{playbackError}</Callout> : null}
        <video
          ref={videoRef}
          src={src}
          controls
          preload="metadata"
          className="w-full rounded-md bg-terminal"
          onLoadedMetadata={handleLoadedMetadata}
          onTimeUpdate={handleTimeUpdate}
          onEnded={handleEnded}
          onError={() => setPlaybackError('视频暂时无法播放,请刷新页面重新获取播放授权后重试。')}
        >
          {/* 不支持 video 元素的环境给出可读说明,而不是空白 */}
          你的浏览器暂不支持在页面内播放视频。
        </video>
        <p className="text-sm text-ink-sub">
          {fileName} · {formatFileSize(size)}
        </p>
        {startPosition > 0 ? (
          <p className="text-xs text-ink-sub">已从上次观看的位置继续播放。</p>
        ) : null}
      </CardBody>
    </Card>
  )
}

interface LessonAttachmentProps {
  lessonId: string
  fileName: string
  size: number
}

/**
 * LessonAttachment 呈现课时资料并提供取件。
 * 取件授权是一次性的:每次点击重新换取,不复用上一次的 token。
 */
function LessonAttachment({ lessonId, fileName, size }: LessonAttachmentProps) {
  const [downloading, setDownloading] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const handleDownload = useCallback(async () => {
    setDownloading(true)
    setActionError(undefined)
    try {
      // 一次性授权:重新换取而不是复用页面加载时那份,避免它已被消费或过期
      const grant = await api.teaching.issueLessonMaterialAccess(lessonId)
      const attachment = await api.storage.consumeGrant(grant.token)
      downloadAttachment(attachment)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '资料下载没有完成,请稍后重试。'))
    } finally {
      setDownloading(false)
    }
  }, [lessonId])

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-4 rounded-lg bg-surface p-4 shadow-xs">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-md bg-primary-soft text-primary">
          <Icon icon={FileText} size="md" />
        </span>
        <span className="min-w-0 flex-1">
          {/* 文件名可能很长:最多两行,完整名称在悬停提示里 */}
          <span className="line-clamp-2 break-all text-base font-semibold text-ink" title={fileName}>
            {fileName}
          </span>
          <span className="mt-0.5 block text-sm text-ink-sub">
            {[fileKindLabel(fileName), formatFileSize(size)].filter(Boolean).join(' · ')}
          </span>
        </span>
        <Button
          variant="secondary"
          leftIcon={Download}
          loading={downloading}
          onClick={() => void handleDownload()}
        >
          下载
        </Button>
      </div>
      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
    </div>
  )
}

interface LessonCompletionProps {
  lesson: Lesson
  status: ProgressStatus
  progress: Progress | undefined
  /** 下一节地址;学完后完成动作变成「下一节」 */
  nextHref: string | undefined
  onProgressChanged: () => void
}

/**
 * LessonCompletion 正文末尾的完成动作:「标记为学完」,标记后显示「已学完」与累计时长,动作变成「下一节」。
 * 视频播完自动标记(播放器上报),这里仍提供手动标记;实验课时写明「做完了,标记为学完」。
 * 进度保存在服务器上,刷新或换设备都不会丢。
 */
function LessonCompletion({
  lesson,
  status,
  progress,
  nextHref,
  onProgressChanged,
}: LessonCompletionProps) {
  const [submitting, setSubmitting] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const durationSec = progress ? progress.duration_sec : 0

  const markDone = useCallback(async () => {
    setSubmitting(true)
    setActionError(undefined)
    try {
      await api.teaching.reportProgress(lesson.id, {
        status: ProgressStatus.DONE,
        video_pos: progress ? progress.video_pos : 0,
        duration_sec: durationSec,
      })
      toast.success('已标记为学完')
      onProgressChanged()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '学习进度没有保存成功,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [durationSec, lesson.id, onProgressChanged, progress])

  const done = status === ProgressStatus.DONE
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-surface-hover px-4 py-3">
        <p className="flex items-center gap-2 text-sm text-ink-sub">
          <LessonStatusIcon status={status} />
          <span>
            {progressStatusLabel(status)}
            {durationSec > 0 ? ` · 已学 ${formatDuration(durationSec)}` : ''}
          </span>
        </p>
        {done ? (
          nextHref ? (
            <Link to={nextHref} className={buttonVariants({ variant: 'primary' })}>
              下一节
              <Icon icon={ChevronRight} size="sm" />
            </Link>
          ) : null
        ) : (
          <Button
            variant="primary"
            leftIcon={CheckCheck}
            loading={submitting}
            onClick={() => void markDone()}
          >
            {lesson.content_type === LessonContentType.EXPERIMENT
              ? '做完了,标记为学完'
              : '标记为学完'}
          </Button>
        )}
      </div>
      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
    </div>
  )
}
