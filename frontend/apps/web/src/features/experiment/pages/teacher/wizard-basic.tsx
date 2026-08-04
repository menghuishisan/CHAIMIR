// 实验编排向导:基础信息步(第 1 步)。
// 名称、说明、完成方式、是否需要报告与小组规模 —— 这些决定后续步骤的可选项
// (独立完成时不需要小组规模,故按完成方式显隐)。

import { useMemo } from 'react'
import { ExperimentCollabMode } from '@chaimir/api-client'
import {
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  FormField,
  Input,
  SegmentedControl,
  Select,
  Textarea,
} from '@chaimir/ui'
import type { Course } from '@chaimir/api-client'
import { experimentCollabModeLabel } from '../../../../utils/labels/experiment'
import type { ExperimentDraft } from './wizard-state'

export interface WizardBasicStepProps {
  draft: ExperimentDraft
  /** 可挂载的课程清单:实验可以不挂课程(独立实验),故选项含「不挂课程」 */
  courses: Course[]
  errors: Record<string, string | null>
  onChange: (patch: Partial<ExperimentDraft>) => void
}

/**
 * WizardBasicStep 渲染基础信息表单。
 */
export function WizardBasicStep({ draft, courses, errors, onChange }: WizardBasicStepProps) {
  const courseOptions = useMemo(
    () => [
      { value: '', label: '不挂课程(独立实验)' },
      ...courses.map((course) => ({ value: course.id, label: course.name })),
    ],
    [courses],
  )

  const isGroup = draft.collab_mode === ExperimentCollabMode.GROUP

  return (
    <Card>
      <CardHeader title="基础信息" description="这些内容会显示在学生的实验列表与详情页。" />
      <CardBody className="flex flex-col gap-4">
        <FormField label="实验名称" htmlFor="wizard-name" required error={errors.name}>
          <Input
            id="wizard-name"
            value={draft.name}
            invalid={Boolean(errors.name)}
            onChange={(event) => onChange({ name: event.target.value })}
          />
        </FormField>

        <FormField
          label="实验说明"
          htmlFor="wizard-description"
          helper="告诉学生这个实验要做什么、达到什么目标"
          error={errors.description}
        >
          <Textarea
            id="wizard-description"
            value={draft.description}
            rows={4}
            invalid={Boolean(errors.description)}
            onChange={(event) => onChange({ description: event.target.value })}
          />
        </FormField>

        <FormField label="所属课程" htmlFor="wizard-course" helper="挂到课程后可在课时里引用这个实验">
          <Select
            id="wizard-course"
            options={courseOptions}
            value={draft.course_id ?? ''}
            onValueChange={(value) => onChange({ course_id: value === '' ? undefined : value })}
          />
        </FormField>

        <FormField label="完成方式" required>
          <SegmentedControl
            aria-label="实验完成方式"
            options={[
              {
                value: String(ExperimentCollabMode.SOLO),
                label: experimentCollabModeLabel(ExperimentCollabMode.SOLO),
              },
              {
                value: String(ExperimentCollabMode.GROUP),
                label: experimentCollabModeLabel(ExperimentCollabMode.GROUP),
              },
            ]}
            value={String(draft.collab_mode)}
            onValueChange={(value) =>
              onChange({ collab_mode: Number(value) as ExperimentCollabMode })
            }
          />
        </FormField>

        {isGroup ? (
          <div className="flex flex-col gap-4 rounded-md border border-line bg-surface-sunken p-4">
            <FormField
              label="每组人数"
              htmlFor="wizard-group-size"
              required
              error={errors.groupSize}
              helper="小组共享同一套实验环境与进度"
            >
              <Input
                id="wizard-group-size"
                type="number"
                min="2"
                value={String(draft.group_config.size)}
                invalid={Boolean(errors.groupSize)}
                onChange={(event) =>
                  onChange({
                    group_config: { ...draft.group_config, size: Number(event.target.value) || 0 },
                  })
                }
              />
            </FormField>
            <FormField
              label="组内分工"
              htmlFor="wizard-group-roles"
              helper="用逗号分隔,例如:开发,测试,汇报。留空则不分工"
            >
              <Input
                id="wizard-group-roles"
                value={draft.group_config.roles.join(',')}
                onChange={(event) =>
                  onChange({
                    group_config: {
                      ...draft.group_config,
                      roles: event.target.value
                        .split(',')
                        .map((role) => role.trim())
                        .filter((role) => role !== ''),
                    },
                  })
                }
              />
            </FormField>
            <Callout tone="info">
              小组实验需要你在实验详情里给学生分组,分组后学生才能进入。
            </Callout>
          </div>
        ) : null}

        <Checkbox
          checked={draft.require_report}
          label="要求学生提交实验报告"
          onCheckedChange={(checked) => onChange({ require_report: checked === true })}
        />
      </CardBody>
    </Card>
  )
}
