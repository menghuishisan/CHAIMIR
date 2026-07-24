// ContentBodyEditor 按内容类型提供结构化正文、判题配置和答案编辑，不暴露 JSON 存储格式。

import React from 'react'
import type { ChainAssertion, ContentBody, ContentJudgeConfig, ContestProblemBody, ExperimentTemplateBody, TheoryQuestionBody } from '@chaimir/api-client'
import { ContentType } from '@chaimir/api-client'
import { Button, Checkbox, Input, Select, Textarea, FormField } from '@chaimir/ui'
import { Plus, Trash2 } from 'lucide-react'
import styles from '../pages/content.module.css'

export interface ContentBodyEditorProps {
  type: ContentType
  value: ContentBody
  onChange: (value: ContentBody) => void
}

const assertionOperatorOptions = [
  { value: 'eq', label: '等于' }, { value: 'neq', label: '不等于' }, { value: 'gt', label: '大于' },
  { value: 'gte', label: '大于或等于' }, { value: 'lt', label: '小于' }, { value: 'lte', label: '小于或等于' },
  { value: 'contains', label: '包含' },
]

const questionTypeOptions = [
  { value: 'single_choice', label: '单选题' }, { value: 'multiple_choice', label: '多选题' },
  { value: 'true_false', label: '判断题' }, { value: 'fill_blank', label: '填空题' },
  { value: 'short_answer', label: '简答题' },
]

/** createContentBody 为切换后的内容类型生成唯一合法初始结构。 */
export function createContentBody(type: ContentType): ContentBody {
  const judge_config: ContentJudgeConfig = { judger_code: '', max_score: 100, expectation: { public: false, assertions: [] } }
  if (type === ContentType.EXPERIMENT_TEMPLATE) {
    return { runtime_code: '', tools: [], init_code_ref: '', sim_package_ref: '', judge_config, description: '', init_script: '' }
  }
  if (type === ContentType.CONTEST_PROBLEM) return { statement: '', judge_config, init_contracts: [] }
  return { statement: '', q_type: 'single_choice', options: ['', ''], answer: '', explanation: '' }
}

/** ContentBodyEditor 根据互斥内容类型渲染对应领域表单。 */
export function ContentBodyEditor({ type, value, onChange }: ContentBodyEditorProps): React.ReactElement {
  if (type === ContentType.EXPERIMENT_TEMPLATE) {
    const body = value as ExperimentTemplateBody
    return (
      <div className={styles.bodyEditor}>
        <FormField className={styles.fieldFull} label="实验说明"><Textarea rows={6} value={body.description} onChange={(event) => onChange({ ...body, description: event.target.value })} /></FormField>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="运行环境"><Input fullWidth value={body.runtime_code} onChange={(event) => onChange({ ...body, runtime_code: event.target.value })} /></FormField>
          <FormField className={styles.field} label="仿真包引用"><Input fullWidth value={body.sim_package_ref} onChange={(event) => onChange({ ...body, sim_package_ref: event.target.value })} /></FormField>
          <FormField className={styles.field} label="辅助工具"><Input fullWidth value={body.tools.join(', ')} onChange={(event) => onChange({ ...body, tools: splitList(event.target.value) })} placeholder="多个工具用逗号分隔" /></FormField>
          <FormField className={styles.field} label="初始化代码引用"><Input fullWidth value={body.init_code_ref} onChange={(event) => onChange({ ...body, init_code_ref: event.target.value })} /></FormField>
        </div>
        <FormField className={styles.fieldFull} label="初始化脚本"><Textarea rows={6} value={body.init_script} onChange={(event) => onChange({ ...body, init_script: event.target.value })} /></FormField>
        <JudgeConfigEditor value={body.judge_config} onChange={(judge_config) => onChange({ ...body, judge_config })} />
      </div>
    )
  }

  if (type === ContentType.CONTEST_PROBLEM) {
    const body = value as ContestProblemBody
    const adConfig = body.ad_config
    return (
      <div className={styles.bodyEditor}>
        <FormField className={styles.fieldFull} label="题面"><Textarea rows={8} value={body.statement} onChange={(event) => onChange({ ...body, statement: event.target.value })} /></FormField>
        <FormField className={styles.fieldFull} label="初始化合约引用"><Input fullWidth value={body.init_contracts.join(', ')} onChange={(event) => onChange({ ...body, init_contracts: splitList(event.target.value) })} placeholder="多个引用用逗号分隔" /></FormField>
        <Checkbox label="启用对抗赛环境" checked={Boolean(adConfig)} onChange={(event) => onChange({ ...body, ad_config: event.target.checked ? { runtime_code: '', runtime_image_version: '', tool_codes: [] } : undefined })} />
        {adConfig && (
          <div className={styles.formGrid}>
            <FormField className={styles.field} label="运行环境"><Input fullWidth value={adConfig.runtime_code} onChange={(event) => onChange({ ...body, ad_config: { ...adConfig, runtime_code: event.target.value } })} /></FormField>
            <FormField className={styles.field} label="环境版本"><Input fullWidth value={adConfig.runtime_image_version} onChange={(event) => onChange({ ...body, ad_config: { ...adConfig, runtime_image_version: event.target.value } })} /></FormField>
            <FormField className={styles.field} label="辅助工具"><Input fullWidth value={adConfig.tool_codes.join(', ')} onChange={(event) => onChange({ ...body, ad_config: { ...adConfig, tool_codes: splitList(event.target.value) } })} /></FormField>
          </div>
        )}
        <JudgeConfigEditor value={body.judge_config} onChange={(judge_config) => onChange({ ...body, judge_config })} />
      </div>
    )
  }

  const body = value as TheoryQuestionBody
  const choiceQuestion = body.q_type === 'single_choice' || body.q_type === 'multiple_choice'
  return (
    <div className={styles.bodyEditor}>
      <FormField className={styles.fieldFull} label="题干"><Textarea rows={7} value={body.statement} onChange={(event) => onChange({ ...body, statement: event.target.value })} /></FormField>
      <FormField className={styles.field} label="题型"><Select fullWidth value={body.q_type} options={questionTypeOptions} onChange={(qType) => onChange({ ...body, q_type: qType as TheoryQuestionBody['q_type'], options: qType === 'single_choice' || qType === 'multiple_choice' ? body.options : [] })} /></FormField>
      {choiceQuestion && <StringRows label="选项" values={body.options} onChange={(options) => onChange({ ...body, options })} />}
      {body.q_type === 'true_false' ? (
        <FormField className={styles.field} label="正确答案"><Select fullWidth value={String(body.answer)} options={[{ value: 'true', label: '正确' }, { value: 'false', label: '错误' }]} onChange={(answer) => onChange({ ...body, answer: answer === 'true' })} /></FormField>
      ) : body.q_type === 'multiple_choice' ? (
        <FormField className={styles.fieldFull} label="正确选项"><Input fullWidth value={Array.isArray(body.answer) ? body.answer.join(', ') : ''} onChange={(event) => onChange({ ...body, answer: splitList(event.target.value) })} placeholder="填写选项内容，多个答案用逗号分隔" /></FormField>
      ) : (
        <FormField className={styles.fieldFull} label="参考答案"><Textarea rows={4} value={typeof body.answer === 'string' ? body.answer : ''} onChange={(event) => onChange({ ...body, answer: event.target.value })} /></FormField>
      )}
      <FormField className={styles.fieldFull} label="答案解析"><Textarea rows={5} value={body.explanation} onChange={(event) => onChange({ ...body, explanation: event.target.value })} /></FormField>
    </div>
  )
}

/** JudgeConfigEditor 编辑判题器、资源、分值和链上断言。 */
function JudgeConfigEditor({ value, onChange }: { value: ContentJudgeConfig; onChange: (value: ContentJudgeConfig) => void }): React.ReactElement {
  const assertions = value.expectation.assertions || []
  return (
    <div className={styles.editorSection}>
      <h3>判题配置</h3>
      <div className={styles.formGrid}>
        <FormField className={styles.field} label="判题器"><Input fullWidth value={value.judger_code} onChange={(event) => onChange({ ...value, judger_code: event.target.value })} /></FormField>
        <FormField className={styles.field} label="测试资源引用"><Input fullWidth value={value.suite_ref || ''} onChange={(event) => onChange({ ...value, suite_ref: event.target.value || undefined })} /></FormField>
        <FormField className={styles.field} label="满分"><Input fullWidth type="number" min={1} step={1} value={value.max_score} onChange={(event) => onChange({ ...value, max_score: Number(event.target.value) })} /></FormField>
      </div>
      <Checkbox label="允许向学生公开判题摘要" checked={Boolean(value.expectation.public)} onChange={(event) => onChange({ ...value, expectation: { ...value.expectation, public: event.target.checked } })} />
      <div className={styles.editorHeading}><h3>链上断言</h3><Button type="button" variant="outline" size="sm" icon={<Plus size={14} />} onClick={() => onChange({ ...value, expectation: { ...value.expectation, assertions: [...assertions, emptyAssertion()] } })}>添加断言</Button></div>
      {assertions.map((assertion, index) => (
        <div className={styles.assertionRow} key={index}>
          <Input aria-label={`断言 ${index + 1} 名称`} value={assertion.label} onChange={(event) => replaceAssertion(value, index, { ...assertion, label: event.target.value }, onChange)} placeholder="断言名称" />
          <Input aria-label={`断言 ${index + 1} 查询目标`} value={assertion.target} onChange={(event) => replaceAssertion(value, index, { ...assertion, target: event.target.value }, onChange)} placeholder="查询目标" />
          <Input aria-label={`断言 ${index + 1} 字段`} value={assertion.field} onChange={(event) => replaceAssertion(value, index, { ...assertion, field: event.target.value }, onChange)} placeholder="结果字段" />
          <Select aria-label={`断言 ${index + 1} 比较方式`} value={assertion.op} options={assertionOperatorOptions} onChange={(op) => replaceAssertion(value, index, { ...assertion, op: op as ChainAssertion['op'] }, onChange)} />
          <Input aria-label={`断言 ${index + 1} 期望值`} value={String(assertion.value)} onChange={(event) => replaceAssertion(value, index, { ...assertion, value: scalarValue(event.target.value) }, onChange)} placeholder="期望值" />
          <Input aria-label={`断言 ${index + 1} 期望说明`} value={assertion.expected_label} onChange={(event) => replaceAssertion(value, index, { ...assertion, expected_label: event.target.value }, onChange)} placeholder="期望说明" />
          <Button type="button" variant="ghost" size="sm" icon={<Trash2 size={14} />} aria-label={`删除断言 ${index + 1}`} onClick={() => onChange({ ...value, expectation: { ...value.expectation, assertions: assertions.filter((_, rowIndex) => rowIndex !== index) } })} />
        </div>
      ))}
    </div>
  )
}

/** StringRows 编辑同构字符串列表。 */
function StringRows({ label, values, onChange }: { label: string; values: string[]; onChange: (values: string[]) => void }): React.ReactElement {
  return <div className={styles.editorSection}>
    <div className={styles.editorHeading}><h3>{label}</h3><Button type="button" variant="outline" size="sm" icon={<Plus size={14} />} onClick={() => onChange([...values, ''])}>添加选项</Button></div>
    {values.map((value, index) => <div className={styles.stringRow} key={index}><Input aria-label={`${label} ${index + 1}`} fullWidth value={value} onChange={(event) => onChange(values.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} /><Button type="button" variant="ghost" size="sm" icon={<Trash2 size={14} />} aria-label={`删除${label} ${index + 1}`} onClick={() => onChange(values.filter((_, itemIndex) => itemIndex !== index))} /></div>)}
  </div>
}

/** replaceAssertion 替换指定断言并保持其他行顺序。 */
function replaceAssertion(config: ContentJudgeConfig, index: number, assertion: ChainAssertion, onChange: (value: ContentJudgeConfig) => void): void {
  const assertions = (config.expectation.assertions || []).map((item, itemIndex) => itemIndex === index ? assertion : item)
  onChange({ ...config, expectation: { ...config.expectation, assertions } })
}

/** emptyAssertion 创建一条可填写的链上断言。 */
function emptyAssertion(): ChainAssertion { return { label: '', target: '', field: '', op: 'eq', value: '', expected_label: '' } }

/** splitList 把用户输入的逗号列表归一为非空字符串数组。 */
function splitList(value: string): string[] { return value.split(/[,，]/).map((item) => item.trim()).filter(Boolean) }

/** scalarValue 把布尔和数值文本转为断言实际值，其余保留字符串。 */
function scalarValue(value: string): string | number | boolean {
  if (value === 'true') return true
  if (value === 'false') return false
  return value.trim() !== '' && Number.isFinite(Number(value)) ? Number(value) : value
}
