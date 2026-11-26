// 入队前不读取赛事详情，跨校访问仍由服务端当前成员鉴权。
import { Link, useNavigate, useParams } from 'react-router'
import { buttonVariants, PageHeader, PageScaffold } from '@chaimir/ui'
import { usePageLocation } from '../../../../hooks'
import { JoinTeamForm } from '../../components/JoinTeamForm'

/** StudentContestJoinPage 接收队长分享的入口，入队成功后进入赛事详情。 */
export default function StudentContestJoinPage() {
  const { contestId = '' } = useParams<{ contestId: string }>()
  const navigate = useNavigate()
  usePageLocation([{ label: '加入队伍' }])

  return (
    <PageScaffold>
      <PageHeader
        title="加入队伍"
        description="输入队长分享的邀请码。加入成功后，就可以查看这场赛事和你的队伍。"
        actions={<Link to="/student/contests" className={buttonVariants({ variant: 'outline' })}>查看我的赛事</Link>}
      />
      <section aria-label="邀请码入队" className="max-w-md rounded-lg bg-surface p-5">
        <JoinTeamForm
          key={contestId}
          contestId={contestId}
          onJoined={() => navigate(`/student/contests/${contestId}`, { replace: true })}
        />
      </section>
    </PageScaffold>
  )
}
