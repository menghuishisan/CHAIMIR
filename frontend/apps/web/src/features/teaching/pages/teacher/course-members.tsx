// 选课成员管理(课程详情页内区块)。
// 批量添加的粒度是「一个班级」(M6 需求 C1):教师从组织架构选班,服务端按班解析在校学生,
// 页面不接触学生编号 —— 账号目录是学校管理员能力,教师端也不该先拉一份全校名录。
// 成员列表的姓名与学号由 M6 随成员一并下发(服务端经 M1 契约批量解析),不在前端二次匹配。
// 设计 §7.4:成员表(姓名、学号、加入方式、加入时间),搜索写进地址。成员接口不接受关键词,
// 一门课 30–200 人,故按页取完后在前端按姓名或学号筛选、分页。

import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { UserMinus, UserPlus, Users } from 'lucide-react'
import { PAGINATION_DEFAULT_SIZE, PAGINATION_MAX_SIZE, type Class, type CourseMember } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
  FormField,
  IconButton,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageSection,
  Pagination,
  Select,
  Skeleton,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { SearchBox } from '../../../../components/SearchBox'
import { collectAllPages } from '../../../../utils/allPages'
import { readPage } from '../../../../utils/urlQuery'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatShortDateTime } from '../../../../utils/formatters'
import { joinModeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

export interface CourseMembersProps {
  courseId: string
}

/** 地址参数:q 是成员搜索词(姓名或学号) */
const MEMBERS_QUERY = {
  q: { defaultValue: '' },
} as const

/**
 * CourseMembers 管理课程成员。
 */
export function CourseMembers({ courseId }: CourseMembersProps) {
  const [addOpen, setAddOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<CourseMember>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const [query, setQuery] = useUrlState(MEMBERS_QUERY)
  const [searchParams, setSearchParams] = useSearchParams()
  const page = readPage(searchParams, 'page')
  const members = useAsyncResource<CourseMember[]>(
    () => collectAllPages((params) => api.teaching.listMembers(courseId, params), PAGINATION_MAX_SIZE),
    [courseId],
  )
  const keyword = query.q.trim().toLowerCase()
  const filtered = useMemo(
    () =>
      (members.data ?? []).filter(
        (member) =>
          keyword === '' ||
          member.student_name.toLowerCase().includes(keyword) ||
          (member.student_no ?? '').toLowerCase().includes(keyword),
      ),
    [keyword, members.data],
  )
  const pageItems = filtered.slice((page - 1) * PAGINATION_DEFAULT_SIZE, page * PAGINATION_DEFAULT_SIZE)
  /** setPage 把成员表页码写进地址(替换历史记录) */
  const setPage = (next: number) =>
    setSearchParams(
      (previous) => {
        const params = new URLSearchParams(previous)
        if (next === 1) params.delete('page')
        else params.set('page', String(next))
        return params
      },
      { replace: true },
    )

  const removeMember = useCallback(async () => {
    if (!removeTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.teaching.removeMember(courseId, removeTarget.student_id)
      toast.success('已移出课程')
      setRemoveTarget(undefined)
      members.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '移出没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, members, removeTarget])

  const columns: TableColumn<CourseMember>[] = [
    {
      key: 'student_id',
      header: '学生',
      render: (member) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{member.student_name}</div>
          {member.student_no ? (
            <div className="truncate font-mono text-xs text-ink-sub">{member.student_no}</div>
          ) : null}
        </div>
      ),
    },
    {
      key: 'join_mode',
      header: '加入方式',
      render: (member) => <Badge tone="neutral">{joinModeLabel(member.join_mode)}</Badge>,
    },
    {
      key: 'joined_at',
      header: '加入时间',
      render: (member) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(member.joined_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (member) => (
        <IconButton
          variant="ghost"
          size="sm"
          icon={UserMinus}
          aria-label={`把${member.student_name}移出课程`}
          onClick={() => setRemoveTarget(member)}
        />
      ),
    },
  ]

  return (
    <PageSection
      title="选课成员"
      description="学生也可以用邀请码自行加入。"
      actions={
        <Button variant="primary" leftIcon={UserPlus} onClick={() => setAddOpen(true)}>
          按班级添加
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        <SearchBox
          label="搜索成员"
          placeholder="按姓名或学号搜索"
          value={query.q}
          onSearch={(value) => setQuery({ q: value })}
        />
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <DataPanel
          label="选课成员"
          footer={
            <Pagination
              page={page}
              pageSize={PAGINATION_DEFAULT_SIZE}
              total={members.data ? filtered.length : undefined}
              onPageChange={setPage}
            />
          }
        >
          <ResourceState
            resource={members}
            emptyIcon={Users}
            emptyTitle="还没有学生加入"
            emptyDescription="把课程邀请码发给学生,或按班级批量添加。"
            emptyAction={
              <Button variant="primary" leftIcon={UserPlus} onClick={() => setAddOpen(true)}>
                按班级添加
              </Button>
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {() => (
              <Table
                columns={columns}
                data={pageItems}
                rowKey={(member) => member.id}
                elevated={false}
                empty={<p className="px-4 py-8 text-center text-sm text-ink-sub">没有匹配「{query.q}」的成员。</p>}
                // <md 换行卡(§6.4.1 规则 3):学生名一行、学号与加入方式一行,移出按钮在右
                mobileCard={(member) => ({
                  title: member.student_name,
                  meta: `${member.student_no ? `${member.student_no} · ` : ''}${joinModeLabel(member.join_mode)} · ${formatShortDateTime(member.joined_at)}`,
                  action: (
                    <IconButton
                      variant="ghost"
                      size="sm"
                      icon={UserMinus}
                      aria-label="把该学生移出课程"
                      onClick={() => setRemoveTarget(member)}
                    />
                  ),
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
      </div>

      {addOpen ? (
        <AddMembersModal
          courseId={courseId}
          onClose={() => setAddOpen(false)}
          onAdded={() => {
            setAddOpen(false)
            members.reload()
          }}
        />
      ) : null}

      <Modal open={removeTarget !== undefined} onOpenChange={(open) => !open && setRemoveTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>把「{removeTarget?.student_name ?? ''}」移出课程?</ModalTitle>
            <ModalDescription>
              移出后这名学生看不到这门课程,已有的作业提交与成绩记录仍保留。
            </ModalDescription>
          </ModalHeader>
          <ModalFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(undefined)}>
              取消
            </Button>
            <Button variant="danger" loading={working} onClick={() => void removeMember()}>
              移出课程
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}

interface AddMembersModalProps {
  courseId: string
  onClose: () => void
  onAdded: () => void
}

/**
 * AddMembersModal 选班级并把该班在校学生整批加入课程。
 * 粒度就是「一个班」(M6 需求 C1):教师手上有的是班级,学生编号是内部标识,
 * 由服务端按班级解析,页面不先取一份账号目录再回传编号 —— 账号目录是校管能力。
 * 已在课程内的学生重复加入不会出错(服务端按课程+学生幂等),故不需要先做差集。
 */
function AddMembersModal({ courseId, onClose, onAdded }: AddMembersModalProps) {
  const [classId, setClassId] = useState<string>('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(useCallback(() => ({
    classId: classId === '' ? '请选择要加入课程的班级' : null,
  }), [classId]))

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)

  const classOptions = useMemo(
    () => (classes.data ?? []).map((item: Class) => ({ value: item.id, label: item.name })),
    [classes.data],
  )

  const submit = useCallback(async () => {
    if (!validate()) return
    setFormError(undefined)
    setWorking(true)
    try {
      const added = await api.teaching.addMembers(courseId, { class_id: classId })
      toast.success(`已添加 ${added.length} 名学生`)
      onAdded()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '添加没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [classId, courseId, onAdded, validate])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="md">
        <ModalHeader>
          <ModalTitle>按班级添加学生</ModalTitle>
          <ModalDescription>
            选定班级后,该班在读学生会全部加入本课程;已在课程里的学生不会重复添加。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <ResourceState
            resource={classes}
            emptyIcon={Users}
            emptyTitle="学校还没有建班级"
            emptyDescription="请联系学校管理员在组织架构里创建班级。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() => (
              <FormField label="班级" htmlFor="add-members-class" required error={errors.classId}>
                <Select
                  id="add-members-class"
                  options={classOptions}
                  value={classId}
                  invalid={Boolean(errors.classId)}
                  onBlur={() => validate(['classId'])}
                  placeholder="选择班级"
                  onValueChange={setClassId}
                />
              </FormField>
            )}
          </ResourceState>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            添加学生
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
