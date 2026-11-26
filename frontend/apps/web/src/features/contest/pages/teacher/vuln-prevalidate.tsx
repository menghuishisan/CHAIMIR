// 漏洞题预验证抽屉；初始化随组合执行，只有正向验证追加官方攻击。

import { useState } from 'react'
import { CircleCheck, CircleX, FlaskConical } from 'lucide-react'
import {
  VulnProblemStatus,
  VulnPrevalidateStatus,
  type VulnProblem,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DescriptionList,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import {
  vulnLevelLabel,
  vulnPrevalidateStatusLabel,
  vulnRuntimeModeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { vulnPrevalidateStatusTone } from '../../statusPresentation'
import { readString } from '../../jsonReaders'

export interface VulnPrevalidateModalProps {
  problem: VulnProblem
  onClose: () => void
  onDone: () => void
}

/** isVulnPrevalidateDetailConsistent 校验状态是否与正反向断言明细一致。 */
export function isVulnPrevalidateDetailConsistent(problem: VulnProblem): boolean {
  if (problem.prevalidate_status !== VulnPrevalidateStatus.PASSED) return false
  const positive = readSection(problem.prevalidate_detail, 'positive')
  const negative = readSection(problem.prevalidate_detail, 'negative')
  return positive?.passed === true && negative?.passed === true
}

/**
 * VulnPrevalidateModal 触发预验证并呈现双向验证结果。
 */
export function VulnPrevalidateModal({ problem, onClose, onDone }: VulnPrevalidateModalProps) {
  const [result, setResult] = useState<VulnProblem>(problem)
  const [formError, setFormError] = useState<string>()
  const [running, setRunning] = useState(false)

  async function run() {
    setFormError(undefined)
    setRunning(true)
    try {
      const updated = await api.contest.prevalidateVulnProblem(problem.id)
      setResult(updated)
      if (updated.prevalidate_status === VulnPrevalidateStatus.PASSED) {
        toast.success('双向验证通过,可以固化到题库了')
      }
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '验证没有完成,请稍后重试。'))
    } finally {
      setRunning(false)
    }
  }

  const positive = readSection(result.prevalidate_detail, 'positive')
  const negative = readSection(result.prevalidate_detail, 'negative')
  const detailError = readString(result.prevalidate_detail, 'error')
  const detailTraceId = readString(result.prevalidate_detail, 'trace_id')
  const detailConsistent = isVulnPrevalidateDetailConsistent(result)
  const canRun = result.status === VulnProblemStatus.DRAFT
  const displayStatus =
    result.prevalidate_status === VulnPrevalidateStatus.PASSED && !detailConsistent
      ? VulnPrevalidateStatus.FAILED
      : result.prevalidate_status

  return (
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent size="wide">
        <DrawerHeader>
          <DrawerTitle>预验证漏洞题</DrawerTitle>
          <DrawerDescription>
            在隔离环境里跑两遍:执行攻击步骤时断言应全部成立,不执行时应全部不成立。
          </DrawerDescription>
        </DrawerHeader>
        <DrawerBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '漏洞题', description: result.title },
              { term: '可复现性', description: vulnLevelLabel(result.level) },
              { term: '复现方式', description: vulnRuntimeModeLabel(result.runtime_mode) },
              {
                term: '当前状态',
                description: vulnPrevalidateStatusLabel(displayStatus),
              },
            ]}
          />

          <Callout tone="info">验证将读取服务器上已保存的题目与运行环境。需要调整时，请先编辑草稿并保存。</Callout>

          {running ? (
            <Callout tone="info" title="正在验证">
              正在准备隔离环境并依次跑正向、反向两遍,通常需要一两分钟。完成后结果会显示在下方。
            </Callout>
          ) : null}
          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {result.prevalidate_status === VulnPrevalidateStatus.PENDING &&
          Object.keys(result.prevalidate_detail).length === 0 ? (
            <Callout tone="info">
              还没有验证过。请先在草稿中保存运行环境，再开始验证。
            </Callout>
          ) : (
            <div className="flex flex-col gap-4">
              <StatusIndicator
                tone={vulnPrevalidateStatusTone(displayStatus)}
                label={vulnPrevalidateStatusLabel(displayStatus)}
              />

              {result.prevalidate_status === VulnPrevalidateStatus.PASSED && !detailConsistent ? (
                <Callout tone="warning" title="验证记录需要重新确认">
                  当前状态与正反向断言结果不一致,请重新验证后再固化题目。
                </Callout>
              ) : null}

              {detailError ? (
                <Callout tone="danger" title="验证过程没有跑完">
                  <p>{detailError}</p>
                  {detailTraceId ? (
                    <p className="mt-1 text-xs">如需帮助,请提供编号 {detailTraceId}</p>
                  ) : null}
                </Callout>
              ) : null}

              <ValidationSection
                title="正向验证"
                description="执行攻击步骤后,断言应当全部成立 —— 证明这个漏洞确实可以被利用。"
                expectPassed
                section={positive}
              />
              <ValidationSection
                title="反向验证"
                description="不执行攻击步骤时,断言应当全部不成立 —— 证明判定不会误判正常状态。"
                expectPassed={false}
                section={negative}
              />
            </div>
          )}
        </DrawerBody>
        <DrawerFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          {canRun ? (
            <Button
              variant="primary"
              leftIcon={FlaskConical}
              loading={running}
              onClick={() => void run()}
            >
              {Object.keys(result.prevalidate_detail).length === 0 ? '开始验证' : '重新验证'}
            </Button>
          ) : null}
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}

/** AssertionResult 是一条断言的验证结果。 */
interface AssertionResult {
  name: string
  passed: boolean
  expected: string
  actual: string
  hint: string
}

interface ValidationSectionProps {
  title: string
  description: string
  /** 这一段期望断言成立还是不成立(仅用于文案,通过与否由后端判定) */
  expectPassed: boolean
  section: { passed: boolean; assertions: AssertionResult[] } | undefined
}

/**
 * ValidationSection 渲染单向验证结果。
 * passed 已由后端按方向换算(反向验证里"断言不成立"即 passed=true),故这里不再取反。
 */
function ValidationSection({ title, description, expectPassed, section }: ValidationSectionProps) {
  if (section === undefined) {
    return (
      <div className="rounded-md bg-surface-hover p-4">
        <p className="text-base text-ink">{title}</p>
        <p className="text-sm text-ink-sub">这一段没有执行到。</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3 rounded-md bg-surface-hover p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-base text-ink">{title}</p>
          <p className="text-sm text-ink-sub">{description}</p>
        </div>
        <Badge tone={section.passed ? 'success' : 'danger'}>
          {section.passed ? '符合预期' : '不符合预期'}
        </Badge>
      </div>

      {section.assertions.length === 0 ? (
        <p className="text-sm text-ink-sub">这一段没有断言结果。</p>
      ) : (
        // 井内的并列条目用分隔线区分,不再各自画盒(规范 §6.5.1)
        <div className="flex flex-col divide-y divide-line">
          {section.assertions.map((assertion, index) => (
            <div key={index} className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="min-w-0 truncate text-sm text-ink">
                  {assertion.name || `断言 ${index + 1}`}
                </span>
                <StatusIndicator
                  tone={assertion.passed ? 'success' : 'danger'}
                  label={
                    assertion.passed
                      ? expectPassed
                        ? '成立'
                        : '未成立(符合预期)'
                      : expectPassed
                        ? '未成立'
                        : '成立(不符合预期)'
                  }
                />
              </div>
              {assertion.expected ? (
                <p className="text-xs text-ink-sub">期望:{assertion.expected}</p>
              ) : null}
              {assertion.actual ? (
                <p className="truncate font-mono text-xs text-ink-sub">实际:{assertion.actual}</p>
              ) : null}
              {!assertion.passed && assertion.hint ? (
                <p className="text-xs text-warning">{assertion.hint}</p>
              ) : null}
            </div>
          ))}
        </div>
      )}

      {section.passed ? (
        <p className="flex items-center gap-1.5 text-xs text-success">
          <CircleCheck aria-hidden className="size-3.5" />
          这一向验证通过
        </p>
      ) : (
        <p className="flex items-center gap-1.5 text-xs text-danger">
          <CircleX aria-hidden className="size-3.5" />
          这一向没通过,按提示修改草稿后重新验证
        </p>
      )}
    </div>
  )
}

/** readSection 从验证明细里读出一段结果;缺失或形状不符回 undefined。 */
function readSection(
  detail: Record<string, unknown>,
  key: string
): { passed: boolean; assertions: AssertionResult[] } | undefined {
  const raw = detail[key]
  if (typeof raw !== 'object' || raw === null) return undefined
  const section = raw as Record<string, unknown>
  if (Object.keys(section).length === 0) return undefined
  const assertions = readAssertions(section.assertions)
  return { passed: assertions.length > 0 && assertions.every((item) => item.passed), assertions }
}

/** readAssertions 把断言结果数组转成有类型的行;非数组回空数组。 */
function readAssertions(raw: unknown): AssertionResult[] {
  if (!Array.isArray(raw)) return []
  return raw
    .filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
    .map((item) => ({
      name: readString(item, 'case'),
      passed: item.passed === true,
      expected: readString(item, 'expected_label'),
      actual: readString(item, 'actual'),
      hint: readString(item, 'hint'),
    }))
}
