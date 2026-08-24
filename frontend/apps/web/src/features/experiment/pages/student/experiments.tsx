// 实验实训页(学生侧栏,/student/experiments)。
// 列表走学生专用投影 GET /experiment/student/experiments —— teacher 组的实验读取
// 会返回 init_code_ref/judger_code/extra_input,那是答案链路,学生不可用。
// 「进入实验」调 POST /experiments/{id}/instances:该接口对已有活跃实例幂等(见对齐清单 §6.8),
// 因此不做「开始 / 继续」二态判断,一个按钮既创建也恢复。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { FlaskConical, Play } from 'lucide-react'
import { ExperimentCollabMode, type StudentExperiment } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource } from '../../../../hooks'
import { facetCount } from '../../../../utils/facets'
import {
  experimentCollabModeLabel,
} from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ExperimentIdentityCell, ExperimentStatusCell } from '../../components/ExperimentTableCells'

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

  // 协作形态与报告要求取后端 facets:全量分组计数,不用当前页切片去数(§6.5.4)
  const groupCount = facetCount(
    experiments.data?.facets,
    'collab_mode',
    ExperimentCollabMode.GROUP,
  )
  const reportCount = facetCount(experiments.data?.facets, 'require_report', true)

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

  const columns: TableColumn<StudentExperiment>[] = [
    {
      key: 'name',
      header: '实验',
      render: (experiment) => <ExperimentIdentityCell experiment={experiment} />,
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
      render: (experiment) => <ExperimentStatusCell experiment={experiment} />,
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
        kicker={<Breadcrumb items={[{ label: '学习区' }]} />}
        title="实验实训"
        description="进入实验会为你准备独立的实验环境,中途退出后再进入可以接着做。"
        icon={FlaskConical}
      />

      {/*
        归族:资源列表族(§6.5.3 第 ①)。指标退为一行内联摘要:
        「小组实验/需交报告」走后端聚合契约(facets.collab_mode / facets.require_report),
        是全量口径而非当前页切片(§6.5.4)。
      */}
      <MetricStrip
        label="实验总量摘要"
        className="mb-5"
        items={[
          { label: '可做实验', value: experiments.total, hint: '已发布给你的' },
          { label: '小组实验', value: groupCount, hint: '要先加入小组' },
          { label: '需交报告', value: reportCount, hint: '做完还要写报告' },
        ]}
      />

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 数据表与分页同处一块抬起片(§6.5.2)。学生投影接口没有筛选参数,故不排筛选井 */}
      <DataPanel
        label="实验列表"
        footer={
          <Pagination
            page={experiments.page}
            pageSize={experiments.pageSize}
            total={experiments.total}
            onPageChange={experiments.setPage}
          />
        }
      >
        <ResourceState
          resource={experiments}
          emptyIcon={FlaskConical}
          emptyTitle="暂无可做的实验"
          emptyDescription="老师发布实验后会显示在这里,你可以直接进入实验环境动手做。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(item) => item.id}
              elevated={false}
              // <md 换行卡(§6.4.1 规则 3):实验名一行、完成方式与检查点一行,进入按钮在右
              mobileCard={(item) => ({
                title: item.name,
                meta: `${experimentCollabModeLabel(item.collab_mode)} · ${item.components.checkpoints.length} 个检查点 · ${item.require_report ? '需交报告' : '免交报告'}`,
                badge: <ExperimentStatusCell experiment={item} />,
                action: (
                  <Button
                    variant="primary"
                    size="sm"
                    leftIcon={Play}
                    loading={enteringId === item.id}
                    onClick={() => void enterExperiment(item)}
                  >
                    进入
                  </Button>
                ),
              })}
            />
          )}
        </ResourceState>
      </DataPanel>
    </PageScaffold>
  )
}
