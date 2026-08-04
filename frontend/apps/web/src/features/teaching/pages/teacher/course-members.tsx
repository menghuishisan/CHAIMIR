// 选课成员管理(课程详情页内区块)。
// 批量添加需要学生编号,而教师手上只有姓名与学号 —— 故从组织架构选班级、
// 再从该班级账号里勾选学生,编号由页面在提交时解析(不让教师手填内部 ID)。

import { useCallback, useMemo, useState } from 'react'
import { UserMinus, UserPlus, Users } from 'lucide-react'
import { AccountStatus, UserRole, type Account, type Class, type CourseMember } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Checkbox,
  Empty,
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
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { joinModeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 一次最多取回的班级学生数:后端分页上限 100,一个班级不会超过这个量级。 */
const CLASS_STUDENT_SIZE = 100

export interface CourseMembersProps {
  courseId: string
}

/**
 * CourseMembers 管理课程成员。
 */
export function CourseMembers({ courseId }: CourseMembersProps) {
  const [addOpen, setAddOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<CourseMember>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const members = usePagedResource<CourseMember>(
    (params) => api.teaching.listMembers(courseId, params),
    [courseId],
  )

  // 成员姓名需要账号档案:成员记录只回 student_id
  const accounts = useAsyncResource(
    () => api.identity.getAccounts({ role: UserRole.STUDENT, page: 1, size: CLASS_STUDENT_SIZE }),
    [],
    () => false,
  )

  const accountById = useMemo(
    () => new Map((accounts.data?.list ?? []).map((account: Account) => [account.id, account])),
    [accounts.data],
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
      render: (member) => {
        const account = accountById.get(member.student_id)
        return (
          <div className="min-w-0">
            <div className="truncate font-medium text-ink">{account ? account.name : '已离校学生'}</div>
            {account?.no ? <div className="truncate font-mono text-xs text-ink-sub">{account.no}</div> : null}
          </div>
        )
      },
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
          aria-label="把该学生移出课程"
          onClick={() => setRemoveTarget(member)}
        />
      ),
    },
  ]

  return (
    <PageSection
      title="选课成员"
      description={`共 ${members.total} 名学生。学生也可以用邀请码自行加入。`}
      actions={
        <Button variant="primary" leftIcon={UserPlus} onClick={() => setAddOpen(true)}>
          按班级添加
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

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
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <>
              <Table columns={columns} data={page.list} rowKey={(member) => member.id} />
              <Pagination
                page={members.page}
                pageSize={members.pageSize}
                total={members.total}
                onPageChange={members.setPage}
              />
            </>
          )}
        </ResourceState>
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
            <ModalTitle>确认移出课程</ModalTitle>
            <ModalDescription>
              移出后该学生看不到这门课程,已有的作业提交与成绩记录仍保留。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">
              {removeTarget ? (accountById.get(removeTarget.student_id)?.name ?? '该学生') : ''}
            </p>
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(undefined)}>
              取消
            </Button>
            <Button variant="danger" loading={working} onClick={() => void removeMember()}>
              确认移出
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
 * AddMembersModal 按班级勾选学生并批量加入课程。
 * 学生编号由页面从账号档案解析,教师只看到姓名与学号。
 */
function AddMembersModal({ courseId, onClose, onAdded }: AddMembersModalProps) {
  const [classId, setClassId] = useState<string>('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)

  // 选定班级后取该班学生:未选班级不拉全校账号(数据量与意义都不对)
  const students = useAsyncResource(
    () =>
      classId
        ? api.identity.getAccounts({
            role: UserRole.STUDENT,
            class_id: classId,
            status: AccountStatus.ACTIVE,
            page: 1,
            size: CLASS_STUDENT_SIZE,
          })
        : Promise.resolve({ list: [], total: 0, page: 1, size: CLASS_STUDENT_SIZE }),
    [classId],
    (value) => value.list.length === 0,
  )

  const toggle = useCallback((accountId: string) => {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(accountId)) next.delete(accountId)
      else next.add(accountId)
      return next
    })
  }, [])

  const submit = useCallback(async () => {
    if (selected.size === 0) {
      setFormError('请至少勾选一名学生')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.teaching.addMembers(courseId, { student_ids: Array.from(selected) })
      toast.success(`已添加 ${selected.size} 名学生`)
      onAdded()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '添加没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [courseId, onAdded, selected])

  const classOptions = useMemo(
    () => (classes.data ?? []).map((item: Class) => ({ value: item.id, label: item.name })),
    [classes.data],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>按班级添加学生</ModalTitle>
          <ModalDescription>先选班级,再勾选要加入课程的学生。</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="班级" htmlFor="add-members-class" required>
            <Select
              id="add-members-class"
              options={classOptions}
              value={classId}
              placeholder={classOptions.length > 0 ? '选择班级' : '暂无班级'}
              disabled={classOptions.length === 0}
              onValueChange={(value) => {
                setClassId(value)
                setSelected(new Set())
              }}
            />
          </FormField>

          {classId ? (
            <ResourceState
              resource={students}
              emptyIcon={Users}
              emptyTitle="这个班级没有可添加的学生"
              emptyDescription="班级里没有状态正常的学生账号。"
              skeleton={<Skeleton variant="line" lines={4} />}
            >
              {(page) => (
                <div className="flex max-h-72 flex-col gap-2 overflow-y-auto rounded-md border border-line p-3">
                  {page.list.map((account: Account) => (
                    <Checkbox
                      key={account.id}
                      checked={selected.has(account.id)}
                      label={`${account.name}${account.no ? ` · ${account.no}` : ''}`}
                      onCheckedChange={() => toggle(account.id)}
                    />
                  ))}
                </div>
              )}
            </ResourceState>
          ) : (
            <Empty icon={Users} title="请先选择班级" description="选定班级后才会列出学生。" />
          )}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="seal" loading={working} onClick={() => void submit()}>
            添加 {selected.size > 0 ? `${selected.size} 名` : ''}学生
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
