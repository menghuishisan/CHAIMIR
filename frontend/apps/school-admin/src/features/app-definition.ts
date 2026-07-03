// 学校管理端路由：看板、用户组织、成绩治理、配置和审计页面定义。

import { Building2, Gauge, Gavel, History, KeyRound, LineChart, ListChecks, ScrollText, Settings, ShieldAlert, UserCog, Users, Upload } from 'lucide-react'
import type { AppDefinition } from '@chaimir/shared'
import {
  accountColumns,
  accountTarget,
  alertActions,
  alertColumns,
  appealColumns,
  appealReviewActions,
  arrayResult,
  auditColumns,
  dashboardResult,
  defaultRange,
  fileInput,
  gradeConfigActions,
  gradeReviewColumns,
  hiddenResourceRoute,
  importBatchColumns,
  levelConfigColumns,
  listResult,
  numberInput,
  objectResult,
  optionalNumber,
  optionalText,
  orgColumns,
  pageAction,
  passwordInput,
  rowAction,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  ssoColumns,
  statisticsColumns,
  tenantConfigColumns,
  textInput,
  textareaInput,
  valueFile,
  valueJson,
  valueNumber,
  valueText,
  warningColumns,
} from '@chaimir/shared'

export const schoolAdminApp: AppDefinition = {
  role: 'school-admin',
  title: '学校管理端',
  subtitle: '学校租户内账号、组织、成绩、审计与运行配置',
  homePath: 'dashboard',
  routes: [
    {
      path: 'dashboard',
      label: '学校看板',
      description: '查看学校课程、实验、竞赛和沙箱运行概览',
      icon: Gauge,
      load: async (api) => dashboardResult(await api.admin.getSchoolDashboard()),
    },
    {
      path: 'accounts',
      label: '账号管理',
      description: '管理教师、学生、启停用、导入批次和激活码',
      icon: Users,
      load: async (api) => ({
        ...listResult(await api.identity.getAccounts({ page: 1, size: 20 }), accountColumns(), '暂无账号', '导入或创建师生账号后会在这里显示。'),
        actions: [
          pageAction('create-account', '创建账号', '创建单个教师或学生账号，激活码由后端按策略返回。', [
            textInput('phone', '手机号', true),
            textInput('name', '姓名', true),
            textInput('no', '学工号', true),
            numberInput('base_identity', '基础身份', true),
            textInput('org_id', '组织编号', true),
            numberInput('enrollment_year', '入学年份'),
            textInput('title', '职称'),
            passwordInput('initial_password', '初始密码'),
            numberInput('use_activation', '使用激活码', true, '1 表示使用，0 表示不使用。'),
          ], async (values) => {
            await api.identity.createAccount({
              phone: valueText(values, 'phone'),
              name: valueText(values, 'name'),
              no: valueText(values, 'no'),
              base_identity: valueNumber(values, 'base_identity'),
              org_id: valueText(values, 'org_id'),
              enrollment_year: optionalNumber(values, 'enrollment_year'),
              title: optionalText(values, 'title'),
              initial_password: optionalText(values, 'initial_password'),
              use_activation: valueNumber(values, 'use_activation') === 1,
            })
            return '账号已创建'
          }),
          pageAction('preview-account-import', '预览账号导入', '上传教师或学生导入文件，预览结果由服务端持久化。', [
            textInput('target_type', '账号类型', true, '可填写教师或学生。'),
            fileInput('file', '导入文件', true),
          ], async (values) => {
            await api.identity.previewAccountImport(accountTarget(values), valueFile(values, 'file'))
            return '账号导入预览已生成'
          }),
          pageAction('commit-account-import', '提交导入批次', '确认已预览的导入批次。', [textInput('preview_id', '预览编号', true)], async (values) => {
            await api.identity.commitAccountImport({ preview_id: valueText(values, 'preview_id') })
            return '账号导入批次已提交'
          }),
        ],
        rowActions: [
          rowAction('disable-account', '停用', '停用账号并吊销相关会话。', async (row) => {
            await api.identity.disableAccount(row.id)
            return '账号已停用'
          }),
          rowAction('enable-account', '启用', '启用已停用账号。', async (row) => {
            await api.identity.enableAccount(row.id)
            return '账号已启用'
          }),
        ],
      }),
    },
    {
      path: 'org',
      label: '组织架构',
      description: '维护院系、专业和班级',
      icon: Building2,
      load: async (api) => ({
        ...arrayResult(await api.identity.listDepartments(), orgColumns(), '暂无院系', '创建院系后会在这里显示。'),
        actions: [
          pageAction('create-department', '创建院系', '创建院系基础信息。', [
            textInput('name', '院系名称', true),
            textInput('code', '院系编码', true),
          ], async (values) => {
            await api.identity.createDepartment({ name: valueText(values, 'name'), code: valueText(values, 'code') })
            return '院系已创建'
          }),
          pageAction('create-major', '创建专业', '创建专业并绑定所属院系。', [
            textInput('department_id', '院系编号', true),
            textInput('name', '专业名称', true),
          ], async (values) => {
            await api.identity.createMajor({ department_id: valueText(values, 'department_id'), name: valueText(values, 'name') })
            return '专业已创建'
          }),
          pageAction('create-class', '创建班级', '创建班级并绑定所属专业。', [
            textInput('major_id', '专业编号', true),
            textInput('name', '班级名称', true),
            numberInput('enrollment_year', '入学年份', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.identity.createClass({
              major_id: valueText(values, 'major_id'),
              name: valueText(values, 'name'),
              enrollment_year: valueNumber(values, 'enrollment_year'),
              status: valueNumber(values, 'status'),
            })
            return '班级已创建'
          }),
          pageAction('preview-org-import', '预览组织导入', '上传组织架构文件并生成服务端预览。', [fileInput('file', '导入文件', true)], async (values) => {
            await api.identity.previewOrgImport(valueFile(values, 'file'))
            return '组织导入预览已生成'
          }),
        ],
      }),
    },
    {
      path: 'grade-reviews',
      label: '成绩审核',
      description: '审核课程成绩归档、解锁和驳回申请',
      icon: ScrollText,
      load: async (api) => ({
        ...listResult(await api.grade.listReviews({ page: 1, size: 20 }), gradeReviewColumns(), '暂无审核申请', '教师提交课程成绩审核后会在这里显示。'),
        actions: [
          pageAction('approve-grade-review', '通过审核', '通过课程成绩审核并锁定结果。', [
            textInput('review_id', '审核编号', true),
            textInput('semester_id', '学期编号'),
            textareaInput('comment', '审核说明'),
          ], async (values) => {
            await api.grade.approveReview(valueText(values, 'review_id'), {
              semester_id: optionalText(values, 'semester_id'),
              comment: optionalText(values, 'comment'),
            })
            return '成绩审核已通过'
          }),
          pageAction('reject-grade-review', '驳回审核', '驳回课程成绩审核并写入原因。', [
            textInput('review_id', '审核编号', true),
            textareaInput('comment', '驳回原因', true),
          ], async (values) => {
            await api.grade.rejectReview(valueText(values, 'review_id'), { comment: valueText(values, 'comment') })
            return '成绩审核已驳回'
          }),
          pageAction('unlock-grade-review', '解锁成绩', '解锁已通过的课程成绩审核。', [
            textInput('review_id', '审核编号', true),
            textareaInput('comment', '解锁说明', true),
          ], async (values) => {
            await api.grade.unlockReview(valueText(values, 'review_id'), { comment: valueText(values, 'comment') })
            return '成绩审核已解锁'
          }),
        ],
      }),
    },
    {
      path: 'warnings',
      label: '学业预警',
      description: '查看预警规则命中的学生与处理状态',
      icon: ShieldAlert,
      load: async (api) => ({
        ...listResult(await api.grade.listWarnings({ page: 1, size: 20 }), warningColumns(), '暂无预警', '触发学业预警后会在这里显示。'),
        actions: [
          pageAction('scan-warnings', '扫描预警', '按学生或学期触发学业预警扫描。', [
            textInput('student_id', '学生编号'),
            textInput('semester_id', '学期编号'),
          ], async (values) => {
            await api.grade.scanWarnings({
              student_id: optionalText(values, 'student_id'),
              semester_id: optionalText(values, 'semester_id'),
            })
            return '学业预警扫描已触发'
          }),
        ],
      }),
    },
    {
      path: 'config',
      label: '租户配置',
      description: '维护学校展示、认证方式和功能开关',
      icon: Settings,
      load: async (api) => ({
        ...objectResult(await api.identity.getTenantConfig(), tenantConfigColumns(), '租户配置'),
        actions: [
          pageAction('update-tenant-config', '更新租户配置', '更新学校展示、认证方式和功能开关配置。', [
            textInput('logo_url', '标识地址', true),
            textInput('display_name', '展示名称', true),
            textareaInput('feature_flags', '功能开关', true),
            numberInput('auth_mode', '认证方式', true),
            numberInput('enable_activation_code', '启用激活码', true, '1 表示启用，0 表示关闭。'),
          ], async (values) => {
            await api.identity.updateTenantConfig({
              logo_url: valueText(values, 'logo_url'),
              display_name: valueText(values, 'display_name'),
              feature_flags: valueJson(values, 'feature_flags'),
              auth_mode: valueNumber(values, 'auth_mode'),
              enable_activation_code: valueNumber(values, 'enable_activation_code') === 1,
            })
            return '租户配置已更新'
          }),
        ],
      }),
    },
    ...schoolAdminDeepRoutes(),
    {
      path: 'audit',
      label: '审计日志',
      description: '查询学校内敏感操作审计记录',
      icon: ListChecks,
      load: async (api) => ({
        ...listResult(await api.admin.queryAudit({ page: 1, size: 20 }), auditColumns(), '暂无审计记录', '用户完成敏感操作后会在这里显示。'),
        actions: [
          pageAction('export-audit', '导出审计', '创建审计日志导出任务。', [
            textInput('actor_id', '操作者编号'),
            textInput('action', '操作类型'),
            textInput('target_type', '对象类型'),
          ], async (values) => {
            await api.admin.exportAudit({
              actor_id: optionalText(values, 'actor_id'),
              action: optionalText(values, 'action'),
              target_type: optionalText(values, 'target_type'),
            })
            return '审计导出任务已创建'
          }),
        ],
      }),
    },
    sharedNotificationRoute(),
    sharedAnnouncementRoute(),
    sharedProfileRoute(),
  ],
}


function schoolAdminDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('statistics', '运营统计', '查看本校趋势统计和资源使用变化', LineChart, async (api) => arrayResult(await api.admin.getSchoolStatistics(defaultRange()), statisticsColumns(), '暂无统计', '业务运行后会生成趋势统计。')),
    hiddenResourceRoute('account-import', '账号导入', '预览并提交师生导入批次', Upload, async (api) => ({
      ...arrayResult(await api.identity.listAccountImportBatches(), importBatchColumns(), '暂无导入记录', '上传导入文件并预览后会显示。'),
      actions: [
        pageAction('preview-import', '预览账号导入', '上传教师或学生导入文件并生成预览。', [
          textInput('target_type', '账号类型', true, '可填写教师或学生。'),
          fileInput('file', '导入文件', true),
        ], async (values) => {
          await api.identity.previewAccountImport(accountTarget(values), valueFile(values, 'file'))
          return '导入预览已生成'
        }),
        pageAction('commit-import', '确认导入', '确认服务端预览批次并写入账号。', [textInput('preview_id', '预览编号', true)], async (values) => {
          await api.identity.commitAccountImport({ preview_id: valueText(values, 'preview_id') })
          return '账号导入已提交'
        }),
      ],
    })),
    hiddenResourceRoute('account-edit', '账号编辑', '新增、启停、归档和重置账号', UserCog, async (api) => listResult(await api.identity.getAccounts({ page: 1, size: 20 }), accountColumns(), '暂无账号', '账号创建后会显示。')),
    hiddenResourceRoute('import-batches', '导入记录', '查看账号与组织导入批次', History, async (api) => arrayResult(await api.identity.listAccountImportBatches(), importBatchColumns(), '暂无导入记录', '导入批次提交后会显示。')),
    hiddenResourceRoute('appeals', '申诉处理', '处理学生成绩申诉', Gavel, async (api) => ({
      ...listResult(await api.grade.listAppeals({ page: 1, size: 20 }), appealColumns(), '暂无申诉', '学生提交成绩申诉后会显示。'),
      actions: appealReviewActions(api),
    })),
    hiddenResourceRoute('grade-config', '成绩配置', '维护等级映射、学期和预警规则', Settings, async (api) => ({
      ...arrayResult(await api.grade.listLevelConfigs(), levelConfigColumns(), '暂无成绩配置', '创建等级配置后会显示。'),
      actions: gradeConfigActions(api),
    })),
    hiddenResourceRoute('sso', '认证配置', '维护 CAS 或 LDAP 认证配置', KeyRound, async (api) => ({
      ...arrayResult(await api.identity.listSSOConfigs(), ssoColumns(), '暂无认证配置', '保存 CAS 或 LDAP 配置后会显示。'),
      actions: [
        pageAction('upsert-sso', '保存认证配置', '保存学校统一认证配置。', [
          numberInput('type', '认证类型', true),
          textareaInput('config', '认证配置', true),
          numberInput('match_field', '匹配字段', true),
          numberInput('enabled', '是否启用', true),
        ], async (values) => {
          await api.identity.upsertSSOConfig({
            type: valueNumber(values, 'type'),
            config: valueJson(values, 'config'),
            match_field: valueNumber(values, 'match_field'),
            enabled: valueNumber(values, 'enabled') === 1,
          })
          return '认证配置已保存'
        }),
      ],
    })),
    hiddenResourceRoute('alerts', '学校告警', '查看本校告警事件和规则', ShieldAlert, async (api) => ({
      ...listResult(await api.admin.listAlertEvents({ page: 1, size: 20 }), alertColumns(), '暂无告警', '触发告警规则后会显示。'),
      actions: alertActions(api),
    })),
  ]
}

/**
 * platformTenantDeepRoutes 补齐平台租户详情和平台统计页。
 */
