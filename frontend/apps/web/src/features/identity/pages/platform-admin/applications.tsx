// 入驻申请(平台侧栏,/platform-admin/applications)。设计 §7.6:
// 分段「待审核 / 已开通 / 已驳回 / 全部」(默认待审核,带数字)+ 搜索,都写进地址;
// 列表行显示学校名、机构类型、联系人、提交时间(相对时间)、状态,整行链向申请详情。指标条与常驻说明卡去掉。
// 申请接口一次返回全部(不分页,数量按设计只有几十到几百),故分段计数与搜索在本页完成。

import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { Inbox } from 'lucide-react'
import { ApplicationStatus, type TenantApplication } from '@chaimir/api-client'
import {
  ListRow,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { applicationStatusLabel, schoolTypeLabel } from '../../../../utils/labels/identity'
import { applicationStatusTone } from '../../statusPresentation'

/** 地址参数:status 分段(默认待审核),q 学校名或联系人 */
const APPLICATIONS_QUERY = {
  status: { defaultValue: 'pending', allowed: ['pending', 'approved', 'rejected', 'all'] },
  q: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'pending', label: '待审核', status: ApplicationStatus.PENDING },
  { value: 'approved', label: '已开通', status: ApplicationStatus.APPROVED },
  { value: 'rejected', label: '已驳回', status: ApplicationStatus.REJECTED },
  { value: 'all', label: '全部' },
] as const

/** PlatformApplicationsPage 入驻申请分诊:谁在等、谁已处理,整行进入详情审核 */
export default function PlatformApplicationsPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(APPLICATIONS_QUERY)
  const applications = useAsyncResource(
    () => api.identity.getApplications(),
    [],
    (list) => list.length === 0
  )
  const all = useMemo(() => applications.data ?? [], [applications.data])

  const counts = useMemo(() => {
    const result: Record<string, number> = { all: all.length }
    for (const segment of STATUS_SEGMENTS) {
      if ('status' in segment)
        result[segment.value] = all.filter((item) => item.status === segment.status).length
    }
    return result
  }, [all])

  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)
  const keyword = query.q.trim().toLowerCase()
  const visible = useMemo(
    () =>
      all.filter(
        (item) =>
          (!segment || !('status' in segment) || item.status === segment.status) &&
          (keyword === '' ||
            item.school_name.toLowerCase().includes(keyword) ||
            item.contact_name.toLowerCase().includes(keyword))
      ),
    [all, keyword, segment]
  )
  const loaded = applications.status === 'success' || applications.status === 'empty'

  return (
    <PageScaffold>
      <PageHeader
        title="入驻申请"
        meta={loaded && counts.pending > 0 ? [{ value: `${counts.pending} 份待审核` }] : undefined}
      />

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按审核状态筛选"
          options={STATUS_SEGMENTS.map((item) => ({
            value: item.value,
            label: item.label,
            count: loaded ? counts[item.value] : undefined,
          }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full self-start overflow-x-auto"
        />
        <SearchBox
          label="搜索申请"
          placeholder="搜索学校名或联系人"
          value={query.q}
          onSearch={(value) => setQuery({ q: value })}
        />
      </div>

      <ResourceState
        resource={applications}
        emptyIcon={Inbox}
        emptyTitle="还没有入驻申请"
        emptyDescription="学校在入驻页提交申请后会出现在这里。"
        skeleton={<Skeleton variant="line" lines={6} />}
      >
        {() =>
          visible.length === 0 ? (
            <p className="rounded-lg bg-surface px-4 py-10 text-center text-sm text-ink-sub shadow-xs">
              {keyword
                ? '没有匹配的申请,换个学校名或联系人试试。'
                : query.status === 'pending'
                  ? '没有待审核的申请。'
                  : '这个状态下没有申请。'}
            </p>
          ) : (
            <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
              {visible.map((item) => (
                <li key={item.application_id}>
                  <ApplicationRow application={item} onNavigate={navigate} />
                </li>
              ))}
            </ul>
          )
        }
      </ResourceState>
    </PageScaffold>
  )
}

/** ApplicationRow 一份申请:学校名;机构类型 · 联系人 · 提交时间;状态在右;整行链向详情 */
function ApplicationRow({
  application,
  onNavigate,
}: {
  application: TenantApplication
  onNavigate: (href: string) => void
}) {
  return (
    <ListRow
      title={application.school_name}
      meta={
        <span>
          {schoolTypeLabel(application.school_type)} · {application.contact_name} ·{' '}
          <span title={formatDateTime(application.submitted_at)}>
            {formatRelativeOccurrence(application.submitted_at)}提交
          </span>
        </span>
      }
      trailing={
        <StatusIndicator
          tone={applicationStatusTone(application.status)}
          label={applicationStatusLabel(application.status)}
        />
      }
      href={`/platform-admin/applications/${application.application_id}`}
      onNavigate={onNavigate}
    />
  )
}
