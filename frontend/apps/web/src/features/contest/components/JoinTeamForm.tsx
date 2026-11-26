// 详情和独立入队页共用邀请码提交，不预读赛事或队伍。
import { useState, type FormEvent } from 'react'
import { Users } from 'lucide-react'
import { Button, Callout, FormField, Input, toast } from '@chaimir/ui'
import { api } from '../../../app/api'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** JoinTeamForm 仅在服务端确认入队后通知调用方，失败时保留输入。 */
export function JoinTeamForm({ contestId, onJoined }: { contestId: string; onJoined: () => void }) {
  const [inviteCode, setInviteCode] = useState('')
  const [joining, setJoining] = useState(false)
  const [error, setError] = useState<string>()
  const validation = useFormValidation(() => ({ inviteCode: !inviteCode.trim() ? '请输入队伍邀请码' : null }))

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (joining || !validation.validate()) return
    setJoining(true)
    setError(undefined)
    try {
      await api.contest.joinTeam(contestId, { invite_code: inviteCode.trim() })
      toast.success('已加入队伍')
      onJoined()
    } catch (cause) {
      setError(userFacingErrorMessage(cause, '没能加入队伍,请确认邀请码后重试。'))
    } finally {
      setJoining(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-3">
      <FormField label="队伍邀请码" htmlFor="contest-team-invite" required error={validation.errors.inviteCode}>
        <Input
          id="contest-team-invite"
          value={inviteCode}
          autoComplete="off"
          placeholder="队长给你的邀请码"
          invalid={Boolean(validation.errors.inviteCode)}
          onChange={(event) => setInviteCode(event.target.value)}
          onBlur={() => validation.validate(['inviteCode'])}
        />
      </FormField>
      {error ? <Callout tone="danger">{error}</Callout> : null}
      <Button type="submit" variant="outline" leftIcon={Users} loading={joining}>
        加入队伍
      </Button>
    </form>
  )
}
