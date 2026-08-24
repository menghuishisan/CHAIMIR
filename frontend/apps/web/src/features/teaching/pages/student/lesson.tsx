// 课时学习页(深页,/student/courses/:courseId/lessons/:lessonId)。
// 学习进度以服务端为准:进入页面先读服务端进度,上报后重新读取,不用本地状态冒充持久化(FE-7)。
//
// 五种课时形态各有确定链路(docs/06-教学/02-数据模型.md 的 content_ref 形状表):
//   图文   → content_ref.markdown 直接渲染
//   视频   → material/access 换 mode=stream 授权,播放器内播放并按 video_pos 续播
//   附件   → 同一入口换 mode=download 授权,经统一文件服务取件
//   实验   → 跳转实验实训(环境与检查点归 M7)
//   仿真   → 跳转仿真实验室(推演归 M4)
// 材料地址一律经统一文件服务,页面不拼接对象存储地址。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { BookOpen, CheckCheck, Download, FlaskConical, ListTree, Network } from 'lucide-react'
import {
  LessonContentType,
  ProgressStatus,
  type CourseOutline,
  type Lesson,
  type Progress,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  cn,
  DescriptionList,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  Icon,
  PageHeader,
  PageScaffold,
  ReadingLayout,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { formatDuration, formatFileSize, formatShortDateTime } from '../../../../utils/formatters'
import {
  lessonContentTypeLabel,
  progressStatusLabel,
} from '../../../../utils/labels/teaching'
import { errorDiagnostics, userFacingErrorMessage } from '../../../../utils/userFacingError'
import { isLessonMaterialType } from '../../rules'
import { progressStatusTone } from '../../statusPresentation'

/** LessonView 是课时页的静态部分:课时本体 + 课程大纲(供目录)。进度另取,见下。 */
interface LessonView {
  lesson: Lesson
  outline: CourseOutline
}

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
      Promise.all([
        api.teaching.getLesson(lessonId),
        api.teaching.getCourseOutline(courseId),
      ]).then(([lesson, outline]) => ({ lesson, outline })),
    [courseId, lessonId],
    () => false,
  )

  const progressList = useAsyncResource(
    () => api.teaching.getMyProgress(courseId),
    [courseId],
    () => false,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={BookOpen}
        emptyTitle="课时内容暂未开放"
        emptyDescription="老师还没有为这个课时设置内容。"
      >
        {(data) => (
          <LessonContent
            courseId={courseId}
            view={data}
            progressList={progressList.data ?? []}
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
 * LessonContent 渲染课时头部、正文与进度动作区。
 */
/**
 * LessonContent 渲染课时头部、目录、限宽正文与进度动作区。
 *
 * 归族:学习阅读族(规范 §6.5.3 第 ⑦)。全族唯一限宽正文的一族 ——
 * 读整段文字时行太长会让眼睛回到下一行时找错位置。骨架由 ReadingLayout 给出:
 * 左目录 + 限宽正文 + 右进度栏,`<lg` 目录收成一条触发器、进度栏排到正文之后。
 */
function LessonContent({ courseId, view, progressList, onProgressChanged }: LessonContentProps) {
  const { lesson, outline } = view
  const progress = progressList.find((item) => item.lesson_id === lesson.id)
  const status = progress ? progress.status : ProgressStatus.NOT_STARTED

  return (
    <>
      {/* 页头只出课程上下文与面包屑:课时名由正文区的标题承担,不在页头再说一遍(§6.5.0 通则 1) */}
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '我的课程', href: '/student/courses' },
              { label: outline.course.name, href: `/student/courses/${courseId}` },
            ]}
          />
        }
      />

      <ReadingLayout
        toc={
          <LessonToc
            courseId={courseId}
            outline={outline}
            progressList={progressList}
            currentLessonId={lesson.id}
          />
        }
        aside={
          <LessonProgressCard
            lessonId={lesson.id}
            progress={progress}
            onProgressChanged={onProgressChanged}
          />
        }
      >
        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <h1 className="min-w-0 font-display text-2xl text-ink">{lesson.title}</h1>
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              <Badge tone="neutral">{lessonContentTypeLabel(lesson.content_type)}</Badge>
              <StatusIndicator
                tone={progressStatusTone(status)}
                label={progressStatusLabel(status)}
              />
            </div>
          </div>
          <LessonBody lesson={lesson} progress={progress} onProgressChanged={onProgressChanged} />
        </div>
      </ReadingLayout>
    </>
  )
}

interface LessonTocProps {
  courseId: string
  outline: CourseOutline
  /** 各课时进度:与正文区同一份,上报后一起刷新 */
  progressList: Progress[]
  currentLessonId: string
}

/**
 * LessonToc 是学习阅读族的目录列(§6.5.3 第 ⑦)。
 *
 * `≥lg` 常驻左栏;`<lg` 收成一条「当前位置 + 本章目录」触发器,完整目录走 Drawer ——
 * 抽屉的焦点陷阱与 Esc 由 Drawer 保证,这里不重复实现(ReadingLayout 注释里的约定)。
 * 课时按章节顺序、章内按 sort 排;每条带学习状态点,读者能看出自己学到哪儿。
 */
function LessonToc({ courseId, outline, progressList, currentLessonId }: LessonTocProps) {
  const navigate = useNavigate()
  const [drawerOpen, setDrawerOpen] = useState(false)

  const progressByLesson = useMemo(
    () => new Map(progressList.map((item) => [item.lesson_id, item.status])),
    [progressList],
  )

  /** 章节顺序 + 章内 sort:与课程详情页同一排序口径,两处目录顺序必须一致 */
  const chapters = useMemo(
    () =>
      outline.chapters.map((chapter) => ({
        chapter,
        lessons: outline.lessons
          .filter((item) => item.chapter_id === chapter.id)
          .sort((a, b) => a.sort - b.sort),
      })),
    [outline.chapters, outline.lessons],
  )

  const ordered = useMemo(() => chapters.flatMap((entry) => entry.lessons), [chapters])
  const currentIndex = ordered.findIndex((item) => item.id === currentLessonId)

  const openLesson = (lessonId: string) => {
    setDrawerOpen(false)
    navigate(`/student/courses/${courseId}/lessons/${lessonId}`)
  }

  const tocList = (
    <nav aria-label="课程目录" className="flex flex-col gap-3">
      {chapters.map((entry) => (
        <div key={entry.chapter.id} className="flex flex-col gap-1">
          <p className="px-2 text-xs font-medium text-ink-sub">{entry.chapter.title}</p>
          {entry.lessons.map((item) => {
            const isCurrent = item.id === currentLessonId
            const itemStatus = progressByLesson.get(item.id) ?? ProgressStatus.NOT_STARTED
            return (
              <button
                key={item.id}
                type="button"
                aria-current={isCurrent ? 'true' : undefined}
                onClick={() => openLesson(item.id)}
                className={cn(
                  'hit-target flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm',
                  'transition-colors duration-fast ease-out',
                  isCurrent
                    ? 'bg-primary-soft font-medium text-primary'
                    : 'text-ink-sub hover:bg-surface-hover hover:text-ink',
                )}
              >
                <span className="min-w-0 truncate">{item.title}</span>
                {itemStatus === ProgressStatus.DONE ? (
                  <Icon icon={CheckCheck} size="sm" className="shrink-0 text-success" />
                ) : null}
              </button>
            )
          })}
        </div>
      ))}
    </nav>
  )

  return (
    <>
      {/* ≥lg:目录常驻左栏 */}
      <div className="hidden rounded-lg bg-surface p-3 shadow-xs lg:block">{tocList}</div>

      {/* <lg:一条当前位置 + 展开触发;完整目录进抽屉 */}
      <div className="flex items-center justify-between gap-3 rounded-lg bg-surface px-4 py-3 shadow-xs lg:hidden">
        <span className="min-w-0 truncate text-sm text-ink-sub tabular-nums">
          第 {currentIndex + 1} / {ordered.length} 节
        </span>
        <Button variant="ghost" size="sm" leftIcon={ListTree} onClick={() => setDrawerOpen(true)}>
          课程目录
        </Button>
      </div>

      <Drawer open={drawerOpen} onOpenChange={setDrawerOpen}>
        <DrawerContent side="bottom">
          <DrawerHeader>
            <DrawerTitle>课程目录</DrawerTitle>
            <DrawerDescription>选一节继续学习,已学完的课时右侧有对勾。</DrawerDescription>
          </DrawerHeader>
          <DrawerBody>{tocList}</DrawerBody>
        </DrawerContent>
      </Drawer>
    </>
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
  const navigate = useNavigate()

  if (lesson.content_type === LessonContentType.MARKDOWN) {
    const markdown = lesson.content_ref.markdown
    if (markdown?.trim()) {
      return (
        <Card>
          <CardBody>
            {/* 正文按段落渲染:文本节点本身就是 XSS 边界(全站零 dangerouslySetInnerHTML),
                后端按原文入库、不做 HTML 实体转义,故这里拿到的就是老师写下的原文 */}
            <div className="flex flex-col gap-3 text-base leading-relaxed text-ink">
              {markdown.split('\n').map((paragraph, index) =>
                paragraph.trim() === '' ? null : <p key={index}>{paragraph}</p>,
              )}
            </div>
          </CardBody>
        </Card>
      )
    }
    return (
      <Callout tone="info" title="这一节还没有正文">
        老师尚未填写图文内容,填写后会显示在这里。
      </Callout>
    )
  }

  if (isLessonMaterialType(lesson.content_type)) {
    return (
      <LessonMaterial lesson={lesson} progress={progress} onProgressChanged={onProgressChanged} />
    )
  }

  if (lesson.content_type === LessonContentType.EXPERIMENT) {
    return (
      <Card>
        <CardHeader
          title="这一节要动手做实验"
          description="实验环境与检查点在实验实训中统一开启和记录。"
        />
        <CardBody>
          <Button variant="primary" leftIcon={FlaskConical} onClick={() => navigate('/student/experiments')}>
            前往实验实训
          </Button>
        </CardBody>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader
        title="这一节要看仿真推演"
        description="仿真场景在仿真实验室里按版本选择后进入推演。"
      />
      <CardBody>
        <Button variant="primary" leftIcon={Network} onClick={() => navigate('/student/simulations')}>
          前往仿真实验室
        </Button>
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
    () => false,
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
            contentType={grant.content_type}
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
    [lessonId, onProgressChanged],
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
          onError={() =>
            setPlaybackError('视频暂时无法播放,请刷新页面重新获取播放授权后重试。')
          }
        >
          {/* 不支持 video 元素的环境给出可读说明,而不是空白 */}
          你的浏览器暂不支持在页面内播放视频。
        </video>
        <p className="text-sm text-ink-sub">
          {fileName} · {formatFileSize(size)}
        </p>
        {startPosition > 0 ? (
          <p className="text-xs text-ink-faint">已从上次观看的位置继续播放。</p>
        ) : null}
      </CardBody>
    </Card>
  )
}

interface LessonAttachmentProps {
  lessonId: string
  fileName: string
  size: number
  contentType: string
}

/**
 * LessonAttachment 呈现课时资料并提供取件。
 * 取件授权是一次性的:每次点击重新换取,不复用上一次的 token。
 */
function LessonAttachment({ lessonId, fileName, size, contentType }: LessonAttachmentProps) {
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
    <Card>
      <CardHeader title="课时资料" description="下载后即可离线查看。" />
      <CardBody className="flex flex-col gap-3">
        <DescriptionList
          dense
          items={[
            { term: '文件名', description: fileName },
            { term: '大小', description: formatFileSize(size), mono: true },
            { term: '类型', description: contentType, mono: true },
          ]}
        />
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
        <Button variant="primary" leftIcon={Download} loading={downloading} onClick={() => void handleDownload()}>
          下载资料
        </Button>
      </CardBody>
    </Card>
  )
}

interface LessonProgressCardProps {
  lessonId: string
  progress: Progress | undefined
  onProgressChanged: () => void
}

/**
 * LessonProgressCard 展示服务端进度并提供手动标记完成。
 * 视频课时由播放器自动上报,这里的手动标记服务于图文、资料与引擎类课时。
 */
function LessonProgressCard({ lessonId, progress, onProgressChanged }: LessonProgressCardProps) {
  const [submitting, setSubmitting] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const status = progress ? progress.status : ProgressStatus.NOT_STARTED
  const durationSec = progress ? progress.duration_sec : 0

  const items = useMemo(
    () => [
      { term: '学习状态', description: progressStatusLabel(status) },
      { term: '累计学习时长', description: formatDuration(durationSec), mono: true },
      {
        term: '最近学习',
        description: progress ? formatShortDateTime(progress.updated_at) : '尚未开始',
        mono: true,
      },
    ],
    [durationSec, progress, status],
  )

  const report = useCallback(
    async (next: ProgressStatus) => {
      setSubmitting(true)
      setActionError(undefined)
      try {
        await api.teaching.reportProgress(lessonId, {
          status: next,
          video_pos: progress ? progress.video_pos : 0,
          duration_sec: durationSec,
        })
        toast.success(next === ProgressStatus.DONE ? '已标记为学完' : '已记录学习状态')
        onProgressChanged()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '学习进度没有保存成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [durationSec, lessonId, onProgressChanged, progress],
  )

  return (
    <Card>
      <CardHeader title="学习进度" description="进度保存在服务器上,刷新或换设备都不会丢。" />
      <CardBody className="flex flex-col gap-4">
        <DescriptionList dense items={items} />
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
        {status === ProgressStatus.DONE ? (
          <Callout tone="success">这一节已完成。</Callout>
        ) : (
          <div className="flex flex-col gap-2">
            {status === ProgressStatus.NOT_STARTED ? (
              <Button
                variant="outline"
                loading={submitting}
                onClick={() => void report(ProgressStatus.IN_PROGRESS)}
              >
                标记为在学
              </Button>
            ) : null}
            <Button
              variant="primary"
              leftIcon={CheckCheck}
              loading={submitting}
              onClick={() => void report(ProgressStatus.DONE)}
            >
              标记为学完
            </Button>
          </div>
        )}
      </CardBody>
    </Card>
  )
}
