// 作业结果页(深页,/student/courses/:courseId/assignments/:assignmentId/submissions)。
//
// 提交编号从 GET /teaching/assignments/{id}/submissions 取回(该路由师生同用按身份分视角,
// 学生只回本人提交),因此刷新后仍能回到自己的结果 —— 不依赖上一次提交响应里的编号(FE-7)。
//
// 判题结果只经 M6 提交记录的分数与状态字段呈现:M3 的判题任务详情与进度 WS 属教师能力
// (judge 用户组守卫为教师/校管),学生侧不连(对齐清单 §6.6)。

import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ClipboardCheck, ClipboardList, MessageSquare } from 'lucide-react'
import { SubmissionStatus, type Submission } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  PageBody,
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
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import { submissionStatusLabel, submissionStatusTone } from '../../../../utils/labels/teaching'

/**
 * StudentSubmissionsPage 列出本人在该作业下的历次提交与得分。
 */
export default function StudentSubmissionsPage() {
  const { courseId = '', assignmentId = '' } = useParams<{ courseId: string; assignmentId: string }>()
  const navigate = useNavigate()

  const submissions = usePagedResource<Submission>(
    (params) => api.teaching.getSubmissions(assignmentId, params),
    [assignmentId],
  )

  const list = submissions.data ? submissions.data.list : []
  // 最新一次提交按后端排序(submitted_at DESC)取首条
  const latest = list.length > 0 ? list[0] : undefined

  const columns: TableColumn<Submission>[] = [
    { key: 'attempt_no', header: '提交次数', align: 'right', mono: true },
    {
      key: 'submitted_at',
      header: '提交时间',
      render: (submission) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(submission.submitted_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '批改状态',
      render: (submission) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
            loading={submission.status === SubmissionStatus.PENDING}
          />
          {submission.is_late ? <Badge tone="warning">迟交</Badge> : null}
        </div>
      ),
    },
    {
      key: 'auto_score',
      header: '自动判分',
      align: 'right',
      mono: true,
      render: (submission) => formatScore(submission.auto_score),
    },
    {
      key: 'manual_score',
      header: '教师评分',
      align: 'right',
      mono: true,
      render: (submission) => formatScore(submission.manual_score),
    },
    {
      key: 'final_score',
      header: '最终得分',
      align: 'right',
      mono: true,
      render: (submission) => (
        <span className="font-medium text-ink">{formatScore(submission.final_score)}</span>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '我的课程', href: '/student/courses' },
              { label: '课程详情', href: `/student/courses/${courseId}` },
              { label: '作业结果' },
            ]}
          />
        }
        title="作业结果"
        description="这里是你在这次作业下的历次提交、得分与老师反馈。"
        icon={ClipboardCheck}
        actions={
          <Button
            variant="outline"
            leftIcon={ClipboardList}
            onClick={() => navigate(`/student/courses/${courseId}/assignments/${assignmentId}`)}
          >
            回到作答
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="提交次数" value={submissions.total} icon={ClipboardCheck} />
          <Stat
            label="最新得分"
            value={latest ? formatScore(latest.final_score) : '—'}
            icon={ClipboardCheck}
            hint={latest ? submissionStatusLabel(latest.status) : '尚未提交'}
          />
          <Stat
            label="最近提交"
            value={latest ? formatDateTime(latest.submitted_at) : '—'}
            icon={ClipboardCheck}
          />
        </div>
      </PageSection>

      <PageBody rail={latest ? <FeedbackCard submission={latest} /> : undefined}>
        <PageSection title="提交记录" description={`共 ${submissions.total} 次提交`}>
          <ResourceState
            resource={submissions}
            emptyIcon={ClipboardCheck}
            emptyTitle="还没有提交记录"
            emptyDescription="完成作答并提交后,得分与老师反馈会显示在这里。"
            emptyAction={
              <Button
                variant="primary"
                onClick={() => navigate(`/student/courses/${courseId}/assignments/${assignmentId}`)}
              >
                去作答
              </Button>
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={submissions.page}
                  pageSize={submissions.pageSize}
                  total={submissions.total}
                  onPageChange={submissions.setPage}
                />
              </div>
            )}
          </ResourceState>
        </PageSection>
      </PageBody>
    </PageScaffold>
  )
}

/**
 * FeedbackCard 展示最新一次提交的批改反馈。
 * 批改中的提交明确说明"结果还没出来",不留白让用户以为漏了内容。
 */
function FeedbackCard({ submission }: { submission: Submission }) {
  const items = useMemo(
    () => [
      { term: '提交次数', description: `第 ${submission.attempt_no} 次`, mono: true },
      { term: '提交时间', description: formatDateTime(submission.submitted_at), mono: true },
      { term: '自动判分', description: formatScore(submission.auto_score), mono: true },
      { term: '教师评分', description: formatScore(submission.manual_score), mono: true },
      { term: '最终得分', description: formatScore(submission.final_score), mono: true },
    ],
    [submission],
  )

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader
          title="最新一次提交"
          description="最终得分含迟交扣分,以老师批改结果为准。"
          actions={
            <StatusIndicator
              tone={submissionStatusTone(submission.status)}
              label={submissionStatusLabel(submission.status)}
              loading={submission.status === SubmissionStatus.PENDING}
            />
          }
        />
        <CardBody className="flex flex-col gap-3">
          <DescriptionList dense items={items} />
          {submission.is_late ? (
            <Callout tone="warning">这次提交在截止时间之后,按课程迟交规则计分。</Callout>
          ) : null}
          {submission.status === SubmissionStatus.PENDING ? (
            <Callout tone="info">老师正在批改,出分后会在这里显示。</Callout>
          ) : null}
        </CardBody>
      </Card>

      {submission.comment ? (
        <Card>
          <CardHeader
            title={
              <span className="flex items-center gap-2">
                <MessageSquare aria-hidden="true" className="size-4 shrink-0 text-primary" />
                老师评语
              </span>
            }
          />
          <CardBody>
            <p className="whitespace-pre-wrap text-base leading-relaxed text-ink">
              {submission.comment}
            </p>
          </CardBody>
        </Card>
      ) : null}
    </div>
  )
}
