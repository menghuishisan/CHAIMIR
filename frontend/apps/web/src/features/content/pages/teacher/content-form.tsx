// 题目表单弹层(题库内容页与共享资源库共用)。
//
// 正文按内容类型渲染显式字段:实验模板填概述与步骤,竞赛题填场景,理论题填题干与选项。
// 答案、判题配置与 flag 通过 sensitive_fields 声明剥离路径 —— 学生取题面时后端按此过滤。
// 不给裸 JSON 文本域(旧前端 16 处裸 JSON 是被审查列为 P0 的问题)。

import { useCallback, useId, useMemo, useState } from 'react'
import {
  ContentDifficulty,
  ContentType,
  ContentVisibility,
  type ContentAttachment,
  type ContentItem,
  type ContentItemSnapshot,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  Checkbox,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  SegmentedControl,
  Select,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { useAsyncResource } from '../../../../hooks'
import { CompositionDeclarationFields } from '../../../sandbox/components/CompositionDeclarationFields'
import {
  declarationFromSpec,
  derivedInfraFromSpec,
  readScenarioNeutralSpec,
  scenarioNeutralSpecFromDeclaration,
} from '../../../sandbox/composition'
import { CONTENT_DIFFICULTIES, CONTENT_TYPES, CONTENT_VISIBILITIES } from '../../options'
import {
  contentDifficultyLabel,
  contentTypeLabel,
  contentVisibilityLabel,
} from '../../../../utils/labels/content'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ContentAttachmentsField } from '../../components/ContentAttachmentsField'

/** 正文各字段在 body 里的键,与 M5 架构设计 §2.2 的类型化内容体对应。 */
const BODY_KEYS = {
  summary: 'summary',
  steps: 'steps',
  scenario: 'scenario',
  statement: 'statement',
  question: 'question',
  options: 'options',
  answer: 'answer',
  explanation: 'explanation',
  /** 实操题的环境声明:M2 组合契约,竞赛按它编译并起环境 */
  composition: 'composition',
  submitKey: 'submit_key',
  /** 三种类型共用的附件字段:按类型各起一个名就是三套同义结构 */
  attachments: 'attachments',
} as const

/**
 * 需要对学生剥离的敏感路径。
 * 后端 defaultSensitivePaths 已经默认剥离 answer/judge_config/flag 等,
 * 这里显式声明的是本表单会写入的答案与解析路径 —— 声明是作者的责任,不依赖默认值。
 */
const SENSITIVE_PATHS_BY_TYPE: Record<ContentType, string[]> = {
  [ContentType.EXPERIMENT_TEMPLATE]: ['judge_config'],
  [ContentType.CONTEST_PROBLEM]: ['judge_config', 'flag'],
  [ContentType.THEORY_QUESTION]: ['answer', 'explanation'],
}

export interface ContentItemFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  item?: ContentItem
  onClose: () => void
  onSaved: () => void
}

/**
 * ContentItemFormModal 承载题目创建与编辑。
 * 编辑时先取全量正文(教师有权限读 full),否则无法把已有答案回填到表单。
 */
export function ContentItemFormModal({ item, onClose, onSaved }: ContentItemFormModalProps) {
  const editing = item !== undefined

  const snapshot = useAsyncResource(
    () => (item ? api.content.getItemFull(item.code, item.version) : Promise.resolve(undefined)),
    [item?.code, item?.version],
    () => false,
  )

  if (editing && snapshot.status === 'loading') {
    return (
      <Modal open onOpenChange={(open) => !open && onClose()}>
        <ModalContent size="xl">
          <ModalHeader>
            <ModalTitle>编辑题目</ModalTitle>
            <ModalDescription>正在读取题目正文…</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <Skeleton variant="line" lines={6} />
          </ModalBody>
        </ModalContent>
      </Modal>
    )
  }

  if (editing && snapshot.status === 'error') {
    return (
      <Modal open onOpenChange={(open) => !open && onClose()}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>暂时打不开这道题</ModalTitle>
            <ModalDescription>{snapshot.error?.message}</ModalDescription>
          </ModalHeader>
          <ModalBody>
            {snapshot.error?.traceId ? (
              <p className="font-mono text-xs text-ink-faint">
                如需帮助,请提供编号 {snapshot.error.traceId}
              </p>
            ) : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={onClose}>
              关闭
            </Button>
            <Button variant="primary" onClick={snapshot.reload}>
              重新加载
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    )
  }

  return (
    <ItemForm
      item={item}
      snapshot={snapshot.data ?? undefined}
      onClose={onClose}
      onSaved={onSaved}
    />
  )
}

interface ItemFormProps {
  item?: ContentItem
  snapshot?: ContentItemSnapshot
  onClose: () => void
  onSaved: () => void
}

/**
 * ItemForm 渲染题目表单本体。
 */
function ItemForm({ item, snapshot, onClose, onSaved }: ItemFormProps) {
  const fieldId = useId()
  const editing = item !== undefined
  // 正文快照只读一次即定型:包进 useMemo,避免每次渲染都换一个新对象引用
  const body = useMemo(() => snapshot?.body ?? {}, [snapshot?.body])

  const [code, setCode] = useState(item?.code ?? '')
  const [version, setVersion] = useState(item?.version ?? '1.0.0')
  const [type, setType] = useState(String(item?.type ?? ContentType.THEORY_QUESTION))
  const [title, setTitle] = useState(item?.title ?? '')
  const [categoryId, setCategoryId] = useState(item?.category_id ?? '')
  const [difficulty, setDifficulty] = useState(String(item?.difficulty ?? ContentDifficulty.BASIC))
  const [visibility, setVisibility] = useState(String(item?.visibility ?? ContentVisibility.TENANT))
  const [tags, setTags] = useState((item?.tags ?? []).join(','))
  const [knowledgePoints, setKnowledgePoints] = useState((item?.knowledge_points ?? []).join(','))

  // 正文字段:按类型使用其中一部分
  const [summary, setSummary] = useState(readString(body, BODY_KEYS.summary))
  const [steps, setSteps] = useState(readStringArray(body, BODY_KEYS.steps).join('\n'))
  const [scenario, setScenario] = useState(readString(body, BODY_KEYS.scenario))
  const [statement, setStatement] = useState(
    readString(body, BODY_KEYS.statement) || readString(body, BODY_KEYS.question),
  )
  const [options, setOptions] = useState(readStringArray(body, BODY_KEYS.options).join('\n'))
  const [answer, setAnswer] = useState(readString(body, BODY_KEYS.answer))
  const [explanation, setExplanation] = useState(readString(body, BODY_KEYS.explanation))
  const savedComposition = useMemo(
    () => readScenarioNeutralSpec(body, BODY_KEYS.composition),
    [body],
  )
  const [envEnabled, setEnvEnabled] = useState(savedComposition !== undefined)
  const [declaration, setDeclaration] = useState(() => declarationFromSpec(savedComposition))
  const [submitKey, setSubmitKey] = useState(readString(body, BODY_KEYS.submitKey))
  const [attachments, setAttachments] = useState<ContentAttachment[]>(readAttachments(body))

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const categories = useAsyncResource(() => api.content.listCategories(), [], () => false)
  const typeValue = Number(type) as ContentType

  /**
   * buildBody 按类型组装正文,只写该类型用到的键。
   * 附件是三种类型共用的字段:有附件才写这个键,避免给没有附件的题目留个空数组。
   */
  const buildBody = useCallback((): Record<string, unknown> => {
    const shared = attachments.length > 0 ? { [BODY_KEYS.attachments]: attachments } : {}
    if (typeValue === ContentType.EXPERIMENT_TEMPLATE) {
      return {
        ...shared,
        [BODY_KEYS.summary]: summary.trim(),
        [BODY_KEYS.steps]: splitLines(steps),
        // 实验环境由实验编排向导按组合声明配置,模板正文不再写运行时
        // 判题配置由检查点在实验编排里绑定判题器提供,题目正文不写判题细节
      }
    }
    if (typeValue === ContentType.CONTEST_PROBLEM) {
      return {
        ...shared,
        [BODY_KEYS.scenario]: scenario.trim(),
        [BODY_KEYS.statement]: statement.trim(),
        // 实操类赛题写环境声明:竞赛按它编译成不可变快照再起环境(对齐清单 §6.3)。
        // 访问边界不在这里定 —— 同一道题进解题赛还是对抗赛由竞赛侧决定。
        ...(envEnabled
          ? {
              [BODY_KEYS.composition]: scenarioNeutralSpecFromDeclaration(
                code.trim() || item?.code || 'contest-problem',
                declaration,
                derivedInfraFromSpec(savedComposition),
                savedComposition?.links ?? [],
              ),
            }
          : {}),
        // 提交键是表单字段名而不是答案,随题面下发;取值要与判题配置里的 flag_input_key 相同
        ...(submitKey.trim() !== '' ? { [BODY_KEYS.submitKey]: submitKey.trim() } : {}),
      }
    }
    return {
      ...shared,
      [BODY_KEYS.statement]: statement.trim(),
      [BODY_KEYS.options]: splitLines(options),
      [BODY_KEYS.answer]: answer.trim(),
      [BODY_KEYS.explanation]: explanation.trim(),
    }
  }, [
    answer,
    attachments,
    code,
    declaration,
    envEnabled,
    explanation,
    item?.code,
    options,
    savedComposition,
    scenario,
    statement,
    steps,
    submitKey,
    summary,
    typeValue,
  ])

  const validate = useCallback((): boolean => {
    const next: Record<string, string | null> = {
      code: editing ? null : /^[a-z0-9-]{3,}$/.test(code.trim()) ? null : '编号用小写字母、数字与连字符,至少 3 位',
      version: editing ? null : /^\d+\.\d+\.\d+$/.test(version.trim()) ? null : '版本号形如 1.0.0',
      title: title.trim() === '' ? '请输入题目标题' : null,
    }

    if (typeValue === ContentType.THEORY_QUESTION) {
      next.statement = statement.trim() === '' ? '请输入题干' : null
      next.answer = answer.trim() === '' ? '请填写答案,它对学生不可见' : null
    }
    if (typeValue === ContentType.CONTEST_PROBLEM) {
      next.statement = statement.trim() === '' ? '请输入题面' : null
    }
    if (typeValue === ContentType.EXPERIMENT_TEMPLATE) {
      next.summary = summary.trim() === '' ? '请填写实验概述' : null
    }

    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [answer, code, editing, statement, summary, title, typeValue, version])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      setSubmitting(true)
      setFormError(undefined)
      const common = {
        title: title.trim(),
        category_id: categoryId || undefined,
        difficulty: Number(difficulty) as ContentDifficulty,
        tags: splitCommas(tags),
        knowledge_points: splitCommas(knowledgePoints),
        visibility: Number(visibility) as ContentVisibility,
        body: buildBody(),
        sensitive_fields: SENSITIVE_PATHS_BY_TYPE[typeValue],
      }
      try {
        if (editing) {
          await api.content.updateItem(item.id, common)
          toast.success('题目已更新')
        } else {
          await api.content.createItem({
            code: code.trim(),
            version: version.trim(),
            type: typeValue,
            ...common,
          })
          toast.success('题目已创建为草稿')
        }
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(error, editing ? '题目更新失败,请稍后重试。' : '题目创建失败,请稍后重试。'),
        )
      } finally {
        setSubmitting(false)
      }
    },
    [
      buildBody,
      categoryId,
      code,
      difficulty,
      editing,
      item?.id,
      knowledgePoints,
      onSaved,
      tags,
      title,
      typeValue,
      validate,
      version,
      visibility,
    ],
  )

  const categoryOptions = useMemo(
    () => [
      { value: '', label: '不归入分类' },
      ...(categories.data ?? []).map((category) => ({ value: category.id, label: category.name })),
    ],
    [categories.data],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑题目' : '新建题目'}</ModalTitle>
          <ModalDescription>
            答案与判题配置对学生不可见。发布后正文不宜再改,需要修改请建新版本。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            {!editing ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  label="题目短名"
                  htmlFor={`${fieldId}-code`}
                  required
                  error={errors.code}
                  helper="作业与实验按短名和版本引用题目,创建后不可修改"
                >
                  <Input
                    id={`${fieldId}-code`}
                    value={code}
                    invalid={Boolean(errors.code)}
                    onChange={(event) => setCode(event.target.value)}
                  />
                </FormField>
                <FormField
                  label="版本号"
                  htmlFor={`${fieldId}-version`}
                  required
                  error={errors.version}
                  helper="形如 1.0.0"
                >
                  <Input
                    id={`${fieldId}-version`}
                    value={version}
                    invalid={Boolean(errors.version)}
                    onChange={(event) => setVersion(event.target.value)}
                  />
                </FormField>
              </div>
            ) : null}

            <FormField label="题目标题" htmlFor={`${fieldId}-title`} required error={errors.title}>
              <Input
                id={`${fieldId}-title`}
                value={title}
                invalid={Boolean(errors.title)}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            {!editing ? (
              <FormField label="题目类型" required helper="类型决定正文结构,创建后不可修改">
                <SegmentedControl
                  aria-label="题目类型"
                  options={CONTENT_TYPES.map((value) => ({
                    value: String(value),
                    label: contentTypeLabel(value),
                  }))}
                  value={type}
                  onValueChange={setType}
                />
              </FormField>
            ) : null}

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="难度" htmlFor={`${fieldId}-difficulty`} required>
                <Select
                  id={`${fieldId}-difficulty`}
                  options={CONTENT_DIFFICULTIES.map((value) => ({
                    value: String(value),
                    label: contentDifficultyLabel(value),
                  }))}
                  value={difficulty}
                  onValueChange={setDifficulty}
                />
              </FormField>
              <FormField label="分类" htmlFor={`${fieldId}-category`}>
                <Select
                  id={`${fieldId}-category`}
                  options={categoryOptions}
                  value={categoryId}
                  placeholder={categories.status === 'loading' ? '正在读取分类' : '不归入分类'}
                  onValueChange={setCategoryId}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="标签"
                htmlFor={`${fieldId}-tags`}
                helper="用逗号分隔,便于检索"
              >
                <Input
                  id={`${fieldId}-tags`}
                  value={tags}
                  onChange={(event) => setTags(event.target.value)}
                />
              </FormField>
              <FormField
                label="知识点"
                htmlFor={`${fieldId}-knowledge`}
                helper="用逗号分隔,组卷时可按知识点抽题"
              >
                <Input
                  id={`${fieldId}-knowledge`}
                  value={knowledgePoints}
                  onChange={(event) => setKnowledgePoints(event.target.value)}
                />
              </FormField>
            </div>

            <FormField label="可见范围" required>
              <SegmentedControl
                aria-label="可见范围"
                options={CONTENT_VISIBILITIES.map((value) => ({
                  value: String(value),
                  label: contentVisibilityLabel(value),
                }))}
                value={visibility}
                onValueChange={setVisibility}
              />
            </FormField>

            <div className="flex flex-col gap-4 well p-4">
              <div className="text-sm font-medium text-ink">题目正文</div>

              {typeValue === ContentType.EXPERIMENT_TEMPLATE ? (
                <>
                  <FormField
                    label="实验概述"
                    htmlFor={`${fieldId}-summary`}
                    required
                    error={errors.summary}
                  >
                    <Textarea
                      id={`${fieldId}-summary`}
                      value={summary}
                      rows={3}
                      invalid={Boolean(errors.summary)}
                      onChange={(event) => setSummary(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="操作步骤"
                    htmlFor={`${fieldId}-steps`}
                    helper="每行一步"
                  >
                    <Textarea
                      id={`${fieldId}-steps`}
                      value={steps}
                      rows={5}
                      onChange={(event) => setSteps(event.target.value)}
                    />
                  </FormField>
                </>
              ) : null}

              {typeValue === ContentType.CONTEST_PROBLEM ? (
                <>
                  <FormField label="赛题场景" htmlFor={`${fieldId}-scenario`}>
                    <Textarea
                      id={`${fieldId}-scenario`}
                      value={scenario}
                      rows={3}
                      onChange={(event) => setScenario(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="题面"
                    htmlFor={`${fieldId}-statement`}
                    required
                    error={errors.statement}
                    helper="学生答题时看到的内容"
                  >
                    <Textarea
                      id={`${fieldId}-statement`}
                      value={statement}
                      rows={6}
                      invalid={Boolean(errors.statement)}
                      onChange={(event) => setStatement(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="答案提交字段"
                    htmlFor={`${fieldId}-submit-key`}
                    helper="学生输入的答案放在哪个字段中;需与判题配置保持一致;纯代码题留空"
                  >
                    <Input
                      id={`${fieldId}-submit-key`}
                      className="font-mono text-sm"
                      value={submitKey}
                      placeholder="例如 answer"
                      onChange={(event) => setSubmitKey(event.target.value)}
                    />
                  </FormField>

                  <div className="flex flex-col gap-4 well p-4">
                    <Checkbox
                      checked={envEnabled}
                      label="这道题需要实操环境(学生要写代码或操作链上合约)"
                      onCheckedChange={(checked) => setEnvEnabled(checked === true)}
                    />
                    {envEnabled ? (
                      <>
                        <p className="text-sm text-ink-sub">
                          环境随题目版本一起锁定:编排赛事时不再另配一套,平台换镜像也不会改变已开赛的赛题。
                        </p>
                        <CompositionDeclarationFields
                          idPrefix={`${fieldId}-composition`}
                          value={declaration}
                          onChange={setDeclaration}
                          toolsHelper="学生答这道题时能打开哪些工具"
                          derivedInfraCodes={derivedInfraFromSpec(savedComposition).map(
                            (infra) => infra.code,
                          )}
                        />
                      </>
                    ) : (
                      <p className="text-sm text-ink-sub">
                        纯答题类赛题不需要环境,学生按上面的提交字段填答案即可。
                      </p>
                    )}
                  </div>

                  <Callout tone="info">
                    判题配置与答案内容不在这里填写,也不会传给学生。提交键只是表单字段名,
                    随题面下发,学生据此知道答案该放在哪里。
                  </Callout>
                </>
              ) : null}

              {typeValue === ContentType.THEORY_QUESTION ? (
                <>
                  <FormField
                    label="题干"
                    htmlFor={`${fieldId}-statement`}
                    required
                    error={errors.statement}
                  >
                    <Textarea
                      id={`${fieldId}-statement`}
                      value={statement}
                      rows={4}
                      invalid={Boolean(errors.statement)}
                      onChange={(event) => setStatement(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="选项"
                    htmlFor={`${fieldId}-options`}
                    helper="每行一个选项。留空表示简答题"
                  >
                    <Textarea
                      id={`${fieldId}-options`}
                      value={options}
                      rows={4}
                      onChange={(event) => setOptions(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="答案"
                    htmlFor={`${fieldId}-answer`}
                    required
                    error={errors.answer}
                    helper="答案对学生不可见,只用于自动判分"
                  >
                    <Input
                      id={`${fieldId}-answer`}
                      value={answer}
                      invalid={Boolean(errors.answer)}
                      onChange={(event) => setAnswer(event.target.value)}
                    />
                  </FormField>
                  <FormField
                    label="解析"
                    htmlFor={`${fieldId}-explanation`}
                    helper="解析同样对学生不可见,便于出题人之间交流"
                  >
                    <Textarea
                      id={`${fieldId}-explanation`}
                      value={explanation}
                      rows={3}
                      onChange={(event) => setExplanation(event.target.value)}
                    />
                  </FormField>
                </>
              ) : null}

              {/* 附件三种类型共用:题面配图、说明文档、示例数据都走这里,学生取题面时可见 */}
              <FormField
                label="正文附件"
                helper={
                  editing
                    ? '配图、说明文档与示例数据。学生取题面时能看到它们'
                    : '配图、说明文档与示例数据。创建题目后可在此下载已上传的附件'
                }
              >
                <ContentAttachmentsField
                  attachments={attachments}
                  onChange={setAttachments}
                  resourceId={item?.id}
                />
              </FormField>
            </div>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存修改' : '创建题目'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** readString 从正文里读字符串字段;非字符串回空串(不把对象塞进文本控件)。 */
function readString(body: Record<string, unknown>, key: string): string {
  const value = body[key]
  return typeof value === 'string' ? value : ''
}

/** readStringArray 从正文里读字符串数组;非字符串元素跳过。 */
function readStringArray(body: Record<string, unknown>, key: string): string[] {
  const value = body[key]
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

/**
 * readAttachments 从正文里读附件清单。
 * 只接受三字段齐全且 object_ref 是统一文件服务引用的条目 —— 结构不对的条目留在表单里也提交不过去
 * (后端 validateContentBodyRefs 会拒),不如在读取边界就滤掉。
 */
function readAttachments(body: Record<string, unknown>): ContentAttachment[] {
  const value = body[BODY_KEYS.attachments]
  if (!Array.isArray(value)) return []
  return value.filter((item): item is ContentAttachment => {
    if (!item || typeof item !== 'object') return false
    const candidate = item as Partial<ContentAttachment>
    return (
      typeof candidate.object_ref === 'string' &&
      candidate.object_ref.startsWith('s3://') &&
      typeof candidate.file_name === 'string' &&
      typeof candidate.size === 'number'
    )
  })
}

/** splitLines 把多行文本拆成数组,去掉空行。 */
function splitLines(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
}

/** splitCommas 把逗号分隔文本拆成数组,去掉空项。 */
function splitCommas(text: string): string[] {
  return text
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '')
}
