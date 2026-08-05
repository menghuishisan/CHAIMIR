// 漏洞题预验证(漏洞题工坊页内)。
//
// 预验证在隔离沙箱里跑两遍:正向执行 PoC 应让全部断言成立,反向不执行 PoC 应让全部断言不成立。
// 双向通过才算这道题「可解可判」,才能固化进题库。
//
// 运行时与镜像版本从 M2 编排目录里选(后端 validatePrevalidateRequest 两者必填),
// 不让教师手填 —— 拼错要等验证跑完才发现。
//
// 验证结果里的 actual 是后端已脱敏的短文本(chainassert.ShortJSON),
// 页面原样呈现即可,不再二次解析。

import { useCallback, useMemo, useState } from 'react'
import { CircleCheck, CircleX, FlaskConical, Server } from 'lucide-react'
import { VulnPrevalidateStatus, type VulnProblem } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Checkbox,
  DescriptionList,
  FormField,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Select,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useOrchestrationCatalog } from '../../../sandbox/useOrchestrationCatalog'
import {
  vulnLevelLabel,
  vulnPrevalidateStatusLabel,
  vulnPrevalidateStatusTone,
  vulnRuntimeModeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 验证明细里的两段:后端 runVulnPrevalidation 固定写入这两个键。 */
const DETAIL_POSITIVE = 'positive'
const DETAIL_NEGATIVE = 'negative'

/** 单段验证结果里的键。 */
const CASE_PASSED = 'passed'
const CASE_ASSERTIONS = 'assertions'

/** 单条断言结果里的键(后端 checkVulnAssertions 写入)。 */
const ASSERTION_CASE = 'case'
const ASSERTION_PASSED = 'passed'
const ASSERTION_EXPECTED = 'expected_label'
const ASSERTION_ACTUAL = 'actual'
const ASSERTION_HINT = 'hint'

export interface VulnPrevalidateModalProps {
  problem: VulnProblem
  onClose: () => void
  onDone: () => void
}

/**
 * VulnPrevalidateModal 触发预验证并呈现双向验证结果。
 */
export function VulnPrevalidateModal({ problem, onClose, onDone }: VulnPrevalidateModalProps) {
  const [runtimeCode, setRuntimeCode] = useState('')
  const [imageVersion, setImageVersion] = useState('')
  const [toolCodes, setToolCodes] = useState<string[]>([])
  const [result, setResult] = useState<VulnProblem>(problem)
  const [formError, setFormError] = useState<string>()
  const [running, setRunning] = useState(false)

  const catalog = useOrchestrationCatalog()
  const imageOptions = useMemo(() => catalog.imageOptions(runtimeCode), [catalog, runtimeCode])

  const run = useCallback(async () => {
    if (runtimeCode === '' || imageVersion === '') {
      setFormError('请选择验证要用的运行时与镜像版本')
      return
    }
    setFormError(undefined)
    setRunning(true)
    try {
      const updated = await api.contest.prevalidateVulnProblem(problem.id, {
        runtime_code: runtimeCode,
        runtime_image_version: imageVersion,
        tool_codes: toolCodes,
      })
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
  }, [imageVersion, onDone, problem.id, runtimeCode, toolCodes])

  const positive = readSection(result.prevalidate_detail, DETAIL_POSITIVE)
  const negative = readSection(result.prevalidate_detail, DETAIL_NEGATIVE)
  const detailError = readString(result.prevalidate_detail, 'error')

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>预验证漏洞题</ModalTitle>
          <ModalDescription>
            在隔离环境里跑两遍:执行攻击步骤时断言应全部成立,不执行时应全部不成立。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '漏洞题', description: result.title },
              { term: '可复现性', description: vulnLevelLabel(result.level) },
              { term: '复现方式', description: vulnRuntimeModeLabel(result.runtime_mode) },
              {
                term: '当前状态',
                description: vulnPrevalidateStatusLabel(result.prevalidate_status),
              },
            ]}
          />

          <ResourceState
            resource={catalog.resource}
            emptyIcon={Server}
            emptyTitle="平台还没有可用运行时"
            emptyDescription="请联系平台管理员在链运行时里注册并自检运行时。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() => (
              <div className="flex flex-col gap-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <FormField label="运行时" htmlFor="prevalidate-runtime" required>
                    <Select
                      id="prevalidate-runtime"
                      options={catalog.runtimeOptions}
                      value={runtimeCode}
                      placeholder="选择运行时"
                      onValueChange={(value) => {
                        setRuntimeCode(value)
                        setImageVersion('')
                      }}
                    />
                  </FormField>
                  <FormField label="镜像版本" htmlFor="prevalidate-image" required>
                    <Select
                      id="prevalidate-image"
                      options={imageOptions}
                      value={imageVersion}
                      placeholder={
                        runtimeCode === ''
                          ? '请先选择运行时'
                          : imageOptions.length > 0
                            ? '选择镜像版本'
                            : '该运行时暂无镜像'
                      }
                      disabled={imageOptions.length === 0}
                      onValueChange={setImageVersion}
                    />
                  </FormField>
                </div>

                {catalog.tools.length > 0 ? (
                  <FormField label="验证时可用工具" helper="按攻击步骤的需要勾选;不确定就不勾">
                    <div className="flex flex-col gap-2">
                      {catalog.tools.map((tool) => (
                        <Checkbox
                          key={tool.code}
                          checked={toolCodes.includes(tool.code)}
                          label={tool.name}
                          onCheckedChange={(checked) =>
                            setToolCodes((current) =>
                              checked === true
                                ? [...current, tool.code]
                                : current.filter((code) => code !== tool.code),
                            )
                          }
                        />
                      ))}
                    </div>
                  </FormField>
                ) : null}
              </div>
            )}
          </ResourceState>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {result.prevalidate_status === VulnPrevalidateStatus.PENDING &&
          Object.keys(result.prevalidate_detail).length === 0 ? (
            <Callout tone="info">
              还没有验证过。选好运行环境后开始验证,过程会真实起一个隔离沙箱,用后即毁。
            </Callout>
          ) : (
            <div className="flex flex-col gap-4">
              <StatusIndicator
                tone={vulnPrevalidateStatusTone(result.prevalidate_status)}
                label={vulnPrevalidateStatusLabel(result.prevalidate_status)}
              />

              {detailError ? (
                <Callout tone="danger" title="验证过程没有跑完">
                  {detailError}
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
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          <Button variant="seal" leftIcon={FlaskConical} loading={running} onClick={() => void run()}>
            {Object.keys(result.prevalidate_detail).length === 0 ? '开始验证' : '重新验证'}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
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
      <div className="rounded-md border border-line bg-surface-sunken p-4">
        <p className="text-base text-ink">{title}</p>
        <p className="text-sm text-ink-sub">这一段没有执行到。</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3 rounded-md border border-line bg-surface-sunken p-4">
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
        <div className="flex flex-col gap-2">
          {section.assertions.map((assertion, index) => (
            <div key={index} className="flex flex-col gap-1 rounded-md border border-line bg-surface p-3">
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
                <p className="truncate font-mono text-xs text-ink-faint">实际:{assertion.actual}</p>
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
  key: string,
): { passed: boolean; assertions: AssertionResult[] } | undefined {
  const raw = detail[key]
  if (typeof raw !== 'object' || raw === null) return undefined
  const section = raw as Record<string, unknown>
  if (Object.keys(section).length === 0) return undefined
  return {
    passed: section[CASE_PASSED] === true,
    assertions: readAssertions(section[CASE_ASSERTIONS]),
  }
}

/** readAssertions 把断言结果数组转成有类型的行;非数组回空数组。 */
function readAssertions(raw: unknown): AssertionResult[] {
  if (!Array.isArray(raw)) return []
  return raw
    .filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
    .map((item) => ({
      name: readString(item, ASSERTION_CASE),
      passed: item[ASSERTION_PASSED] === true,
      expected: readString(item, ASSERTION_EXPECTED),
      actual: readString(item, ASSERTION_ACTUAL),
      hint: readString(item, ASSERTION_HINT),
    }))
}

/** readString 从开放对象里读字符串字段;非字符串回空串。 */
function readString(source: Record<string, unknown>, key: string): string {
  const value = source[key]
  return typeof value === 'string' ? value : ''
}
