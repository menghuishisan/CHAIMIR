// 学校管理端路由：看板、用户组织、成绩治理、配置和审计页面定义。

import { IMPORT_TEMPLATE_FORMAT } from '@chaimir/api-client'
import { Building2, Gauge, Gavel, History, KeyRound, LineChart, ListChecks, ScrollText, Settings, ShieldAlert, UserCog, Users, Upload } from 'lucide-react'
import type { AppDefinition, MetricItem, ResourceResult } from '../app/types'
import { accountImportTemplateFilename, DOWNLOAD_FILENAMES } from '../copy/downloads'
import { downloadBlob } from '../lib/browser'
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
  defaultPageParams,
  defaultRange,
  fileInput,
  gradeConfigActions,
  gradeReviewColumns,
  hiddenResourceRoute,
  importBatchColumns,
  levelConfigColumns,
  listResult,
  navigatePageAction,
  navigateRowAction,
  numberInput,
  objectResult,
  optionalNumber,
  optionalText,
  orgColumns,
  pageAction,
  passwordInput,
  rowAction,
  resourceRoute,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  sharedTransferRoute,
  ssoColumns,
  statisticsColumns,
  tenantConfigColumns,
  textInput,
  textareaInput,
  valueFile,
  valueFlag,
  valueJson,
  valueNumber,
  valueNumberArray,
  valueText,
  warningColumns,
} from '../route-kit'

export const schoolAdminApp: AppDefinition = {
  role: 'school-admin',
  title: '学校管理端',
  subtitle: '学校租户内账号、组织、成绩、审计与运行配置',
  homePath: 'accounts',
  routes: [
    {
      path: 'accounts',
      label: '账号管理',
      description: '管理教师、学生、启停用、导入批次和激活码',
      icon: Users,
      group: '用户与组织',
      load: async (api) => ({
        ...schoolMetrics(listResult(await api.identity.getAccounts(defaultPageParams()), accountColumns(), '暂无账号', '导入或创建师生账号后会在这里显示。'), '账号记录', '启停归档', '导入预览'),
        actions: [
          navigatePageAction('open-account-import-page', '账号导入', '进入账号导入预览和提交页面。', 'account-import'),
          navigatePageAction('open-import-batches-page', '导入记录', '进入账号与组织导入批次页面。', 'import-batches'),
          pageAction('create-account', '创建账号', '创建单个教师或学生账号，创建后会生成激活信息。', [
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
              use_activation: valueFlag(values, 'use_activation'),
            })
            return '账号已创建'
          }),
          pageAction('preview-account-import', '预览账号导入', '上传教师或学生导入文件，预览结果会自动保存。', [
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
          pageAction('update-account', '更新账号', '更新教师或学生账号基础资料。', [
            textInput('account_id', '账号编号', true),
            textInput('name', '姓名', true),
            textInput('org_id', '组织编号', true),
            numberInput('enrollment_year', '入学年份'),
            textInput('title', '职称'),
          ], async (values) => {
            await api.identity.updateAccount(valueText(values, 'account_id'), {
              name: valueText(values, 'name'),
              org_id: valueText(values, 'org_id'),
              enrollment_year: optionalNumber(values, 'enrollment_year'),
              title: optionalText(values, 'title'),
            })
            return '账号已更新'
          }),
          pageAction('reset-account-password', '重置密码', '为账号重置初始密码。', [
            textInput('account_id', '账号编号', true),
            passwordInput('new_password', '新密码', true),
          ], async (values) => {
            await api.identity.resetAccountPassword(valueText(values, 'account_id'), { new_password: valueText(values, 'new_password'), must_change_pwd: true })
            return '账号密码已重置'
          }),
          pageAction('download-account-template', '获取导入模板', '读取账号导入模板文件授权。', [textInput('target_type', '账号类型', true, '可填写教师或学生。')], async (values) => {
            const target = accountTarget(values)
            const blob = await api.identity.downloadAccountImportTemplate({ type: target, format: IMPORT_TEMPLATE_FORMAT.XLSX })
            downloadBlob(blob, accountImportTemplateFilename(target))
            return '账号导入模板已开始下载'
          }),
          pageAction('batch-disable-accounts', '批量停用账号', '按账号编号批量停用账号。', [textInput('account_ids', '账号编号', true, '多个编号用英文逗号分隔。')], async (values) => {
            await api.identity.batchDisableAccounts({ account_ids: valueNumberArray(values, 'account_ids') })
            return '账号批量停用已提交'
          }),
          pageAction('batch-restore-accounts', '批量恢复账号', '按账号编号批量恢复账号。', [textInput('account_ids', '账号编号', true, '多个编号用英文逗号分隔。')], async (values) => {
            await api.identity.batchRestoreAccounts({ account_ids: valueNumberArray(values, 'account_ids') })
            return '账号批量恢复已提交'
          }),
          pageAction('batch-archive-accounts', '按年级归档账号', '按入学年份批量归档学生账号和班级。', [numberInput('enrollment_year', '入学年份', true)], async (values) => {
            await api.identity.batchArchiveAccounts({ enrollment_year: valueNumber(values, 'enrollment_year') })
            return '账号批量归档已提交'
          }),
        ],
        rowActions: [
          navigateRowAction('open-account-edit', '编辑账号', '进入账号编辑和重置页面。', 'account-edit', 'account_id'),
          rowAction('disable-account', '停用', '停用账号并吊销相关会话。', async (row) => {
            await api.identity.disableAccount(row.id)
            return '账号已停用'
          }),
          rowAction('enable-account', '启用', '启用已停用账号。', async (row) => {
            await api.identity.enableAccount(row.id)
            return '账号已启用'
          }),
          rowAction('archive-account', '归档', '归档不再使用的账号。', async (row) => {
            await api.identity.archiveAccount(row.id)
            return '账号已归档'
          }),
          rowAction('restore-account', '恢复', '恢复已归档账号。', async (row) => {
            await api.identity.restoreAccount(row.id)
            return '账号已恢复'
          }),
          rowAction('force-logout-account', '下线', '吊销该账号当前会话。', async (row) => {
            await api.identity.forceLogoutAccount(row.id)
            return '账号会话已下线'
          }),
          rowAction('cancel-account', '注销', '注销账号并保留审计记录。', async (row) => {
            await api.identity.cancelAccount(row.id)
            return '账号已注销'
          }),
          rowAction('grant-admin', '授予管理员', '授予学校管理员身份。', async (row) => {
            await api.identity.grantSchoolAdmin(row.id)
            return '管理员身份已授予'
          }),
          rowAction('revoke-admin', '取消管理员', '取消学校管理员身份。', async (row) => {
            await api.identity.revokeSchoolAdmin(row.id)
            return '管理员身份已取消'
          }),
        ],
      }),
    },
    {
      path: 'org',
      label: '组织架构',
      description: '维护院系、专业和班级',
      icon: Building2,
      group: '用户与组织',
      load: async (api) => ({
        ...schoolMetrics(arrayResult(await api.identity.listDepartments(), orgColumns(), '暂无院系', '创建院系后会在这里显示。'), '组织节点', '院系专业', '班级升届'),
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
          pageAction('preview-org-import', '预览组织导入', '上传组织架构文件并生成导入预览。', [fileInput('file', '导入文件', true)], async (values) => {
            await api.identity.previewOrgImport(valueFile(values, 'file'))
            return '组织导入预览已生成'
          }),
          pageAction('commit-org-import', '提交组织导入', '确认已预览的组织导入批次。', [textInput('preview_id', '预览编号', true)], async (values) => {
            await api.identity.commitOrgImport({ preview_id: valueText(values, 'preview_id') })
            return '组织导入已提交'
          }),
          pageAction('download-org-template', '获取组织模板', '读取组织导入模板文件授权。', [], async () => {
            const blob = await api.identity.downloadOrgImportTemplate({ format: IMPORT_TEMPLATE_FORMAT.XLSX })
            downloadBlob(blob, DOWNLOAD_FILENAMES.ORG_IMPORT_TEMPLATE)
            return '组织导入模板已开始下载'
          }),
          pageAction('list-majors', '查看专业', '按院系编号读取专业列表。', [textInput('department_id', '院系编号', true)], async (values) => {
            await api.identity.listMajors({ department_id: valueText(values, 'department_id') })
            return '专业列表已读取'
          }),
          pageAction('list-classes', '查看班级', '按专业编号读取班级列表。', [textInput('major_id', '专业编号', true)], async (values) => {
            await api.identity.listClasses({ major_id: valueText(values, 'major_id') })
            return '班级列表已读取'
          }),
          pageAction('promote-classes', '班级升届', '批量推进班级年级状态。', [], async () => {
            await api.identity.promoteClasses()
            return '班级升届已触发'
          }),
          pageAction('archive-classes', '归档班级', '按入学年份批量归档班级。', [numberInput('enrollment_year', '入学年份', true)], async (values) => {
            await api.identity.archiveClasses({ enrollment_year: valueNumber(values, 'enrollment_year') })
            return '班级归档已提交'
          }),
          pageAction('update-major', '更新专业', '更新专业名称和所属院系。', [
            textInput('major_id', '专业编号', true),
            textInput('department_id', '院系编号', true),
            textInput('name', '专业名称', true),
          ], async (values) => {
            await api.identity.updateMajor(valueText(values, 'major_id'), {
              department_id: valueText(values, 'department_id'),
              name: valueText(values, 'name'),
            })
            return '专业已更新'
          }),
          pageAction('delete-major', '删除专业', '删除未被引用的专业。', [textInput('major_id', '专业编号', true)], async (values) => {
            await api.identity.deleteMajor(valueText(values, 'major_id'))
            return '专业已删除'
          }),
          pageAction('update-class', '更新班级', '更新班级名称、年份和状态。', [
            textInput('class_id', '班级编号', true),
            textInput('major_id', '专业编号', true),
            textInput('name', '班级名称', true),
            numberInput('enrollment_year', '入学年份', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.identity.updateClass(valueText(values, 'class_id'), {
              major_id: valueText(values, 'major_id'),
              name: valueText(values, 'name'),
              enrollment_year: valueNumber(values, 'enrollment_year'),
              status: valueNumber(values, 'status'),
            })
            return '班级已更新'
          }),
          pageAction('delete-class', '删除班级', '删除未被引用的班级。', [textInput('class_id', '班级编号', true)], async (values) => {
            await api.identity.deleteClass(valueText(values, 'class_id'))
            return '班级已删除'
          }),
        ],
        rowActions: [
          rowAction('update-department', '更新院系', '按当前行编号更新院系名称和编码。', async (row) => {
            await api.identity.updateDepartment(row.id, { name: String(row.name ?? ''), code: String(row.code ?? '') })
            return '院系已更新'
          }),
          rowAction('delete-department', '删除院系', '删除未被引用的院系。', async (row) => {
            await api.identity.deleteDepartment(row.id)
            return '院系已删除'
          }),
        ],
      }),
    },
    {
      path: 'dashboard',
      label: '学校看板',
      description: '查看学校课程、实验、竞赛和沙箱运行概览',
      icon: Gauge,
      group: '概览',
      load: async (api) => ({
        ...schoolMetrics(dashboardResult(await api.admin.getSchoolDashboard()), '学校概览', '运行数据', '趋势统计'),
        actions: [
          navigatePageAction('open-school-statistics-page', '趋势统计', '进入本校趋势统计和资源使用页面。', 'statistics'),
        ],
      }),
    },
    {
      path: 'grade-reviews',
      label: '成绩审核',
      description: '审核课程成绩归档、解锁和驳回申请',
      icon: ScrollText,
      group: '成绩',
      load: async (api) => ({
        ...schoolMetrics(listResult(await api.grade.listReviews(defaultPageParams()), gradeReviewColumns(), '暂无审核申请', '教师提交课程成绩审核后会在这里显示。'), '审核申请', '通过驳回', '成绩锁定'),
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
      group: '成绩',
      load: async (api) => ({
        ...schoolMetrics(listResult(await api.grade.listWarnings(defaultPageParams()), warningColumns(), '暂无预警', '触发学业预警后会在这里显示。'), '预警记录', '扫描规则', '干预确认'),
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
      group: '系统',
      load: async (api) => ({
        ...schoolMetrics(objectResult(await api.identity.getTenantConfig(), tenantConfigColumns(), '租户配置'), '配置状态', '认证方式', '功能开关'),
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
              enable_activation_code: valueFlag(values, 'enable_activation_code'),
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
      group: '系统',
      load: async (api) => ({
        ...schoolMetrics(listResult(await api.admin.queryAudit(defaultPageParams()), auditColumns(), '暂无审计记录', '用户完成敏感操作后会在这里显示。'), '审计记录', '敏感操作', '导出任务'),
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
          pageAction('query-identity-audit', '查询身份审计', '从身份模块读取学校内审计日志。', [
            textInput('actor_id', '操作者编号'),
            textInput('action', '操作类型'),
            textInput('target_type', '对象类型'),
          ], async (values) => {
            await api.identity.getAuditLogs({
              actor_id: optionalText(values, 'actor_id'),
              action: optionalText(values, 'action'),
              target_type: optionalText(values, 'target_type'),
              page: 1,
              size: 20,
            })
            return '身份审计已读取'
          }),
        ],
      }),
    },
    sharedNotificationRoute(),
    sharedAnnouncementRoute(),
    sharedTransferRoute(),
    sharedProfileRoute(),
  ],
}
/**
 * schoolAdminDeepRoutes 补齐学校管理端统计、导入、申诉、成绩配置、认证和告警页。
 */
function schoolAdminDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('statistics', '运营统计', '查看本校趋势统计和资源使用变化', LineChart, async (api) => arrayResult(await api.admin.getSchoolStatistics(defaultRange()), statisticsColumns(), '暂无统计', '业务运行后会生成趋势统计。'), '概览'),
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
        pageAction('commit-import', '确认导入', '确认导入预览批次并写入账号。', [textInput('preview_id', '预览编号', true)], async (values) => {
          await api.identity.commitAccountImport({ preview_id: valueText(values, 'preview_id') })
          return '账号导入已提交'
        }),
      ],
    })),
    hiddenResourceRoute('account-edit', '账号编辑', '新增、启停、归档和重置账号', UserCog, async (api) => listResult(await api.identity.getAccounts(defaultPageParams()), accountColumns(), '暂无账号', '账号创建后会显示。')),
    hiddenResourceRoute('import-batches', '导入记录', '查看账号与组织导入批次', History, async (api) => arrayResult(await api.identity.listAccountImportBatches(), importBatchColumns(), '暂无导入记录', '导入批次提交后会显示。'), '用户与组织'),
    resourceRoute('appeals', '申诉处理', '处理学生成绩申诉', Gavel, async (api) => ({
      ...schoolMetrics(listResult(await api.grade.listAppeals(defaultPageParams()), appealColumns(), '暂无申诉', '学生提交成绩申诉后会显示。'), '申诉记录', '受理驳回', '处理反馈'),
      actions: appealReviewActions(api),
    }), '成绩'),
    resourceRoute('grade-config', '成绩配置', '维护等级映射、学期和预警规则', Settings, async (api) => ({
      ...schoolMetrics(arrayResult(await api.grade.listLevelConfigs(), levelConfigColumns(), '暂无成绩配置', '创建等级配置后会显示。'), '等级规则', '学期规则', '预警阈值'),
      actions: gradeConfigActions(api),
    }), '成绩'),
    resourceRoute('sso', '认证配置', '维护 CAS 或 LDAP 认证配置', KeyRound, async (api) => ({
      ...schoolMetrics(arrayResult(await api.identity.listSSOConfigs(), ssoColumns(), '暂无认证配置', '保存 CAS 或 LDAP 配置后会显示。'), '认证配置', 'CAS/LDAP', '启用状态'),
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
            enabled: valueFlag(values, 'enabled'),
          })
          return '认证配置已保存'
        }),
      ],
    }), '系统'),
    resourceRoute('alerts', '学校告警', '查看本校告警事件和规则', ShieldAlert, async (api) => ({
      ...schoolMetrics(listResult(await api.admin.listAlertEvents(defaultPageParams()), alertColumns(), '暂无告警', '触发告警规则后会显示。'), '告警事件', '规则维护', '处理闭环'),
      actions: alertActions(api),
    }), '系统'),
  ]
}

/**
 * schoolMetrics 为学校管理端提供校内治理指标，数值只来自当前资源结果。
 */
function schoolMetrics(result: ResourceResult, primaryLabel: string, governanceLabel: string, actionLabel: string): ResourceResult {
  const metrics: MetricItem[] = [
    { label: primaryLabel, value: String(result.rows.length), tone: 'primary' },
    { label: governanceLabel, value: result.rows.length > 0 ? '可处理' : '暂无待办', tone: result.rows.length > 0 ? 'warning' : 'secondary' },
    { label: '治理动作', value: actionLabel, tone: 'success' },
  ]
  return { ...result, metrics }
}
