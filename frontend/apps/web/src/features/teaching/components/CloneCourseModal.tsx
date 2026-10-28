// CloneCourseModal统一本人课程与跨校课程库的克隆表单、校验和请求互斥。
import { useCallback, useRef, useState } from 'react'
import type { Course } from '@chaimir/api-client'
import {
  Button,
  Callout,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

interface CloneCourseModalProps {
  course: Pick<Course, 'id' | 'name'>
  onClose: () => void
  onCloned: (course: Course) => void
}

/** CloneCourseModal仅复制教学内容，成功后由调用方导航或刷新本人课程列表。 */
export function CloneCourseModal({ course, onClose, onCloned }: CloneCourseModalProps) {
  const [name, setName] = useState(`${course.name}(副本)`)
  const [actionError, setActionError] = useState<string>()
  const [working, setWorking] = useState(false)
  const workingRef = useRef(false)
  const { errors, validate } = useFormValidation(
    () => ({ name: name.trim() ? null : '请输入新课程名称' }),
    course.id
  )

  /** submit校验当前输入并只发起一次克隆；失败保留已填名称。 */
  const submit = useCallback(async () => {
    if (workingRef.current || !validate()) return
    workingRef.current = true
    setWorking(true)
    setActionError(undefined)
    try {
      const created = await api.teaching.cloneCourse(course.id, { name: name.trim() })
      toast.success('已克隆为新课程草稿')
      onCloned(created)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '克隆没有成功,请稍后重试。'))
    } finally {
      workingRef.current = false
      setWorking(false)
    }
  }, [course.id, name, onCloned, validate])

  return (
    <Modal open onOpenChange={(open) => !open && !workingRef.current && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>克隆课程</ModalTitle>
          <ModalDescription>
            会复制章节课时与作业设置,不复制学生、提交与成绩。新课程为草稿状态。
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
          <FormField label="新课程名称" htmlFor="clone-course-name" required error={errors.name}>
            <Input
              id="clone-course-name"
              value={name}
              invalid={Boolean(errors.name)}
              disabled={working}
              onChange={(event) => setName(event.target.value)}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" disabled={working} onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            确认克隆
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
