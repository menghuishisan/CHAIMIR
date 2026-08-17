// 批量成绩单(成绩审核页内区块)。
//
// 出成绩单的前提是成绩已锁定,故归到成绩审核页而不是单独放侧栏。
// 后端 GenerateTranscriptBatch 一次最多 200 人,超出直接拒绝 —— 界面按班级选人并显示计数,
// 不让管理员选到 300 人再被后端拒回。
//
// 批量只生成记录,取件仍逐份走一次性下载授权(与学生侧同一个 storage 入口)。

import { useCallback, useMemo, useState } from 'react'
import { Download, FileText, GraduationCap, Users } from 'lucide-react'
import {
  AccountStatus,
  BaseIdentity,
  TranscriptScope,
  type Account,
  type Class,
  type GradeTranscript,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Empty,
  FormField,
  PageSection,
  SegmentedControl,
  Select,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { downloadAttachment } from '../../../../utils/downloadAttachment'
import { formatDateTime } from '../../../../utils/formatters'
import { transcriptScopeLabel } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 后端单次批量上限:超出直接拒绝,故界面先挡住。 */
const BATCH_LIMIT = 200

/** 班级学生一次取回的条数:后端分页上限 100,一个班级不会超过这个量级。 */
const CLASS_STUDENT_SIZE = 100

/**
 * TranscriptBatchSection 按班级批量生成成绩单。
 */
export function TranscriptBatchSection() {
  const [classId, setClassId] = useState('')
  const [scope, setScope] = useState(String(TranscriptScope.FULL))
  const [semesterId, setSemesterId] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [generated, setGenerated] = useState<GradeTranscript[]>()
  const [downloadingId, setDownloadingId] = useState<string>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)

  // 选定班级后取该班在读学生:未选班级不拉全校账号(数据量与意义都不对)
  const students = useAsyncResource(
    () =>
      classId
        ? api.identity.getAccounts({
            role: BaseIdentity.STUDENT,
            class_id: classId,
            status: AccountStatus.ACTIVE,
            page: 1,
            size: CLASS_STUDENT_SIZE,
          })
        : Promise.resolve({ list: [], total: 0, page: 1, size: CLASS_STUDENT_SIZE }),
    [classId],
    (value) => value.list.length === 0,
  )

  const scopeValue = Number(scope) as TranscriptScope
  const needsSemester = scopeValue === TranscriptScope.SEMESTER

  const studentNameById = useMemo(
    () => new Map((students.data?.list ?? []).map((account: Account) => [account.id, account])),
    [students.data],
  )

  const generate = useCallback(async () => {
    if (selected.size === 0) {
      setFormError('请至少勾选一名学生')
      return
    }
    if (selected.size > BATCH_LIMIT) {
      setFormError(`一次最多生成 ${BATCH_LIMIT} 份,请分批处理`)
      return
    }
    if (needsSemester && semesterId === '') {
      setFormError('单学期成绩单需要选择学期')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.grade.generateTranscriptBatch({
        student_ids: Array.from(selected),
        scope: scopeValue,
        semester_id: needsSemester ? semesterId : undefined,
      })
      setGenerated(result)
      toast.success(`已生成 ${result.length} 份成绩单`)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '生成没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [needsSemester, scopeValue, selected, semesterId])

  /** download 逐份取件:授权是一次性的,每次点击重新签发。 */
  const download = useCallback(async (transcript: GradeTranscript) => {
    setDownloadingId(transcript.id)
    setFormError(undefined)
    try {
      const grant = await api.grade.downloadTranscript(transcript.id)
      const file = await api.storage.consumeGrant(grant.token)
      downloadAttachment(file)
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '下载没有完成,请稍后重试。'))
    } finally {
      setDownloadingId(undefined)
    }
  }, [])

  const columns: TableColumn<GradeTranscript>[] = [
    {
      key: 'student_id',
      header: '学生',
      render: (transcript) => {
        const account = studentNameById.get(transcript.student_id)
        return <span className="text-ink">{account ? account.name : '该学生'}</span>
      },
    },
    {
      key: 'scope',
      header: '范围',
      render: (transcript) => <Badge tone="neutral">{transcriptScopeLabel(transcript.scope)}</Badge>,
    },
    {
      key: 'generated_at',
      header: '生成时间',
      render: (transcript) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(transcript.generated_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (transcript) => (
        <Button
          variant="ghost"
          size="sm"
          leftIcon={Download}
          loading={downloadingId === transcript.id}
          onClick={() => void download(transcript)}
        >
          下载
        </Button>
      ),
    },
  ]

  return (
    <PageSection
      title="批量成绩单"
      description="按班级为学生生成成绩单。成绩单反映已锁定的成绩,未审核通过的课程不计入。"
    >
      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader
            title="生成设置"
            description={`一次最多 ${BATCH_LIMIT} 份。生成后在下方逐份下载。`}
            actions={
              selected.size > 0 ? <Badge tone="jade">已选 {selected.size} 人</Badge> : null
            }
          />
          <CardBody className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="班级" htmlFor="transcript-class" required>
                <Select
                  id="transcript-class"
                  options={(classes.data ?? []).map((item: Class) => ({
                    value: item.id,
                    label: `${item.name} · ${item.enrollment_year} 级`,
                  }))}
                  value={classId}
                  placeholder={(classes.data ?? []).length > 0 ? '选择班级' : '暂无班级'}
                  disabled={(classes.data ?? []).length === 0}
                  onValueChange={(value) => {
                    setClassId(value)
                    setSelected(new Set())
                    setGenerated(undefined)
                  }}
                />
              </FormField>
              <FormField label="成绩单范围" required>
                <SegmentedControl
                  aria-label="成绩单范围"
                  size="sm"
                  options={[
                    { value: String(TranscriptScope.FULL), label: transcriptScopeLabel(TranscriptScope.FULL) },
                    {
                      value: String(TranscriptScope.SEMESTER),
                      label: transcriptScopeLabel(TranscriptScope.SEMESTER),
                    },
                  ]}
                  value={scope}
                  onValueChange={setScope}
                />
              </FormField>
            </div>

            {needsSemester ? (
              <FormField label="学期" htmlFor="transcript-semester" required>
                <Select
                  id="transcript-semester"
                  options={(semesters.data ?? []).map((semester: Semester) => ({
                    value: semester.id,
                    label: semester.name,
                  }))}
                  value={semesterId}
                  placeholder="选择学期"
                  onValueChange={setSemesterId}
                />
              </FormField>
            ) : null}

            {classId ? (
              <ResourceState
                resource={students}
                emptyIcon={Users}
                emptyTitle="这个班级没有在读学生"
                emptyDescription="班级里没有状态正常的学生账号。"
                skeleton={<Skeleton variant="line" lines={4} />}
              >
                {(page) => (
                  <FormField label="选择学生" required>
                    <div className="flex flex-col gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            setSelected(new Set(page.list.map((account: Account) => account.id)))
                          }
                        >
                          全选本班
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
                          清空
                        </Button>
                      </div>
                      <div className="flex max-h-72 flex-col gap-2 overflow-y-auto well p-3">
                        {page.list.map((account: Account) => (
                          <Checkbox
                            key={account.id}
                            checked={selected.has(account.id)}
                            label={account.no ? `${account.name} · ${account.no}` : account.name}
                            onCheckedChange={(checked) =>
                              setSelected((current) => {
                                const next = new Set(current)
                                if (checked === true) next.add(account.id)
                                else next.delete(account.id)
                                return next
                              })
                            }
                          />
                        ))}
                      </div>
                    </div>
                  </FormField>
                )}
              </ResourceState>
            ) : (
              <Empty icon={GraduationCap} title="请先选择班级" description="选定班级后才会列出学生。" />
            )}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}

            <div className="flex items-center gap-2">
              <Button
                variant="primary"
                leftIcon={FileText}
                loading={working}
                disabled={selected.size === 0}
                onClick={() => void generate()}
              >
                生成 {selected.size > 0 ? `${selected.size} 份` : ''}成绩单
              </Button>
              <span className="text-sm text-ink-sub">成绩单带学校电子签章,内容按已锁定成绩生成。</span>
            </div>
          </CardBody>
        </Card>

        {generated && generated.length > 0 ? (
          <Card>
            <CardHeader
              title="本次生成结果"
              description="逐份下载。下载链接是一次性的,重新点击会重新签发。"
            />
            <CardBody>
              <Table columns={columns} data={generated} rowKey={(item) => item.id} />
            </CardBody>
          </Card>
        ) : null}
      </div>
    </PageSection>
  )
}
