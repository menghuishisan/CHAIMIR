// 沙箱操作规则统一实验、竞赛与共享工作台对 M2 实时阶段和生命周期的判断。
import { SandboxPhase, SandboxStatus } from '@chaimir/api-client'

/** isSandboxReadyForOperation 只开放已完成初始化且仍有计算资源的工作区。 */
export function isSandboxReadyForOperation(
  sandbox?: { phase: SandboxPhase; status: SandboxStatus }
): boolean {
  if (!sandbox) return false
  const phaseReady =
    sandbox.phase === SandboxPhase.READY || sandbox.phase === SandboxPhase.FULLY_READY
  const statusReady =
    sandbox.status === SandboxStatus.READY ||
    sandbox.status === SandboxStatus.RUNNING ||
    sandbox.status === SandboxStatus.IDLE
  return phaseReady && statusReady
}
