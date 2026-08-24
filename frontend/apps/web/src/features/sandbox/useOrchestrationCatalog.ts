// features/sandbox/useOrchestrationCatalog 提供编排环境时的运行时、镜像版本与组件选项。
//
// M2 的运行时/工具管理接口是平台面(会下发容器编排清单、镜像 digest 地址与命令白名单),
// 编排面只该看到「能选什么」,故统一走 M2 编排目录 `GET /sandbox/catalog`
// (docs/02-沙箱引擎/04-接口设计.md §2.1)。
//
// 目录一次返回运行时及其内联镜像版本,故选运行时后无需再请求镜像清单 ——
// 这同时消掉了原先「每换一次运行时打一次接口」的 N+1。
// 三个编排页(实验环境、竞赛对抗题、漏洞题预验证)共用本 Hook,不各写一遍取数与筛选。
//
// 兼容性由服务端编译器判定:目录不下发「某运行时允许哪些工具」,前端也不自己算 ——
// 那等于在浏览器里重写一遍编译器,与冲突/连接/资源校验必然对不上。
// 教师提交声明后由发布前校验回报诊断(对齐清单 §6.3 / §7.5)。

import { useCallback, useMemo } from 'react'
import type { SandboxCatalogTool, SandboxOrchestrationCatalog } from '@chaimir/api-client'
import { api } from '../../app/api'
import { useAsyncResource, type AsyncResourceState } from '../../hooks'

/** SelectOption 与 @chaimir/ui 的 Select 选项结构一致。 */
interface SelectOption {
  value: string
  label: string
}

export interface OrchestrationCatalogState {
  /** resource 供 ResourceState 渲染加载、空、错误三态。 */
  resource: AsyncResourceState<SandboxOrchestrationCatalog>
  /** runtimeOptions 是运行时下拉选项,标签带编码便于教师核对。 */
  runtimeOptions: SelectOption[]
  /** tools 是可勾选的学生工具,基础设施不在其中。 */
  tools: SandboxCatalogTool[]
  /** infra 是可声明的基础设施组件,与学生工具分开呈现(§7.2)。 */
  infra: SandboxCatalogTool[]
  /** imageOptions 返回指定运行时下已预拉取成功的镜像版本选项。 */
  imageOptions: (runtimeCode: string) => SelectOption[]
}

/** EMPTY_CATALOG 是关闭取数时的空目录,避免页面为此写第二套分支。 */
const EMPTY_CATALOG: SandboxOrchestrationCatalog = { runtimes: [], infra: [], tools: [] }

/**
 * useOrchestrationCatalog 读取编排目录并派生下拉选项。
 * 目录只回可调度项(运行时自检通过、镜像预拉取成功且内置创世),故前端不再按状态过滤 ——
 * 状态门禁在服务端 SQL 层做,避免「停用了但前端仍列出来」和「两边口径不一致」两类问题。
 *
 * enabled 供只在特定分支才需要环境的页面使用(如解题赛题目不需要运行时),
 * 关闭时不发请求并给空目录。
 */
export function useOrchestrationCatalog(enabled = true): OrchestrationCatalogState {
  const resource = useAsyncResource(
    () =>
      enabled
        ? api.sandbox.getOrchestrationCatalog()
        : Promise.resolve<SandboxOrchestrationCatalog>(EMPTY_CATALOG),
    [enabled],
    (value) => value.runtimes.length === 0
  )

  const runtimeOptions = useMemo(
    () =>
      (resource.data?.runtimes ?? []).map((runtime) => ({
        value: runtime.code,
        label: `${runtime.name} · ${runtime.code}`,
      })),
    [resource.data]
  )

  const tools = useMemo(() => resource.data?.tools ?? [], [resource.data])

  const infra = useMemo(() => resource.data?.infra ?? [], [resource.data])

  const imageOptions = useCallback(
    (runtimeCode: string): SelectOption[] => {
      const runtime = (resource.data?.runtimes ?? []).find((item) => item.code === runtimeCode)
      return (runtime?.images ?? []).map((image) => ({
        value: image.version,
        label: image.version,
      }))
    },
    [resource.data]
  )

  return { resource, runtimeOptions, tools, infra, imageOptions }
}
