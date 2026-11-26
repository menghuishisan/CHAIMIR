import { ApiClient, encodePathSegment } from '../client'
import type {
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStatus,
} from '../constants/experiment'
import type { PaginatedResponse } from '../types/common'
import type {
  Experiment,
  StudentExperiment,
  ExperimentRequest,
  CreateInstanceRequest,
  ExperimentInstance,
  ValidationResult,
  CheckpointJudgeRequest,
  ProgressDTO,
  CheckpointResult,
  ReportDTO,
  StudentReportDTO,
  ReportAccessDTO,
  ExperimentGroup,
  ExperimentGroupMemberRequest,
  ExperimentGroupRequest,
  GradeReportRequest,
} from '../types/experiment'

export class ExperimentApi {
  constructor(private client: ApiClient) {}

  async getExperiments(params?: {
    course_id?: string
    status?: ExperimentStatus
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Experiment>> {
    return this.client.get('/experiment/experiments', params)
  }

  /** 返回教师视图。服务端校验作者或本校管理员身份。 */
  async getExperiment(experimentId: string): Promise<Experiment> {
    return this.client.get(`/experiment/experiments/${encodePathSegment(experimentId)}`)
  }

  /** 教师只看自己实验的实例，校管可看全校。 */
  async listInstances(params?: {
    status?: ExperimentInstanceStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ExperimentInstance>> {
    return this.client.get('/experiment/instances', params)
  }

  async getPublishedExperiments(params?: {
    course_id?: string
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<StudentExperiment>> {
    return this.client.get('/experiment/student/experiments', params)
  }

  /** 学生视图省略初始化与判题内部配置。 */
  async getPublishedExperiment(experimentId: string): Promise<StudentExperiment> {
    return this.client.get(`/experiment/student/experiments/${encodePathSegment(experimentId)}`)
  }

  async createExperiment(data: ExperimentRequest): Promise<Experiment> {
    return this.client.post('/experiment/experiments', data)
  }

  async updateExperiment(experimentId: string, data: ExperimentRequest): Promise<Experiment> {
    return this.client.patch(`/experiment/experiments/${encodePathSegment(experimentId)}`, data)
  }

  async validateExperiment(experimentId: string): Promise<ValidationResult> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/validate`)
  }

  async publishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/publish`)
  }

  async unpublishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/unpublish`)
  }

  /** 返回的总数遵循状态筛选。 */
  async listReports(
    experimentId: string,
    params?: {
      status?: ExperimentReportStatus
      page?: number
      size?: number
    }
  ): Promise<PaginatedResponse<ReportDTO>> {
    return this.client.get(
      `/experiment/experiments/${encodePathSegment(experimentId)}/reports`,
      params
    )
  }

  /**
   * 教师换取实验报告的一次性下载授权。
   */
  async issueReportAccess(reportId: string): Promise<ReportAccessDTO> {
    return this.client.post(`/experiment/reports/${encodePathSegment(reportId)}/access`)
  }

  async gradeReport(reportId: string, data: GradeReportRequest): Promise<ReportDTO> {
    return this.client.post(`/experiment/reports/${encodePathSegment(reportId)}/grade`, data)
  }

  async createGroup(experimentId: string, data: ExperimentGroupRequest): Promise<ExperimentGroup> {
    return this.client.post(
      `/experiment/experiments/${encodePathSegment(experimentId)}/groups`,
      data
    )
  }

  /** 列表省略共享实例。单组详情可返回当前实例。 */
  async listGroups(experimentId: string): Promise<ExperimentGroup[]> {
    return this.client.get(`/experiment/experiments/${encodePathSegment(experimentId)}/groups`)
  }

  async upsertGroupMember(
    groupId: string,
    data: ExperimentGroupMemberRequest
  ): Promise<ExperimentGroup> {
    return this.client.post(`/experiment/groups/${encodePathSegment(groupId)}/members`, data)
  }

  /** 服务端先收回活跃资源权限，再删除成员。 */
  async removeGroupMember(groupId: string, studentId: string): Promise<ExperimentGroup> {
    return this.client.delete(
      `/experiment/groups/${encodePathSegment(groupId)}/members/${encodePathSegment(studentId)}`
    )
  }

  async createInstance(
    experimentId: string,
    data: CreateInstanceRequest
  ): Promise<ExperimentInstance> {
    return this.client.post(
      `/experiment/experiments/${encodePathSegment(experimentId)}/instances`,
      data
    )
  }

  async getInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.get(`/experiment/instances/${encodePathSegment(instanceId)}`)
  }

  async judgeCheckpoint(
    instanceId: string,
    checkpointId: string,
    data: CheckpointJudgeRequest
  ): Promise<CheckpointResult> {
    return this.client.post(
      `/experiment/instances/${encodePathSegment(instanceId)}/checkpoints/${encodePathSegment(checkpointId)}/judge`,
      data
    )
  }

  async submitReport(
    instanceId: string,
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<ReportDTO> {
    return this.client.upload(
      `/experiment/instances/${encodePathSegment(instanceId)}/report`,
      file,
      onProgress
    )
  }

  /** 读取当前学生本人报告的批改结果。 */
  async getStudentReport(instanceId: string): Promise<StudentReportDTO> {
    return this.client.get(`/experiment/instances/${encodePathSegment(instanceId)}/report`)
  }

  async getProgress(instanceId: string): Promise<ProgressDTO> {
    return this.client.get(`/experiment/instances/${encodePathSegment(instanceId)}/progress`)
  }

  async getGroup(groupId: string): Promise<ExperimentGroup> {
    return this.client.get(`/experiment/groups/${encodePathSegment(groupId)}`)
  }

  async pauseInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/pause`)
  }

  async resumeInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/resume`)
  }

  async activateStage(instanceId: string, stage: number): Promise<ExperimentInstance> {
    return this.client.post(
      `/experiment/instances/${encodePathSegment(instanceId)}/stages/${encodePathSegment(String(stage))}/activate`
    )
  }

  async finishInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/finish`)
  }
}
