// 选课成员管理(课程详情页内区块)。
// 批量添加的粒度是「一个班级」(M6 需求 C1):教师从组织架构选班,服务端按班解析在校学生,
// 页面不接触学生编号 —— 账号目录是学校管理员能力,教师端也不该先拉一份全校名录。
// 成员列表的姓名与学号由 M6 随成员一并下发(服务端经 M1 契约批量解析),不在前端二次匹配。

import { useCallback, useMemo, useState } from 'react'
import { UserMinus, UserPlus, Users } from 'lucide-react'
import type { Class, CourseMember } from '@chaimir/api-client'
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
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import { joinModeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

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
          aria-label="把该学生移出课程"
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
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):数据表与分页同处一块抬起片 */}
        <DataPanel
          label="选课成员"
          footer={
            <Pagination
              page={members.page}
              pageSize={members.pageSize}
              total={members.total}
              onPageChange={members.setPage}
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
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(member) => member.id}
                elevated={false}
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
            <ModalTitle>确认移出课程</ModalTitle>
            <ModalDescription>
              移出后该学生看不到这门课程,已有的作业提交与成绩记录仍保留。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">{removeTarget?.student_name ?? ''}</p>
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
 * AddMembersModal 选班级并把该班在校学生整批加入课程。
 * 粒度就是「一个班」(M6 需求 C1):教师手上有的是班级,学生编号是内部标识,
 * 由服务端按班级解析,页面不先取一份账号目录再回传编号 —— 账号目录是校管能力。
 * 已在课程内的学生重复加入不会出错(服务端按课程+学生幂等),故不需要先做差集。
 */
function AddMembersModal({ courseId, onClose, onAdded }: AddMembersModalProps) {
  const [classId, setClassId] = useState<string>('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const classes = useAsyncResource(() => api.identity.listClasses(), [], () => false)

  const classOptions = useMemo(
    () => (classes.data ?? []).map((item: Class) => ({ value: item.id, label: item.name })),
    [classes.data],
  )

  const submit = useCallback(async () => {
    if (classId === '') {
      setFormError('请选择要加入课程的班级')
      return
    }
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
  }, [classId, courseId, onAdded])

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
              <FormField label="班级" htmlFor="add-members-class" required>
                <Select
                  id="add-members-class"
                  options={classOptions}
                  value={classId}
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
