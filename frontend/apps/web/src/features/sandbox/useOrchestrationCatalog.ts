import { api } from '../../app/api'
import { useAsyncResource } from '../../hooks'

/** 目录由服务端筛选；完整组合与预拉取证明仍由 M2 校验。 */
export function useOrchestrationCatalog() {
  const resource = useAsyncResource(
    () => api.sandbox.getOrchestrationCatalog(),
    [],
    (value) => value.runtimes.length === 0
  )
  const runtimes = resource.data?.runtimes ?? []

  return {
    resource,
    runtimeOptions: runtimes.map((runtime) => ({
      value: runtime.code,
      label: `${runtime.name} · ${runtime.code}`,
    })),
    tools: resource.data?.tools ?? [],
    infra: resource.data?.infra ?? [],
    imageOptions: (runtimeCode: string) =>
      (runtimes.find((runtime) => runtime.code === runtimeCode)?.images ?? []).map((image) => ({
        value: image.version,
        label: image.version,
      })),
  }
}
