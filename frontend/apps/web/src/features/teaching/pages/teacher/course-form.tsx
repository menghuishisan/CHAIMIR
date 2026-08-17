// 课程表单弹层(课程管理页与课程详情页共用)。
// 创建与编辑共用同一表单:后端 CourseRequest 两者字段完全一致,分两套表单必然漂移。
//
// 课程表(schedule)是 JSONB 开放对象。这里不把它作为裸 JSON 文本域交给用户,
// 而是渲染「星期 + 时间段」的显式字段(旧前端 16 处裸 JSON 是被审查列为 P0 的问题)。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  CourseType,
  TeachingDifficulty,
  type Course,
  type CourseRequest,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  CoverImage,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Select,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import {
  COURSE_TYPES,
  TEACHING_DIFFICULTIES,
  courseTypeCover,
  courseTypeLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ImageUploadField, readImageDataUrl } from '../../../../components/ImageUploadField'
import { useCourseCoverSrc } from '../../useCourseCoverSrc'

/** 星期取值与显示名:课程表用它构造结构化的上课时间,不让用户手写 JSON。 */
const WEEKDAYS = [
  { value: 'mon', label: '周一' },
  { value: 'tue', label: '周二' },
  { value: 'wed', label: '周三' },
  { value: 'thu', label: '周四' },
  { value: 'fri', label: '周五' },
  { value: 'sat', label: '周六' },
  { value: 'sun', label: '周日' },
] as const

/** 课程表在 schedule 里的键:与后端约定的结构化形状对应。 */
const SCHEDULE_WEEKDAY_KEY = 'weekday'
const SCHEDULE_PERIOD_KEY = 'period'

export interface CourseFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  course?: Course
  onClose: () => void
  onSaved: () => void
}

/**
 * CourseFormModal 承载课程创建与编辑。
 */
export function CourseFormModal({ course, onClose, onSaved }: CourseFormModalProps) {
  const fieldId = useId()
  const editing = course !== undefined

  const [name, setName] = useState(course?.name ?? '')
  const [description, setDescription] = useState(course?.description ?? '')
  const [type, setType] = useState(String(course?.type ?? CourseType.MIXED))
  const [difficulty, setDifficulty] = useState(String(course?.difficulty ?? TeachingDifficulty.INTRO))
  const [semester, setSemester] = useState(course?.semester ?? '')
  const [credits, setCredits] = useState(String(course?.credits ?? '2'))
  const [weekday, setWeekday] = useState(readScheduleString(course, SCHEDULE_WEEKDAY_KEY, 'mon'))
  const [period, setPeriod] = useState(readScheduleString(course, SCHEDULE_PERIOD_KEY, ''))
  const [startAt, setStartAt] = useState(toDateInput(course?.start_at))
  const [endAt, setEndAt] = useState(toDateInput(course?.end_at))
  const [coverRef, setCoverRef] = useState(course?.cover_ref ?? '')
  const [coverPreview, setCoverPreview] = useState('')
  // 编辑已有封面时换一次投放授权用于预览;本次刚选的图优先,清空后两者都不显示。
  const savedCoverSrc = useCourseCoverSrc(course?.id, course?.cover_ref)
  const coverSrc = coverPreview !== '' ? coverPreview : coverRef !== '' ? savedCoverSrc : ''

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  /** validate 校验必填与取值范围,返回是否通过并把错误留在原位。 */
  const validate = useCallback((): boolean => {
    const next: Record<string, string | null> = {
      name: name.trim() === '' ? '请输入课程名称' : null,
      semester: semester.trim() === '' ? '请输入学期,例如 2026 春' : null,
      credits:
        Number.isFinite(Number(credits)) && Number(credits) > 0 ? null : '学分需要是大于 0 的数字',
      startAt: startAt === '' ? '请选择开课日期' : null,
      endAt:
        endAt === ''
          ? '请选择结课日期'
          : startAt !== '' && endAt < startAt
            ? '结课日期不能早于开课日期'
            : null,
    }
    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [credits, endAt, name, semester, startAt])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      setSubmitting(true)
      setFormError(undefined)
      const payload: CourseRequest = {
        name: name.trim(),
        description: description.trim(),
        type: Number(type) as CourseType,
        difficulty: Number(difficulty) as TeachingDifficulty,
        semester: semester.trim(),
        credits: Number(credits),
        // 课程表按结构化字段组装,不接受用户手写 JSON
        schedule: { [SCHEDULE_WEEKDAY_KEY]: weekday, [SCHEDULE_PERIOD_KEY]: period.trim() },
        start_at: toIsoStart(startAt),
        end_at: toIsoEnd(endAt),
        // 留空即不设封面:传空串会把「没设过」和「设了个空值」混成一种状态
        cover_ref: coverRef === '' ? undefined : coverRef,
      }
      try {
        if (editing) {
          await api.teaching.updateCourse(course.id, payload)
          toast.success('课程已更新')
        } else {
          await api.teaching.createCourse(payload)
          toast.success('课程已创建为草稿')
        }
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(error, editing ? '课程更新失败,请稍后重试。' : '课程创建失败,请稍后重试。'),
        )
      } finally {
        setSubmitting(false)
      }
    },
    [
      course?.id,
      coverRef,
      credits,
      description,
      difficulty,
      editing,
      endAt,
      name,
      onSaved,
      period,
      semester,
      startAt,
      type,
      validate,
      weekday,
    ],
  )

  const typeOptions = useMemo(
    () => COURSE_TYPES.map((value) => ({ value: String(value), label: courseTypeLabel(value) })),
    [],
  )
  const difficultyOptions = useMemo(
    () =>
      TEACHING_DIFFICULTIES.map((value) => ({
        value: String(value),
        label: teachingDifficultyLabel(value),
      })),
    [],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑课程' : '新建课程'}</ModalTitle>
          <ModalDescription>
            {editing
              ? '修改课程基础信息。已发布课程的部分字段可能受后端状态限制。'
              : '新建后为草稿状态,补齐章节课时再发布给学生。'}
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="课程名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
              <Input
                id={`${fieldId}-name`}
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => setErrors((prev) => ({ ...prev, name: name.trim() === '' ? '请输入课程名称' : null }))}
              />
            </FormField>

            <FormField label="课程简介" htmlFor={`${fieldId}-desc`} helper="向学生说明这门课学什么">
              <Textarea
                id={`${fieldId}-desc`}
                value={description}
                rows={3}
                onChange={(event) => setDescription(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="课程形式" htmlFor={`${fieldId}-type`} required>
                <Select id={`${fieldId}-type`} options={typeOptions} value={type} onValueChange={setType} />
              </FormField>
              <FormField label="难度" htmlFor={`${fieldId}-difficulty`} required>
                <Select
                  id={`${fieldId}-difficulty`}
                  options={difficultyOptions}
                  value={difficulty}
                  onValueChange={setDifficulty}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="学期"
                htmlFor={`${fieldId}-semester`}
                required
                error={errors.semester}
                helper="例如 2026 春"
              >
                <Input
                  id={`${fieldId}-semester`}
                  value={semester}
                  invalid={Boolean(errors.semester)}
                  onChange={(event) => setSemester(event.target.value)}
                />
              </FormField>
              <FormField label="学分" htmlFor={`${fieldId}-credits`} required error={errors.credits}>
                <Input
                  id={`${fieldId}-credits`}
                  type="number"
                  min="0.5"
                  step="0.5"
                  value={credits}
                  invalid={Boolean(errors.credits)}
                  onChange={(event) => setCredits(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="上课日" htmlFor={`${fieldId}-weekday`}>
                <Select
                  id={`${fieldId}-weekday`}
                  options={WEEKDAYS.map((day) => ({ value: day.value, label: day.label }))}
                  value={weekday}
                  onValueChange={setWeekday}
                />
              </FormField>
              <FormField
                label="上课时间"
                htmlFor={`${fieldId}-period`}
                helper="例如 第 3-4 节 或 14:00-15:40"
              >
                <Input
                  id={`${fieldId}-period`}
                  value={period}
                  onChange={(event) => setPeriod(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="开课日期" htmlFor={`${fieldId}-start`} required error={errors.startAt}>
                <Input
                  id={`${fieldId}-start`}
                  type="date"
                  value={startAt}
                  invalid={Boolean(errors.startAt)}
                  onChange={(event) => setStartAt(event.target.value)}
                />
              </FormField>
              <FormField label="结课日期" htmlFor={`${fieldId}-end`} required error={errors.endAt}>
                <Input
                  id={`${fieldId}-end`}
                  type="date"
                  value={endAt}
                  invalid={Boolean(errors.endAt)}
                  onChange={(event) => setEndAt(event.target.value)}
                />
              </FormField>
            </div>

            <FormField
              label="课程封面"
              htmlFor={`${fieldId}-cover`}
              helper="留空时按课程形式自动配一张。"
            >
              <ImageUploadField
                inputId={`${fieldId}-cover`}
                hasImage={coverRef !== ''}
                failureMessage="封面这次没有更新成功,请稍后重试。"
                preview={
                  <CoverImage
                    id={course?.id ?? ''}
                    coverSrc={coverSrc}
                    name={name || '课程'}
                    glyph={courseTypeCover(Number(type) as CourseType).glyph}
                    accent={courseTypeCover(Number(type) as CourseType).accent}
                    className="w-40 shrink-0"
                  />
                }
                onUpload={async (file, onProgress) => {
                  const uploaded = await api.teaching.uploadCourseCover(file, onProgress)
                  setCoverRef(uploaded.object_ref)
                  // 此刻封面还只在暂存位、课程也可能还不存在,拿不到投放地址,先用本地文件预览
                  setCoverPreview(await readImageDataUrl(file))
                }}
                onCleared={() => {
                  setCoverRef('')
                  setCoverPreview('')
                }}
              />
            </FormField>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存修改' : '创建课程'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/**
 * readScheduleString 从课程表对象里读一个字符串字段。
 * schedule 是开放 JSONB:只接受字符串值,其他类型回默认值 —— 不把对象塞进文本控件。
 */
function readScheduleString(course: Course | undefined, key: string, fallback: string): string {
  const value = course?.schedule?.[key]
  return typeof value === 'string' ? value : fallback
}

/** toDateInput 把后端时间转成 date 控件需要的 YYYY-MM-DD;无法解析时回空串。 */
function toDateInput(value: string | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 10)
}

/** toIsoStart 把日期转成当天开始时刻的 ISO 串(后端按 RFC3339 解析)。 */
function toIsoStart(value: string): string {
  return new Date(`${value}T00:00:00`).toISOString()
}

/** toIsoEnd 把日期转成当天结束时刻的 ISO 串,保证结课日整天有效。 */
function toIsoEnd(value: string): string {
  return new Date(`${value}T23:59:59`).toISOString()
}
