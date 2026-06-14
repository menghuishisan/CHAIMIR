// @chaimir/api-client 主入口

export { ApiClient } from './client'
export type { ApiConfig, ApiError, ApiResponse } from './client'

// API 模块
export { IdentityApi } from './modules/identity'
export { ContentApi } from './modules/content'
export { TeachingApi } from './modules/teaching'
export { SandboxApi } from './modules/sandbox'
export { JudgeApi } from './modules/judge'
export { ExperimentApi } from './modules/experiment'
export { ContestApi } from './modules/contest'
export { AdminApi } from './modules/admin'
export { NotifyApi } from './modules/notify'
export { GradeApi } from './modules/grade'
export { SimApi } from './modules/sim'

// 类型导出
export type * from './types'

// API 工厂：创建完整的 API 实例
import { ApiClient, ApiConfig } from './client'
import { IdentityApi } from './modules/identity'
import { ContentApi } from './modules/content'
import { TeachingApi } from './modules/teaching'
import { SandboxApi } from './modules/sandbox'
import { JudgeApi } from './modules/judge'
import { ExperimentApi } from './modules/experiment'
import { ContestApi } from './modules/contest'
import { AdminApi } from './modules/admin'
import { NotifyApi } from './modules/notify'
import { GradeApi } from './modules/grade'
import { SimApi } from './modules/sim'

export interface ChaimirApi {
  identity: IdentityApi
  content: ContentApi
  teaching: TeachingApi
  sandbox: SandboxApi
  judge: JudgeApi
  experiment: ExperimentApi
  contest: ContestApi
  admin: AdminApi
  notify: NotifyApi
  grade: GradeApi
  sim: SimApi
}

export function createApi(config: ApiConfig): ChaimirApi {
  const client = new ApiClient(config)

  return {
    identity: new IdentityApi(client),
    content: new ContentApi(client),
    teaching: new TeachingApi(client),
    sandbox: new SandboxApi(client),
    judge: new JudgeApi(client),
    experiment: new ExperimentApi(client),
    contest: new ContestApi(client),
    admin: new AdminApi(client),
    notify: new NotifyApi(client),
    grade: new GradeApi(client),
    sim: new SimApi(client),
  }
}
