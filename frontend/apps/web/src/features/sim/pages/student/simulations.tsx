// 仿真实验室页(学生侧栏,/student/simulations)。设计 §7.2:
// 页头 + 搜索;分类筛选用带计数的分类标签(计数来自接口分组计数);卡片网格,卡片显示场景名、分类、
// 当前版本、运行位置;整卡用当前版本直接进入推演,卡内「其他版本」打开版本弹窗。分类与搜索写进地址。
//
// 只做场景检索与版本选择:仿真会话由 M7 实验实例编排产生(POST /sim/sessions 在 internal 组),
// 学生不能独立建会话。因此本页能直接进入的只有「本机确定性推演」——
// 平台内置场景在浏览器 Worker 内按同一 seed 复现,不上报动作、不建实时连接(对齐清单 §6.6)。
// 本校自建场景(compute=isolated)在服务端隔离容器里运行,只能从课程实验进入,
// 卡片上直接说明且不是链接 —— 不让人点进去才发现进不了。
//
// 分类计数另取一次(只带搜索词、不带分类,每页 1 条):接口的分组计数与当前筛选同口径,
// 选中某一类后其余分类的数字会消失,故标签上的数字取自不带分类的那一次。

import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { History, Layers, Network, Server } from 'lucide-react'
import { SIM_COMPUTE, SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  FormField,
  Icon,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { facetGroup, facetSum } from '../../../../utils/facets'
import { formatDate } from '../../../../utils/formatters'
import { simCategoryLabel } from '../../../../utils/labels/sim'

/** 地址参数:category 是分类(空为全部),q 是搜索词 */
const SIM_QUERY = {
  category: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

/** 分类标签里「全部」的取值(分段控件不接受空串) */
const ALL_CATEGORIES = 'all'

/** workspaceHref 推演工作台地址:版本随地址带入,刷新后回到同一版本 */
function workspaceHref(code: string, version: string): string {
  return `/student/simulations/${encodeURIComponent(code)}/workspace?version=${encodeURIComponent(version)}`
}

/**
 * StudentSimulationsPage 检索可用仿真场景并进入推演。
 */
export default function StudentSimulationsPage() {
  const [query, setQuery] = useUrlState(SIM_QUERY)
  const [versionPickerCode, setVersionPickerCode] = useState<string>()

  // 学生只看已发布场景:草稿与审核中的包属教师提交流程,下架包不再可用
  const packages = usePagedResource<SimPackageMeta>(
    (params) =>
      api.sim.getPackages({
        status: SIM_PACKAGE_STATUS.PUBLISHED,
        category: query.category || undefined,
        keyword: query.q || undefined,
        ...params,
      }),
    [query.category, query.q],
    { urlPageKey: 'page' },
  )
  // 分类分段的数字取列表接口的 facets:套用搜索,不套用分类筛选,选中一个分类后其他分类的数字仍然可见
  const categoryFacets = packages.status === 'success' || packages.status === 'empty' ? packages.data?.facets : undefined
  const categoryOptions = useMemo(() => {
    const group = facetGroup(categoryFacets, 'category') ?? {}
    const known = Object.keys(group).sort((a, b) => simCategoryLabel(a).localeCompare(simCategoryLabel(b), 'zh-CN'))
    // 地址里的分类没有场景时也保留它的标签,否则分段控件无选中项
    if (query.category && !known.includes(query.category)) known.push(query.category)
    return [
      { value: ALL_CATEGORIES, label: '全部', count: facetSum(categoryFacets, 'category') },
      ...known.map((category) => ({ value: category, label: simCategoryLabel(category), count: group[category] ?? 0 })),
    ]
  }, [categoryFacets, query.category])

  const filtered = query.category !== '' || query.q !== ''
  const list = packages.data?.list ?? []

  return (
    <PageScaffold>
      <PageHeader title="仿真实验室" description="选一个场景开始推演。平台场景在你的浏览器里按固定的随机种子运行,每次结果都能复现。" />

      <div className="mb-5 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="-mx-1 overflow-x-auto px-1 pb-1">
          <SegmentedControl
            aria-label="按分类筛选"
            options={categoryOptions}
            value={query.category || ALL_CATEGORIES}
            onValueChange={(value) => setQuery({ category: value === ALL_CATEGORIES ? '' : value })}
          />
        </div>
        <SearchBox
          label="搜索仿真场景"
          placeholder="搜索场景名称"
          value={query.q}
          onSearch={(value) => setQuery({ q: value })}
        />
      </div>

      {packages.status === 'error' ? (
        <ResourceError
          message={packages.error?.message ?? '仿真场景暂时读取不到'}
          traceId={packages.error?.traceId}
          onRetry={packages.reload}
        />
      ) : packages.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Network} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有符合条件的场景' : '还没有可用的仿真场景'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个分类或关键词试试。' : '平台上架和老师提交并通过审核的场景会显示在这里。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={() => setQuery({ category: '', q: '' })}>
              看全部场景
            </Button>
          ) : null}
        </div>
      ) : packages.status === 'loading' && list.length === 0 ? (
        <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取仿真场景">
          {Array.from({ length: 6 }, (_, index) => (
            <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
              <Skeleton variant="title" width="half" />
              <Skeleton variant="line" lines={2} />
            </li>
          ))}
        </ul>
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={packages.status === 'loading' || undefined}>
            {list.map((pkg) => (
              <li key={pkg.id}>
                <SimPackageCard pkg={pkg} onOtherVersions={() => setVersionPickerCode(pkg.code)} />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={packages.page} pageSize={packages.pageSize} total={packages.total} onPageChange={packages.setPage} />
          </div>
        </>
      )}

      {versionPickerCode ? (
        <VersionPickerModal code={versionPickerCode} onClose={() => setVersionPickerCode(undefined)} />
      ) : null}
    </PageScaffold>
  )
}

/**
 * SimPackageCard 一个场景:场景名、分类、当前版本、运行位置。
 * 平台场景整卡链向当前版本的工作台,卡内「其他版本」单独可点;学校自建场景不可点,直接写明从哪里进入。
 */
function SimPackageCard({ pkg, onOtherVersions }: { pkg: SimPackageMeta; onOtherVersions: () => void }) {
  const runnable = pkg.compute === SIM_COMPUTE.BROWSER
  return (
    <Card interactive={runnable} className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-3">
        {runnable ? (
          <Link to={workspaceHref(pkg.code, pkg.version)} className="card-link line-clamp-2 text-lg font-bold text-ink">
            {pkg.name}
          </Link>
        ) : (
          <span className="line-clamp-2 text-lg font-bold text-ink">{pkg.name}</span>
        )}
        <Badge tone="neutral" className="shrink-0">
          {simCategoryLabel(pkg.category)}
        </Badge>
      </div>
      <p className="text-sm text-ink-sub">
        版本 <span className="font-mono">{pkg.version}</span> · {formatDate(pkg.updated_at)} 更新
      </p>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-3">
        {runnable ? (
          <>
            <span className="text-sm text-ink-sub">在浏览器里运行</span>
            <Button variant="ghost" size="sm" leftIcon={History} className="relative z-base" onClick={onOtherVersions}>
              其他版本
            </Button>
          </>
        ) : (
          <span className="flex items-center gap-1.5 text-sm text-ink-sub">
            <Icon icon={Server} size="sm" className="shrink-0" />
            在学校服务器上运行,请从课程的实验任务里打开
          </span>
        )}
      </div>
    </Card>
  )
}

interface VersionPickerModalProps {
  code: string
  onClose: () => void
}

/**
 * VersionPickerModal 列出场景的全部可用版本供选择。
 * 老版本仍可进入:课程可能指定了特定版本作为教学材料。
 */
function VersionPickerModal({ code, onClose }: VersionPickerModalProps) {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<string>()
  const versions = useAsyncResource(() => api.sim.getPackageVersions(code), [code])

  const options = useMemo(
    () =>
      (versions.data ?? [])
        .filter((item) => item.status === SIM_PACKAGE_STATUS.PUBLISHED && item.compute === SIM_COMPUTE.BROWSER)
        .map((item) => ({ value: item.version, label: `${item.version} · ${formatDate(item.updated_at)}` })),
    [versions.data],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>选择场景版本</ModalTitle>
          <ModalDescription>不同版本的推演内容可能不同。如果课程指定了版本,请按老师的要求选择。</ModalDescription>
        </ModalHeader>
        <ModalBody>
          <ResourceState
            resource={versions}
            emptyIcon={Layers}
            emptyTitle="暂无可用版本"
            emptyDescription="这个场景当前没有已发布的版本。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() =>
              options.length === 0 ? (
                <Callout tone="info">这个场景当前没有已发布的版本。</Callout>
              ) : (
                <FormField label="场景版本" required>
                  <Select options={options} value={selected ?? options[0].value} placeholder="选择版本" onValueChange={setSelected} />
                </FormField>
              )
            }
          </ResourceState>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            disabled={options.length === 0}
            onClick={() => navigate(workspaceHref(code, selected ?? options[0].value))}
          >
            进入推演
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
