// Experiment API：实验编排
// 对应后端 M7 模块

import { ApiClient } from '../client'
import type { ExperimentStatus } from '../constants/experiment'
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
  ExperimentGroup,
  ExperimentGroupMemberRequest,
  ExperimentGroupRequest,
  GradeReportRequest,
  ReportDownloadGrant,
} from '../types/experiment'

/**
 * ExperimentApi 封装后端 M7 实验编排、实例、报告和协作小组接口。
 */
export class ExperimentApi {
  /**
   * constructor 注入统一 API 客户端，保持实验接口的路径和错误处理一致。
   */
  constructor(private client: ApiClient) {}

  /**
   * 获取实验列表
   */
  async getExperiments(params?: {
    course_id?: string
    status?: ExperimentStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Experiment>> {
    return this.client.get('/experiment/experiments', params)
  }

  /** getPublishedExperiments 查询学生可直接进入的独立已发布实验。 */
  async getPublishedExperiments(params?: { page?: number; size?: number }): Promise<PaginatedResponse<StudentExperiment>> {
    return this.client.get('/experiment/student/experiments', params)
  }

  /** getPublishedExperiment 读取单条学生实验；课程入口须传 M6 签发的启动授权。 */
  async getPublishedExperiment(experimentId: string, launchGrant?: string): Promise<StudentExperiment> {
    return this.client.get(
      `/experiment/student/experiments/${experimentId}`,
      undefined,
      launchGrant ? { 'X-Experiment-Launch-Grant': launchGrant } : undefined,
    )
  }

  /**
   * 创建实验（教师）
   */
  async createExperiment(data: ExperimentRequest): Promise<Experiment> {
    return this.client.post('/experiment/experiments', data)
  }

  /**
   * 更新实验
   */
  async updateExperiment(experimentId: string, data: ExperimentRequest): Promise<Experiment> {
    return this.client.patch(`/experiment/experiments/${experimentId}`, data)
  }

  /**
   * 发布前校验
   */
  async validateExperiment(experimentId: string): Promise<ValidationResult> {
    return this.client.post(`/experiment/experiments/${experimentId}/validate`)
  }

  /**
   * 发布实验
   */
  async publishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${experimentId}/publish`)
  }

  /**
   * 下架实验
   */
  async unpublishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${experimentId}/unpublish`)
  }

  /**
   * 查询实验报告列表。
   */
  async listReports(experimentId: string, params?: { page?: number; size?: number }): Promise<PaginatedResponse<ReportDTO>> {
    return this.client.get(`/experiment/experiments/${experimentId}/reports`, params)
  }

  /**
   * 教师批改实验报告。
   */
  async gradeReport(reportId: string, data: GradeReportRequest): Promise<ReportDTO> {
    return this.client.post(`/experiment/reports/${reportId}/grade`, data)
  }

  /** issueReportDownloadGrant 在教师业务鉴权后签发一次性报告下载授权。 */
  async issueReportDownloadGrant(reportId: string): Promise<ReportDownloadGrant> {
    return this.client.post(`/experiment/reports/${reportId}/download-grant`)
  }

  /** downloadReport 签发并立即消费报告授权，避免页面接触对象存储引用。 */
  async downloadReport(reportId: string): Promise<{ blob: Blob; fileName: string }> {
    const grant = await this.issueReportDownloadGrant(reportId)
    return {
      blob: await this.client.getBlob('/storage/download', { token: grant.token }),
      fileName: grant.file_name,
    }
  }

  /**
   * 创建实验协作小组。
   */
  async createGroup(experimentId: string, data: ExperimentGroupRequest): Promise<ExperimentGroup> {
    return this.client.post(`/experiment/experiments/${experimentId}/groups`, data)
  }

  /**
   * 加入或调整协作小组成员角色。
   */
  async upsertGroupMember(groupId: string, data: ExperimentGroupMemberRequest): Promise<ExperimentGroup> {
    return this.client.post(`/experiment/groups/${groupId}/members`, data)
  }

  /**
   * 创建实验实例（学生发起）
   */
  async createInstance(experimentId: string, data: CreateInstanceRequest): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/experiments/${experimentId}/instances`, data)
  }

  /**
   * 获取实验实例详情
   */
  async getInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.get(`/experiment/instances/${instanceId}`)
  }

  /**
   * 触发检查点判分
   */
  async judgeCheckpoint(
    instanceId: string,
    checkpointId: string,
    data: CheckpointJudgeRequest
  ): Promise<CheckpointResult> {
    return this.client.post(`/experiment/instances/${instanceId}/checkpoints/${checkpointId}/judge`, data)
  }

  /**
   * 提交实验报告
   */
  async submitReport(instanceId: string, file: File, onProgress?: (progress: number) => void): Promise<ReportDTO> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.postFormData(`/experiment/instances/${instanceId}/report`, formData, onProgress)
  }

  /**
   * 获取实验进度订阅信息
   */
  async getProgress(instanceId: string): Promise<ProgressDTO> {
    return this.client.get(`/experiment/instances/${instanceId}/progress`)
  }

  /**
   * 读取协作小组详情。
   */
  async getGroup(groupId: string): Promise<ExperimentGroup> {
    return this.client.get(`/experiment/groups/${groupId}`)
  }

  /**
   * 暂停实验实例
   */
  async pauseInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${instanceId}/pause`)
  }

  /**
   * 恢复实验实例
   */
  async resumeInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${instanceId}/resume`)
  }

  /**
   * 激活已解锁阶段
   */
  async activateStage(instanceId: string, stage: number): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${instanceId}/stages/${stage}/activate`)
  }

  /**
   * 完成实验实例
   */
  async finishInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${instanceId}/finish`)
  }

  /**
   * 回收实验实例资源
   */
  async recycleInstance(instanceId: string): Promise<void> {
    return this.client.delete(`/experiment/instances/${instanceId}`)
  }
}
