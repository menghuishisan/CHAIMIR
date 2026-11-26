// 赛事创建与编辑共用表单，发布后锁定赛制和参赛形式。

import { useId, useState } from 'react'
import { ContestMode, ContestStatus, MatchMode, TeamMode, type Contest, type ContestRequest } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  FormField,
  Input,
  SegmentedControl,
  Select,
  cn,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { toDateTimeInputValue } from '../../../utils/dateInput'
import { contestModeLabel, matchModeLabel, teamModeLabel } from '../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

const matchModeOptions = [
  { value: String(MatchMode.ELO), label: matchModeLabel(MatchMode.ELO) },
  { value: String(MatchMode.ROUND_ROBIN), label: matchModeLabel(MatchMode.ROUND_ROBIN) },
]

export interface ContestFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  contest?: Contest
  onClose: () => void
  /** 保存成功;参数是保存后的赛事(新建时调用方据此进入新赛事详情) */
  onSaved: (contest: Contest) => void
}

/** ContestFormModal 承载赛事创建与编辑。 */
export function ContestFormModal({ contest, onClose, onSaved }: ContestFormModalProps) {
  const fieldId = useId()
  const editing = contest !== undefined
  // 发布后赛制与参赛形式不能再改:赛题按赛制锁定、报名按参赛形式组队(服务端同样拒绝)
  const structureLocked = editing && contest.status !== ContestStatus.DRAFT

  const [name, setName] = useState(contest?.name ?? '')
  const [mode, setMode] = useState(String(contest?.mode ?? ContestMode.SOLVE))
  const [matchMode, setMatchMode] = useState(String(contest?.match_mode ?? MatchMode.ELO))
  const [teamMode, setTeamMode] = useState(String(contest?.team_mode ?? TeamMode.GROUP))
  const [signupStart, setSignupStart] = useState(toDateTimeInputValue(contest?.signup_start))
  const [signupEnd, setSignupEnd] = useState(toDateTimeInputValue(contest?.signup_end))
  const [startAt, setStartAt] = useState(toDateTimeInputValue(contest?.start_at))
  const [endAt, setEndAt] = useState(toDateTimeInputValue(contest?.end_at))
  const [freezeMinutes, setFreezeMinutes] = useState(String(contest?.freeze_minutes ?? 30))

  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const isBattle = Number(mode) === ContestMode.BATTLE

  const { errors, validate } = useFormValidation(() => {
    const minutes = Number(freezeMinutes)
    return {
      name: name.trim() === '' ? '请输入赛事名称' : [...name.trim()].length > 255 ? '赛事名称不能超过 255 个字' : null,
      signupStart: signupStart === '' ? '请选择报名开始时间' : null,
      signupEnd:
        signupEnd === ''
          ? '请选择报名截止时间'
          : signupStart !== '' && signupEnd <= signupStart
            ? '报名截止要晚于报名开始'
            : null,
      startAt:
        startAt === ''
          ? '请选择比赛开始时间'
          : signupEnd !== '' && startAt < signupEnd
            ? '比赛开始不能早于报名截止'
            : null,
      endAt:
        endAt === ''
          ? '请选择比赛结束时间'
          : startAt !== '' && endAt <= startAt
            ? '比赛结束要晚于比赛开始'
            : null,
      freezeMinutes:
        freezeMinutes.trim() === '' || !Number.isInteger(minutes) || minutes < 0 || minutes > 2147483647
          ? '请填写有效的整数分钟数，0 表示不封榜'
          : null,
    }
  })

  // submit 保存当前输入，失败时保留表单并恢复操作状态。
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!validate()) return

    setSubmitting(true)
    setFormError(undefined)
    const payload: ContestRequest = {
      name: name.trim(),
      mode: Number(mode) as ContestMode,
      match_mode: isBattle ? (Number(matchMode) as MatchMode) : undefined,
      team_mode: Number(teamMode) as TeamMode,
      signup_start: new Date(signupStart).toISOString(),
      signup_end: new Date(signupEnd).toISOString(),
      start_at: new Date(startAt).toISOString(),
      end_at: new Date(endAt).toISOString(),
      freeze_minutes: Number(freezeMinutes),
    }
    try {
      const saved = editing
        ? await api.contest.updateContest(contest.id, payload)
        : await api.contest.createContest(payload)
      toast.success(editing ? '赛事已更新' : '赛事已创建为草稿')
      onSaved(saved)
    } catch (error) {
      setFormError(
        userFacingErrorMessage(error, editing ? '赛事更新失败,请稍后重试。' : '赛事创建失败,请稍后重试。'),
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>{editing ? '编辑赛事' : '新建赛事'}</DrawerTitle>
          <DrawerDescription>
            {editing
              ? '修改赛事配置。已发布赛事的时间与赛制变更会影响学生,请谨慎。'
              : '新建后为草稿状态,编排赛题再发布开放报名。'}
          </DrawerDescription>
        </DrawerHeader>
        <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
          <DrawerBody className="flex flex-col gap-4">
            <FormSectionTitle>基本信息</FormSectionTitle>
            <FormField label="赛事名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
              <Input
                id={`${fieldId}-name`}
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>

            <FormField
              label="赛制"
              required
              helper={
                structureLocked
                  ? '赛事已发布,赛制不能再改'
                  : '解题赛按得分排名；对抗赛按对局结果计算积分'
              }
            >
              <SegmentedControl
                aria-label="赛制"
                options={[
                  { value: String(ContestMode.SOLVE), label: contestModeLabel(ContestMode.SOLVE) },
                  { value: String(ContestMode.BATTLE), label: contestModeLabel(ContestMode.BATTLE) },
                ]}
                value={mode}
                onValueChange={setMode}
                disabled={structureLocked}
              />
            </FormField>

            <FormField
              label="参赛形式"
              required
              helper={structureLocked ? '赛事已发布,参赛形式不能再改' : undefined}
            >
              <SegmentedControl
                aria-label="参赛形式"
                options={[
                  { value: String(TeamMode.SOLO), label: teamModeLabel(TeamMode.SOLO) },
                  { value: String(TeamMode.GROUP), label: teamModeLabel(TeamMode.GROUP) },
                ]}
                value={teamMode}
                onValueChange={setTeamMode}
                disabled={structureLocked}
              />
            </FormField>

            {isBattle ? (
              <FormField label="对局匹配方式" htmlFor={`${fieldId}-match`} required>
                <Select
                  id={`${fieldId}-match`}
                  options={matchModeOptions}
                  value={matchMode}
                  onValueChange={setMatchMode}
                />
              </FormField>
            ) : null}

            <FormSectionTitle className="mt-2">时间安排</FormSectionTitle>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                label="报名开始"
                htmlFor={`${fieldId}-signup-start`}
                required
                error={errors.signupStart}
              >
                <Input
                  id={`${fieldId}-signup-start`}
                  type="datetime-local"
                  value={signupStart}
                  invalid={Boolean(errors.signupStart)}
                  onChange={(event) => setSignupStart(event.target.value)}
                />
              </FormField>
              <FormField
                label="报名截止"
                htmlFor={`${fieldId}-signup-end`}
                required
                error={errors.signupEnd}
              >
                <Input
                  id={`${fieldId}-signup-end`}
                  type="datetime-local"
                  value={signupEnd}
                  invalid={Boolean(errors.signupEnd)}
                  onChange={(event) => setSignupEnd(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="比赛开始" htmlFor={`${fieldId}-start`} required error={errors.startAt}>
                <Input
                  id={`${fieldId}-start`}
                  type="datetime-local"
                  value={startAt}
                  invalid={Boolean(errors.startAt)}
                  onChange={(event) => setStartAt(event.target.value)}
                />
              </FormField>
              <FormField label="比赛结束" htmlFor={`${fieldId}-end`} required error={errors.endAt}>
                <Input
                  id={`${fieldId}-end`}
                  type="datetime-local"
                  value={endAt}
                  invalid={Boolean(errors.endAt)}
                  onChange={(event) => setEndAt(event.target.value)}
                />
              </FormField>
            </div>

            <FormField
              label="封榜时长(分钟)"
              htmlFor={`${fieldId}-freeze`}
              required
              helper="比赛结束前这段时间内榜单对学生停止更新,填 0 表示不封榜"
              error={errors.freezeMinutes}
            >
              <Input
                id={`${fieldId}-freeze`}
                type="number"
                min="0"
                step="1"
                value={freezeMinutes}
                invalid={Boolean(errors.freezeMinutes)}
                onChange={(event) => setFreezeMinutes(event.target.value)}
              />
            </FormField>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </DrawerBody>
          <DrawerFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存修改' : '创建赛事'}
            </Button>
          </DrawerFooter>
        </form>
      </DrawerContent>
    </Drawer>
  )
}

/** FormSectionTitle 标记赛事配置分组。 */
function FormSectionTitle({ children, className }: { children: string; className?: string }) {
  return <h3 className={cn('text-sm font-semibold text-ink', className)}>{children}</h3>
}
