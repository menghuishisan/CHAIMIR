// Experiment API：实验编排
// 对应后端 M7 模块

import { ApiClient, encodePathSegment } from '../client'
import type { ExperimentReportStatus, ExperimentStatus } from '../constants/experiment'
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
  ReportAccessDTO,
  ExperimentGroup,
  ExperimentGroupMemberRequest,
  ExperimentGroupRequest,
  GradeReportRequest,
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

  /** getPublishedExperiments 查询学生可进入的已发布实验。 */
  async getPublishedExperiments(params?: { course_id?: string; page?: number; size?: number }): Promise<PaginatedResponse<StudentExperiment>> {
    return this.client.get('/experiment/student/experiments', params)
  }

  /**
   * 读取单个已发布实验的学生视图（与列表同一投影，不含环境初始化与判题内部配置）。
   * 实验详情页深链、刷新以及退出沉浸式工作台后的回落都用这条。
   */
  async getPublishedExperiment(experimentId: string): Promise<StudentExperiment> {
    return this.client.get(`/experiment/student/experiments/${encodePathSegment(experimentId)}`)
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
    return this.client.patch(`/experiment/experiments/${encodePathSegment(experimentId)}`, data)
  }

  /**
   * 发布前校验
   */
  async validateExperiment(experimentId: string): Promise<ValidationResult> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/validate`)
  }

  /**
   * 发布实验
   */
  async publishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/publish`)
  }

  /**
   * 下架实验
   */
  async unpublishExperiment(experimentId: string): Promise<Experiment> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/unpublish`)
  }

  /**
   * 查询实验报告列表。状态筛选由服务端执行,total 与筛选同口径。
   */
  async listReports(experimentId: string, params?: {
    status?: ExperimentReportStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<ReportDTO>> {
    return this.client.get(`/experiment/experiments/${encodePathSegment(experimentId)}/reports`, params)
  }

  /**
   * 教师换取实验报告的一次性下载授权。
   */
  async issueReportAccess(reportId: string): Promise<ReportAccessDTO> {
    return this.client.post(`/experiment/reports/${encodePathSegment(reportId)}/access`)
  }

  /**
   * 教师批改实验报告。
   */
  async gradeReport(reportId: string, data: GradeReportRequest): Promise<ReportDTO> {
    return this.client.post(`/experiment/reports/${encodePathSegment(reportId)}/grade`, data)
  }

  /**
   * 创建实验协作小组。
   */
  async createGroup(experimentId: string, data: ExperimentGroupRequest): Promise<ExperimentGroup> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/groups`, data)
  }

  /**
   * 查询本实验全部协作小组(教师编组视角)。
   * 与 getGroup 的差别:这里按实验列全部分组、不含共享实例;getGroup 按组读单组并附带实例。
   */
  async listGroups(experimentId: string): Promise<ExperimentGroup[]> {
    return this.client.get(`/experiment/experiments/${encodePathSegment(experimentId)}/groups`)
  }

  /**
   * 加入或调整协作小组成员角色。
   */
  async upsertGroupMember(groupId: string, data: ExperimentGroupMemberRequest): Promise<ExperimentGroup> {
    return this.client.post(`/experiment/groups/${encodePathSegment(groupId)}/members`, data)
  }

  /**
   * 创建实验实例（学生发起）
   */
  async createInstance(experimentId: string, data: CreateInstanceRequest): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/experiments/${encodePathSegment(experimentId)}/instances`, data)
  }

  /**
   * 获取实验实例详情
   */
  async getInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.get(`/experiment/instances/${encodePathSegment(instanceId)}`)
  }

  /**
   * 触发检查点判分
   */
  async judgeCheckpoint(
    instanceId: string,
    checkpointId: string,
    data: CheckpointJudgeRequest
  ): Promise<CheckpointResult> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/checkpoints/${encodePathSegment(checkpointId)}/judge`, data)
  }

  /**
   * 上传并提交实验报告。
   */
  async submitReport(
    instanceId: string,
    file: File,
    onProgress?: (progress: number) => void,
  ): Promise<ReportDTO> {
    return this.client.upload(
      `/experiment/instances/${encodePathSegment(instanceId)}/report`,
      file,
      onProgress,
    )
  }

  /**
   * 获取实验进度订阅信息
   */
  async getProgress(instanceId: string): Promise<ProgressDTO> {
    return this.client.get(`/experiment/instances/${encodePathSegment(instanceId)}/progress`)
  }

  /**
   * 读取协作小组详情。
   */
  async getGroup(groupId: string): Promise<ExperimentGroup> {
    return this.client.get(`/experiment/groups/${encodePathSegment(groupId)}`)
  }

  /**
   * 暂停实验实例
   */
  async pauseInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/pause`)
  }

  /**
   * 恢复实验实例
   */
  async resumeInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/resume`)
  }

  /**
   * 激活已解锁阶段
   */
  async activateStage(instanceId: string, stage: number): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/stages/${encodePathSegment(String(stage))}/activate`)
  }

  /**
   * 完成实验实例
   */
  async finishInstance(instanceId: string): Promise<ExperimentInstance> {
    return this.client.post(`/experiment/instances/${encodePathSegment(instanceId)}/finish`)
  }
}
