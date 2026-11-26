// 匿名分享只回放平台内置包，不提供运行授权或包正文。
import { useParams } from 'react-router'
import { Info, LoaderCircle, Lock, LogIn, TriangleAlert } from 'lucide-react'
import { Button } from '@chaimir/ui'
import { isBuiltinSimulationCode } from '@chaimir/sim-sdk'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { useAsyncResource } from '../../../../hooks/useAsyncResource'
import { useImmersive } from '../../../../layouts/immersive/context'
import { SimReplayRuntime } from '../../components/SimReplayRuntime'

/** PublicReplayPage 只读取分享剧本，不换取隔离运行授权。 */
export default function PublicReplayPage() {
  const { shareCode = '' } = useParams<{ shareCode: string }>()
  const { title, exit } = useImmersive()
  const replay = useAsyncResource(
    () => api.sim.getSharedReplay(shareCode),
    [shareCode],
    (value) => value.actions.length === 0,
  )

  if (replay.status === 'loading') {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在打开分享的推演" fullScreen={false} />
  }

  if (replay.status === 'error' || !replay.data) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="暂时打不开这段推演"
        description={replay.error?.message}
        traceId={replay.error?.traceId}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={replay.reload}>
            重新加载
          </Button>
        }
      />
    )
  }

  if (replay.status === 'empty') {
    return (
      <AppStatusScreen
        icon={Info}
        title="这段分享里没有可回放的操作"
        description="分享者在这个场景里没有留下操作记录。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回登录
          </Button>
        }
      />
    )
  }

  if (!isBuiltinSimulationCode(replay.data.package_code)) {
    return (
      <AppStatusScreen
        icon={Lock}
        title="这段推演不支持公开回放"
        description="分享的是本校自建的仿真场景,需要登录校内账号后在仿真工作台中查看。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回登录
          </Button>
        }
      />
    )
  }

  return (
    <SimReplayRuntime
      key={shareCode}
      replay={replay.data}
      title={title}
      onExit={exit}
      action={<Button variant="primary" size="sm" leftIcon={LogIn} onClick={exit}>登录 Chaimir</Button>}
    />
  )
}
