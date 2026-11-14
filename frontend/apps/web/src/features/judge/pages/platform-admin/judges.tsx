// 判题器(平台侧栏,/platform-admin/judges)。设计 §7.6:
// 页头 +「登记判题器」;分段按类型(带数字,写进地址);列表行显示名称与短名、类型、是否需要链环境、
// 判题环境状态(未准备 / 已冻结 / 自检失败)、上次自测结果;整行打开判题器大抽屉,抽屉底部「运行自测」「保存」。
// 指标条、平铺属性的卡片与常驻说明卡去掉。判题器接口一次返回全部(不分页),计数在本页完成。

import { useMemo, useState } from 'react'
import { Cpu, Plus } from 'lucide-react'
import { JudgerSelftestStatus, JudgerType, type Judger } from '@chaimir/api-client'
import {
  Button,
  ListRow,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { judgerStatusLabel, judgerTypeLabel } from '../../../../utils/labels/judge'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { judgerEnvironment, judgerSelftest } from '../../judgerEnvironment'
import { JUDGER_TYPES } from '../../options'
import { getFrozenJudgerSnapshot } from '../../rules'
import { judgerStatusTone } from '../../statusPresentation'
import { JudgerFormModal } from './judger-form'

/** 地址参数:type 分段(all 为全部,其余为类型编号) */
const JUDGES_QUERY = {
  type: { defaultValue: 'all', allowed: ['all', ...JUDGER_TYPES.map(String)] },
} as const

/** PlatformJudgesPage 判题器清单:按类型分诊,整行打开详情、修改与自测 */
export default function PlatformJudgesPage() {
  const [query, setQuery] = useUrlState(JUDGES_QUERY)
  const [target, setTarget] = useState<{ judger?: Judger }>()
  const [testingId, setTestingId] = useState<string>()
  const judgers = useAsyncResource(
    () => api.judge.listJudgers(),
    [],
    (list) => list.length === 0
  )
  const all = useMemo(() => judgers.data ?? [], [judgers.data])
  const loaded = judgers.status === 'success' || judgers.status === 'empty'

  const counts = useMemo(() => {
    const result: Record<string, number> = { all: all.length }
    for (const type of JUDGER_TYPES)
      result[String(type)] = all.filter((item) => item.type === type).length
    return result
  }, [all])
  const visible =
    query.type === 'all' ? all : all.filter((item) => String(item.type) === query.type)

  /** runSelftest 按判题器声明真跑一次样例(会创建并回收判题环境),结果写回判题器 */
  const runSelftest = async (judger: Judger) => {
    setTestingId(judger.id)
    try {
      const updated = await api.judge.runJudgerSelftest(judger.id)
      toast.success(
        updated.selftest_status === JudgerSelftestStatus.PASSED
          ? `「${judger.name}」自测通过`
          : `「${judger.name}」自测已结束,请查看结果`
      )
      judgers.reload()
      setTarget((current) => (current?.judger?.id === judger.id ? { judger: updated } : current))
    } catch (error) {
      toast.error(userFacingErrorMessage(error, '自测没有跑完。请确认判题环境可用后重试。'))
    } finally {
      setTestingId(undefined)
    }
  }

  return (
    <PageScaffold>
      <PageHeader
        title="判题器"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            登记判题器
          </Button>
        }
      />

      <SegmentedControl
        aria-label="按判题器类型筛选"
        options={[
          { value: 'all', label: '全部', count: loaded ? counts.all : undefined },
          ...JUDGER_TYPES.map((type) => ({
            value: String(type),
            label: judgerTypeLabel(type),
            count: loaded ? counts[String(type)] : undefined,
          })),
        ]}
        value={query.type}
        onValueChange={(value) => setQuery({ type: value })}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <ResourceState
        resource={judgers}
        emptyIcon={Cpu}
        emptyTitle="还没有登记判题器"
        emptyDescription="登记判题器后,教师才能给题目配上自动判分方式。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            登记判题器
          </Button>
        }
        skeleton={<Skeleton variant="line" lines={6} />}
      >
        {() =>
          visible.length === 0 ? (
            <p className="rounded-lg bg-surface px-4 py-10 text-center text-sm text-ink-sub shadow-xs">
              这个类型下没有判题器。
            </p>
          ) : (
            <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
              {visible.map((judger) => (
                <li key={judger.id}>
                  <JudgerRow
                    judger={judger}
                    testing={testingId === judger.id}
                    onOpen={() => setTarget({ judger })}
                  />
                </li>
              ))}
            </ul>
          )
        }
      </ResourceState>

      {target ? (
        <JudgerFormModal
          key={target.judger?.id ?? 'new'}
          judger={target.judger}
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            judgers.reload()
          }}
          selftest={
            target.judger
              ? {
                  running: testingId === target.judger.id,
                  onRun: () => void runSelftest(target.judger!),
                  unavailableReason:
                    target.judger.type === JudgerType.MANUAL ? '人工评分不需要自测' : undefined,
                }
              : undefined
          }
        />
      ) : null}
    </PageScaffold>
  )
}

/** JudgerRow 一个判题器:名称;类型 · 短名 · 是否需要链环境 · 判题环境 · 上次自测;右侧状态 */
function JudgerRow({
  judger,
  testing,
  onOpen,
}: {
  judger: Judger
  testing: boolean
  onOpen: () => void
}) {
  const environment = judgerEnvironment({
    runtimeRequired: judger.runtime_required,
    frozen: getFrozenJudgerSnapshot(judger.resource_spec) !== undefined,
    selftest: judger.selftest_status,
  })
  const selftest = judgerSelftest({ type: judger.type, selftest: judger.selftest_status })
  return (
    <ListRow
      title={judger.name}
      meta={
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span>
            {judgerTypeLabel(judger.type)} · <span className="font-mono">{judger.code}</span> ·{' '}
            {judger.runtime_required ? '需要链环境' : '不需要链环境'}
          </span>
          {environment ? (
            <StatusIndicator appearance="text" tone={environment.tone} label={environment.label} />
          ) : null}
          <StatusIndicator
            appearance="text"
            tone={testing ? 'neutral' : selftest.tone}
            label={testing ? '正在自测' : selftest.label}
            loading={testing}
          />
        </span>
      }
      trailing={
        <StatusIndicator
          tone={judgerStatusTone(judger.status)}
          label={judgerStatusLabel(judger.status)}
        />
      }
      onSelect={onOpen}
    />
  )
}
