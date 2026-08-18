// 竞赛配置表单(赛事组织页与赛事详情页共用)。
// 创建与编辑共用同一表单:后端 ContestRequest 两者字段一致。
//
// 赛制规则(rules)是 JSONB 开放对象。这里按赛制渲染结构化字段
// (解题赛:是否允许重复提交、失败冷却;对抗赛:匹配方式、每轮时长),
// 不把它作为裸 JSON 文本域交给用户。

import { useCallback, useId, useMemo, useState } from 'react'
import { ContestMode, MatchMode, TeamMode, type Contest, type ContestRequest } from '@chaimir/api-client'
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
  SegmentedControl,
  Select,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { toDateTimeInputValue } from '../../../utils/dateInput'
import { contestModeLabel, matchModeLabel, teamModeLabel } from '../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 赛制规则在 rules 里的键:与后端约定的结构化形状对应。 */
const RULE_ALLOW_RESUBMIT = 'allow_resubmit'
const RULE_FAILED_COOLDOWN = 'failed_cooldown_seconds'
const RULE_ROUND_MINUTES = 'round_minutes'

export interface ContestFormModalProps {
  /** 传入即为编辑模式;缺省为新建 */
  contest?: Contest
  onClose: () => void
  onSaved: () => void
}

/**
 * ContestFormModal 承载赛事创建与编辑。
 */
export function ContestFormModal({ contest, onClose, onSaved }: ContestFormModalProps) {
  const fieldId = useId()
  const editing = contest !== undefined

  const [name, setName] = useState(contest?.name ?? '')
  const [mode, setMode] = useState(String(contest?.mode ?? ContestMode.SOLVE))
  const [matchMode, setMatchMode] = useState(String(contest?.match_mode ?? MatchMode.ELO))
  const [teamMode, setTeamMode] = useState(String(contest?.team_mode ?? TeamMode.GROUP))
  const [signupStart, setSignupStart] = useState(toDateTimeInputValue(contest?.signup_start))
  const [signupEnd, setSignupEnd] = useState(toDateTimeInputValue(contest?.signup_end))
  const [startAt, setStartAt] = useState(toDateTimeInputValue(contest?.start_at))
  const [endAt, setEndAt] = useState(toDateTimeInputValue(contest?.end_at))
  const [freezeMinutes, setFreezeMinutes] = useState(String(contest?.freeze_minutes ?? 30))
  const [allowResubmit, setAllowResubmit] = useState(readBoolean(contest, RULE_ALLOW_RESUBMIT, true))
  const [failedCooldown, setFailedCooldown] = useState(
    String(readNumber(contest, RULE_FAILED_COOLDOWN, 60)),
  )
  const [roundMinutes, setRoundMinutes] = useState(String(readNumber(contest, RULE_ROUND_MINUTES, 10)))

  const [errors, setErrors] = useState<Record<string, string | null>>({})
  const [formError, setFormError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const isBattle = Number(mode) === ContestMode.BATTLE

  /** validate 校验时间顺序:报名要早于比赛,结束要晚于开始。 */
  const validate = useCallback((): boolean => {
    const next: Record<string, string | null> = {
      name: name.trim() === '' ? '请输入赛事名称' : null,
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
    }
    setErrors(next)
    return Object.values(next).every((value) => value === null)
  }, [endAt, name, signupEnd, signupStart, startAt])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
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
        freeze_minutes: Number(freezeMinutes) || 0,
        // 规则按结构化字段组装,不接受用户手写 JSON
        rules: isBattle
          ? { [RULE_ROUND_MINUTES]: Number(roundMinutes) || 10 }
          : {
              [RULE_ALLOW_RESUBMIT]: allowResubmit,
              [RULE_FAILED_COOLDOWN]: Number(failedCooldown) || 0,
            },
      }
      try {
        if (editing) {
          await api.contest.updateContest(contest.id, payload)
          toast.success('赛事已更新')
        } else {
          await api.contest.createContest(payload)
          toast.success('赛事已创建为草稿')
        }
        onSaved()
      } catch (error) {
        setFormError(
          userFacingErrorMessage(error, editing ? '赛事更新失败,请稍后重试。' : '赛事创建失败,请稍后重试。'),
        )
      } finally {
        setSubmitting(false)
      }
    },
    [
      allowResubmit,
      contest?.id,
      editing,
      endAt,
      failedCooldown,
      freezeMinutes,
      isBattle,
      matchMode,
      mode,
      name,
      onSaved,
      roundMinutes,
      signupEnd,
      signupStart,
      startAt,
      teamMode,
      validate,
    ],
  )

  const matchModeOptions = useMemo(
    () => [
      { value: String(MatchMode.ELO), label: matchModeLabel(MatchMode.ELO) },
      { value: String(MatchMode.ROUND_ROBIN), label: matchModeLabel(MatchMode.ROUND_ROBIN) },
    ],
    [],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑赛事' : '新建赛事'}</ModalTitle>
          <ModalDescription>
            {editing
              ? '修改赛事配置。已发布赛事的时间与赛制变更会影响学生,请谨慎。'
              : '新建后为草稿状态,编排赛题再发布开放报名。'}
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="赛事名称" htmlFor={`${fieldId}-name`} required error={errors.name}>
              <Input
                id={`${fieldId}-name`}
                value={name}
                invalid={Boolean(errors.name)}
                onChange={(event) => setName(event.target.value)}
              />
            </FormField>

            <FormField label="赛制" required helper="解题赛按题目通过数与用时排名;对抗赛按对局胜负积分">
              <SegmentedControl
                aria-label="赛制"
                options={[
                  { value: String(ContestMode.SOLVE), label: contestModeLabel(ContestMode.SOLVE) },
                  { value: String(ContestMode.BATTLE), label: contestModeLabel(ContestMode.BATTLE) },
                ]}
                value={mode}
                onValueChange={setMode}
              />
            </FormField>

            <FormField label="参赛形式" required>
              <SegmentedControl
                aria-label="参赛形式"
                options={[
                  { value: String(TeamMode.SOLO), label: teamModeLabel(TeamMode.SOLO) },
                  { value: String(TeamMode.GROUP), label: teamModeLabel(TeamMode.GROUP) },
                ]}
                value={teamMode}
                onValueChange={setTeamMode}
              />
            </FormField>

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
            >
              <Input
                id={`${fieldId}-freeze`}
                type="number"
                min="0"
                value={freezeMinutes}
                onChange={(event) => setFreezeMinutes(event.target.value)}
              />
            </FormField>

            {isBattle ? (
              <div className="grid gap-4 well p-4 sm:grid-cols-2">
                <FormField label="对局匹配方式" htmlFor={`${fieldId}-match`} required>
                  <Select
                    id={`${fieldId}-match`}
                    options={matchModeOptions}
                    value={matchMode}
                    onValueChange={setMatchMode}
                  />
                </FormField>
                <FormField
                  label="每轮对局时长(分钟)"
                  htmlFor={`${fieldId}-round`}
                  helper="单场对局的最长执行时间"
                >
                  <Input
                    id={`${fieldId}-round`}
                    type="number"
                    min="1"
                    value={roundMinutes}
                    onChange={(event) => setRoundMinutes(event.target.value)}
                  />
                </FormField>
              </div>
            ) : (
              <div className="grid gap-4 well p-4 sm:grid-cols-2">
                <FormField label="允许重复提交" required>
                  <SegmentedControl
                    aria-label="是否允许重复提交"
                    size="sm"
                    options={[
                      { value: 'true', label: '允许' },
                      { value: 'false', label: '不允许' },
                    ]}
                    value={String(allowResubmit)}
                    onValueChange={(value) => setAllowResubmit(value === 'true')}
                  />
                </FormField>
                <FormField
                  label="判错冷却(秒)"
                  htmlFor={`${fieldId}-cooldown`}
                  helper="提交未通过后需要等待的时间,防止暴力试错"
                >
                  <Input
                    id={`${fieldId}-cooldown`}
                    type="number"
                    min="0"
                    value={failedCooldown}
                    onChange={(event) => setFailedCooldown(event.target.value)}
                  />
                </FormField>
              </div>
            )}

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={submitting}>
              {editing ? '保存修改' : '创建赛事'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

/** readBoolean 从赛制规则里读布尔字段;非布尔回默认值(不把对象塞进控件)。 */
function readBoolean(contest: Contest | undefined, key: string, fallback: boolean): boolean {
  const value = contest?.rules?.[key]
  return typeof value === 'boolean' ? value : fallback
}

/** readNumber 从赛制规则里读数字字段;非数字回默认值。 */
function readNumber(contest: Contest | undefined, key: string, fallback: number): number {
  const value = contest?.rules?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}
