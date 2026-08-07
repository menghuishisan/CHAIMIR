// 实验实训页(学生侧栏,/student/experiments)。
// 列表走学生专用投影 GET /experiment/student/experiments —— teacher 组的实验读取
// 会返回 init_code_ref/judger_code/extra_input,那是答案链路,学生不可用。
// 「进入实验」调 POST /experiments/{id}/instances:该接口对已有活跃实例幂等(见对齐清单 §6.8),
// 因此不做「开始 / 继续」二态判断,一个按钮既创建也恢复。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { FlaskConical, Play, Target, Users } from 'lucide-react'
import { ExperimentCollabMode, type StudentExperiment } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import {
  experimentCollabModeLabel,
  experimentStatusLabel,
  experimentStatusTone,
} from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/**
 * StudentExperimentsPage 列出可做实验并提供进入入口。
 */
export default function StudentExperimentsPage() {
  const navigate = useNavigate()
  const [enteringId, setEnteringId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const experiments = usePagedResource<StudentExperiment>(
    (params) => api.experiment.getPublishedExperiments(params),
    [],
  )

  /**
   * enterExperiment 创建或恢复实例后进入沉浸式工作台。
   * 小组实验必须带 my_group_id:后端按组成员关系校验,未分组时不发请求(该字段是学生
   * 获取自身 group_id 的唯一来源,见 M7 接口设计 §3.1)。
   */
  const enterExperiment = useCallback(
    async (experiment: StudentExperiment) => {
      if (experiment.collab_mode === ExperimentCollabMode.GROUP && !experiment.my_group_id) {
        setActionError('这个实验需要分组完成,请等老师把你分到小组后再进入。')
        return
      }
      setEnteringId(experiment.id)
      setActionError(undefined)
      try {
        const instance = await api.experiment.createInstance(experiment.id, {
          group_id: experiment.my_group_id,
        })
        navigate(`/student/experiments/${experiment.id}/workspace?instance=${instance.instance_id}`)
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '实验环境没能准备好,请稍后重试。'))
      } finally {
        setEnteringId(undefined)
      }
    },
    [navigate],
  )

  const list = experiments.data ? experiments.data.list : []
  const groupCount = list.filter((item) => item.collab_mode === ExperimentCollabMode.GROUP).length
  const reportCount = list.filter((item) => item.require_report).length

  const columns: TableColumn<StudentExperiment>[] = [
    {
      key: 'name',
      header: '实验',
      render: (experiment) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{experiment.name}</div>
          <div className="line-clamp-1 text-xs text-ink-sub">{experiment.description}</div>
        </div>
      ),
    },
    {
      key: 'collab_mode',
      header: '完成方式',
      render: (experiment) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="neutral">{experimentCollabModeLabel(experiment.collab_mode)}</Badge>
          {experiment.collab_mode === ExperimentCollabMode.GROUP && !experiment.my_group_id ? (
            <Badge tone="warning">等待分组</Badge>
          ) : null}
        </div>
      ),
    },
    {
      key: 'checkpoints',
      header: '检查点',
      align: 'right',
      mono: true,
      render: (experiment) => experiment.components.checkpoints.length,
    },
    {
      key: 'require_report',
      header: '实验报告',
      render: (experiment) => (
        <span className="text-ink-sub">{experiment.require_report ? '需要提交' : '不需要'}</span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (experiment) => (
        <StatusIndicator
          tone={experimentStatusTone(experiment.status)}
          label={experimentStatusLabel(experiment.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (experiment) => (
        <div className="flex justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate(`/student/experiments/${experiment.id}`)}
          >
            查看详情
          </Button>
          <Button
            variant="primary"
            size="sm"
            leftIcon={Play}
            loading={enteringId === experiment.id}
            onClick={() => void enterExperiment(experiment)}
          >
            进入实验
          </Button>
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学习区' }, { label: '实验实训' }]} />}
        title="实验实训"
        description="进入实验会为你准备独立的实验环境,中途退出后再进入可以接着做。"
        icon={FlaskConical}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="可做实验" value={experiments.total} icon={FlaskConical} />
          <Stat label="本页小组实验" value={groupCount} icon={Users} hint="需要分组后进入" />
          <Stat label="本页需交报告" value={reportCount} icon={Target} />
        </div>
      </PageSection>

      <PageSection title="实验列表" description={`共 ${experiments.total} 个实验`}>
        <div className="flex flex-col gap-4">
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={experiments}
            emptyIcon={FlaskConical}
            emptyTitle="暂无可做的实验"
            emptyDescription="老师发布实验后会显示在这里,你可以直接进入实验环境动手做。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={experiments.page}
                  pageSize={experiments.pageSize}
                  total={experiments.total}
                  onPageChange={experiments.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>
    </PageScaffold>
  )
}
