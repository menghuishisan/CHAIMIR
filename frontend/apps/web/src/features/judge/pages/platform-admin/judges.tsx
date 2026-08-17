// 判题器页(平台侧栏,/platform-admin/judges)。
//
// 判题器是「怎么判一道题」的实现登记:测试用例、链上断言、flag 比对、静态扫描、
// 仿真检查点、人工评分六类。教师出题时按编码引用,平台负责登记与自测。
//
// 自测通过才代表这个判题器真能跑;自测状态由后端在自测后写入判题器记录。
// 判题任务列表不在这里:GET /judge/tasks 在 teacher 组,平台身份调用会被拒 ——
// 任务与重判归教师端批改中心(对齐清单 §3.4)。

import { useCallback, useMemo, useState } from 'react'
import { Cpu, Plus, Settings2, ShieldCheck, Timer } from 'lucide-react'
import { JudgerStatus, JudgerType, type Judger } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FilterBar,
  FilterField,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  Stat,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  JUDGER_TYPES,
  judgerStatusLabel,
  judgerStatusTone,
  judgerTypeLabel,
} from '../../../../utils/labels/judge'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { JudgerFormModal } from './judger-form'

/** 类型筛选项:值为空串表示不过滤。 */
const TYPE_FILTERS = [
  { value: '', label: '全部' },
  ...JUDGER_TYPES.map((type) => ({ value: String(type), label: judgerTypeLabel(type) })),
] as const

/** 各类判题器的一句话说明:说明它判什么,不解释内部实现。 */
const TYPE_HINTS: Record<JudgerType, string> = {
  [JudgerType.TESTCASE]: '跑测试用例,按通过的用例数给分',
  [JudgerType.ONCHAIN_ASSERT]: '在链上执行断言,检查合约状态是否符合预期',
  [JudgerType.FLAG]: '比对学生提交的答案口令,用于夺旗类赛题',
  [JudgerType.STATIC_SCAN]: '静态扫描代码,查找漏洞模式与写法问题',
  [JudgerType.SIM_CHECKPOINT]: '按仿真过程中的检查点给分',
  [JudgerType.MANUAL]: '不自动判,由教师在批改中心人工打分',
}

/**
 * PlatformJudgesPage 列出判题器并承载登记、修改与自测。
 */
export default function PlatformJudgesPage() {
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [formTarget, setFormTarget] = useState<{ judger?: Judger } | undefined>()
  const [actionError, setActionError] = useState<string>()

  const judgers = useAsyncResource(() => api.judge.listJudgers(), [], (value) => value.length === 0)

  const list = useMemo(() => judgers.data ?? [], [judgers.data])

  const visible = useMemo(
    () => (typeFilter ? list.filter((item) => item.type === Number(typeFilter)) : list),
    [list, typeFilter],
  )

  const stats = useMemo(
    () => ({
      available: list.filter((item) => item.status === JudgerStatus.AVAILABLE).length,
      runtimeBound: list.filter((item) => item.runtime_required).length,
      manual: list.filter((item) => item.type === JudgerType.MANUAL).length,
    }),
    [list],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '判题器' }]} />}
        title="判题器"
        description="判一道题用哪种方式。教师出题时按短名引用,登记后建议先自测一次再开放。"
        icon={Cpu}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            登记判题器
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="判题器总数" value={list.length} icon={Cpu} />
          <Stat label="可用" value={stats.available} icon={ShieldCheck} hint="教师可以引用" />
          <Stat label="需要链环境" value={stats.runtimeBound} icon={Cpu} hint="判题时会准备实验环境" />
          <Stat label="人工评分" value={stats.manual} icon={Settings2} hint="由教师打分" />
        </div>
      </PageSection>

      <PageSection
        title="判题器列表"
        description="按类型筛选。停用的判题器不会分配新任务,已排队的任务照旧执行完。"
      >
        <div className="flex flex-col gap-4">
          <FilterBar label="判题器筛选">
            <FilterField label="判题器类型" group>
              <SegmentedControl
                aria-label="按判题器类型筛选"
                size="sm"
                options={TYPE_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={typeFilter}
                onValueChange={setTypeFilter}
              />
            </FilterField>
          </FilterBar>

          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={judgers}
            emptyIcon={Cpu}
            emptyTitle="还没有登记判题器"
            emptyDescription="登记判题器后,教师才能给题目配上自动判分方式。"
            emptyAction={
              <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                登记判题器
              </Button>
            }
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {() =>
              visible.length === 0 ? (
                <Callout tone="info">这个类型下没有判题器,换个类型看看。</Callout>
              ) : (
                <div className="grid gap-4 lg:grid-cols-2">
                  {visible.map((judger) => (
                    <JudgerCard
                      key={judger.id}
                      judger={judger}
                      onEdit={() => setFormTarget({ judger })}
                      onSelftested={judgers.reload}
                      onError={setActionError}
                    />
                  ))}
                </div>
              )
            }
          </ResourceState>

          <Callout tone="info">
            人工评分的判题器不执行任何命令,任务会一直停在待评分状态,等教师在批改中心打分。
          </Callout>
        </div>
      </PageSection>

      {formTarget ? (
        <JudgerFormModal
          judger={formTarget.judger}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            judgers.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface JudgerCardProps {
  judger: Judger
  onEdit: () => void
  onSelftested: () => void
  onError: (message: string) => void
}

/**
 * JudgerCard 展示单个判题器的运行约束并承载自测。
 * 自测会按判题器声明真跑一次样例,耗时较长,故按钮带 loading。
 */
function JudgerCard({ judger, onEdit, onSelftested, onError }: JudgerCardProps) {
  const [testing, setTesting] = useState(false)

  const runSelftest = useCallback(async () => {
    setTesting(true)
    try {
      await api.judge.runJudgerSelftest(judger.id)
      toast.success('自测已完成,结果已更新')
      onSelftested()
    } catch (error) {
      onError(
        userFacingErrorMessage(
          error,
          '自测没有跑完。请确认执行器与链环境声明可用,然后重试。',
        ),
      )
    } finally {
      setTesting(false)
    }
  }, [judger.id, onError, onSelftested])

  const spec = judger.resource_spec

  const items = useMemo(() => {
    const base = [
      { term: '判题器短名', description: judger.code, mono: true },
      { term: '判题实现名称', description: judger.executor_ref, mono: true },
      {
        term: '默认超时',
        description: `${judger.default_timeout_sec} 秒${
          spec.timeout_sec && spec.timeout_sec > 0 ? `(本判题器覆盖为 ${spec.timeout_sec} 秒)` : ''
        }`,
      },
    ]
    if (judger.runtime_required || spec.runtime_code) {
      base.push({
        term: '链环境',
        description: spec.runtime_code
          ? `${spec.runtime_code} · ${spec.runtime_image_version ?? '未指定版本'}`
          : '需要链环境但未指定',
        mono: true,
      })
    }
    if (spec.tool_codes && spec.tool_codes.length > 0) {
      base.push({ term: '需要的工具', description: spec.tool_codes.join('、') })
    }
    if (spec.command && spec.command.length > 0) {
      base.push({ term: '执行命令', description: spec.command.join(' '), mono: true })
    }
    if (spec.max_retries && spec.max_retries > 0) {
      base.push({ term: '失败重试', description: `最多 ${spec.max_retries} 次` })
    }
    if (judger.updated_at) {
      base.push({ term: '最近更新', description: formatDateTime(judger.updated_at), mono: true })
    }
    return base
  }, [judger, spec])

  return (
    <Card>
      <CardHeader
        title={judger.name}
        description={judgerTypeLabel(judger.type)}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {judger.runtime_required ? <Badge tone="neutral">需要实验环境</Badge> : null}
            <StatusIndicator
              tone={judgerStatusTone(judger.status)}
              label={judgerStatusLabel(judger.status)}
            />
          </div>
        }
      />
      <CardBody className="flex flex-col gap-3">
        <p className="text-sm text-ink-sub">{TYPE_HINTS[judger.type]}</p>
        <DescriptionList dense items={items} />
        <div className="flex flex-wrap items-center gap-2">
          {judger.type === JudgerType.MANUAL ? (
            <span className="inline-flex items-center gap-1 text-sm text-ink-sub">
              <Timer aria-hidden="true" className="size-4" />
              人工评分不需要自测
            </span>
          ) : (
            <Button
              variant="outline"
              size="sm"
              leftIcon={ShieldCheck}
              loading={testing}
              onClick={() => void runSelftest()}
            >
              运行自测
            </Button>
          )}
          <Button variant="ghost" size="sm" leftIcon={Settings2} onClick={onEdit}>
            修改配置
          </Button>
        </div>
      </CardBody>
    </Card>
  )
}
