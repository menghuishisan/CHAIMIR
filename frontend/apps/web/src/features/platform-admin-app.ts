// 平台管理端路由：租户、运营、引擎治理和系统页面定义。

import { BarChart3, Boxes, ClipboardCheck, Flag, Gauge, Landmark, ListChecks, MonitorCog, PackageCheck, ServerCog, Settings, ShieldAlert, Wrench } from 'lucide-react'
import type { AppDefinition } from '@chaimir/shared'
import {
  alertColumns,
  alertRuleColumns,
  applicationColumns,
  arrayResult,
  auditColumns,
  backupColumns,
  configColumns,
  dashboardResult,
  defaultPageParams,
  datetimeInput,
  defaultRange,
  hiddenResourceRoute,
  judgerColumns,
  listResult,
  monitoringColumns,
  normalizeBackupSize,
  numberInput,
  objectResult,
  optionalText,
  pageAction,
  quotaColumns,
  routeParam,
  rowAction,
  resourceRoute,
  runtimeColumns,
  runtimeImageColumns,
  sharedAnnouncementRoute,
  sharedNotificationRoute,
  sharedProfileRoute,
  sharedTransferRoute,
  simGovernanceActions,
  simReviewColumns,
  statisticsColumns,
  tenantColumns,
  textInput,
  textareaInput,
  toolColumns,
  valueFlag,
  valueJson,
  valueNumber,
  valueStringArray,
  valueText,
  vulnProblemColumns,
} from '@chaimir/shared'

export const platformAdminApp: AppDefinition = {
  role: 'platform-admin',
  title: '平台管理端',
  subtitle: '平台租户、运行时、工具、告警、审计和运维能力',
  homePath: 'tenants',
  routes: [
    {
      path: 'tenants',
      label: '学校管理',
      description: '管理学校租户、状态、部署形态和到期时间',
      icon: Landmark,
      group: '租户',
      load: async (api) => ({
        ...arrayResult(await api.admin.listTenants(), tenantColumns(), '暂无学校租户', '学校通过入驻审核后会在这里显示。'),
        actions: [
          pageAction('update-tenant-status', '更新学校状态', '更新学校状态和到期时间。', [
            textInput('tenant_id', '学校编号', true),
            numberInput('status', '状态', true),
            datetimeInput('expire_at', '到期时间'),
          ], async (values) => {
            await api.identity.updateTenant(valueText(values, 'tenant_id'), {
              status: valueNumber(values, 'status'),
              expire_at: optionalText(values, 'expire_at'),
            })
            return '学校状态已更新'
          }),
          pageAction('read-identity-tenants', '读取租户原始列表', '从身份模块读取租户原始分页数据。', [], async () => {
            await api.identity.getTenants(defaultPageParams())
            return '租户原始列表已读取'
          }),
        ],
      }),
    },
    ...platformTenantDeepRoutes(),
    {
      path: 'applications',
      label: '入驻申请',
      description: '审核学校入驻申请并开通租户',
      icon: ClipboardCheck,
      group: '租户',
      load: async (api) => ({
        ...arrayResult(await api.admin.listApplications(), applicationColumns(), '暂无入驻申请', '学校提交入驻申请后会在这里显示。'),
        actions: [
          pageAction('approve-application', '通过申请', '通过学校入驻申请并创建租户。', [
            textInput('application_id', '申请编号', true),
            textInput('tenant_code', '租户编码', true),
            textInput('admin_name', '管理员姓名', true),
            textInput('admin_phone', '管理员手机号', true),
          ], async (values) => {
            await api.identity.approveApplication(valueText(values, 'application_id'), {
              tenant_code: valueText(values, 'tenant_code'),
              admin_name: valueText(values, 'admin_name'),
              admin_phone: valueText(values, 'admin_phone'),
            })
            return '入驻申请已通过'
          }),
          pageAction('reject-application', '驳回申请', '驳回学校入驻申请并写入原因。', [
            textInput('application_id', '申请编号', true),
            textareaInput('reason', '驳回原因', true),
          ], async (values) => {
            await api.identity.rejectApplication(valueText(values, 'application_id'), { reason: valueText(values, 'reason') })
            return '入驻申请已驳回'
          }),
          pageAction('read-identity-applications', '读取申请原始列表', '从身份模块读取入驻申请原始数据。', [], async () => {
            await api.identity.getApplications()
            return '入驻申请原始列表已读取'
          }),
        ],
      }),
    },
    ...platformApplicationDeepRoutes(),
    {
      path: 'dashboard',
      label: '平台看板',
      description: '查看平台租户、账号、课程、竞赛和沙箱概览',
      icon: Gauge,
      group: '概览',
      load: async (api) => dashboardResult(await api.admin.getPlatformDashboard()),
    },
    {
      path: 'runtimes',
      label: '链运行时',
      description: '管理链运行时、镜像版本和接入自测',
      icon: ServerCog,
      group: '引擎',
      load: async (api) => ({
        ...arrayResult(await api.sandbox.listRuntimes(), runtimeColumns(), '暂无运行时', '登记链运行时后会在这里显示。'),
        actions: [
          pageAction('register-runtime', '登记运行时', '登记新的链运行时声明。', [
            textInput('code', '运行时编码', true),
            textInput('name', '运行时名称', true),
            textInput('eco', '生态', true),
            numberInput('adapter_level', '适配等级', true),
            textareaInput('adapter_spec', '适配规格', true),
            textInput('capability_impl', '能力实现', true),
            textInput('plugin_ref', '插件引用', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.sandbox.registerRuntime({
              code: valueText(values, 'code'),
              name: valueText(values, 'name'),
              eco: valueText(values, 'eco'),
              adapter_level: valueNumber(values, 'adapter_level'),
              adapter_spec: valueJson(values, 'adapter_spec'),
              capability_impl: valueText(values, 'capability_impl'),
              plugin_ref: valueText(values, 'plugin_ref'),
              status: valueNumber(values, 'status'),
            })
            return '运行时已登记'
          }),
          pageAction('register-runtime-image', '登记镜像版本', '为运行时登记镜像版本和 digest。', [
            textInput('runtime_id', '运行时编号', true),
            textInput('image_url', '镜像地址', true),
            textInput('version', '版本', true),
            textInput('digest', '镜像摘要', true),
            numberInput('genesis_baked', '是否内置创世块', true),
            numberInput('is_default', '是否默认', true),
          ], async (values) => {
            await api.sandbox.registerRuntimeImage(valueText(values, 'runtime_id'), {
              image_url: valueText(values, 'image_url'),
              version: valueText(values, 'version'),
              digest: valueText(values, 'digest'),
              genesis_baked: valueFlag(values, 'genesis_baked'),
              is_default: valueFlag(values, 'is_default'),
            })
            return '运行时镜像版本已登记'
          }),
          pageAction('update-runtime', '更新运行时', '更新运行时声明和适配配置。', [
            textInput('runtime_id', '运行时编号', true),
            textInput('code', '运行时编码', true),
            textInput('name', '运行时名称', true),
            textInput('eco', '生态', true),
            numberInput('adapter_level', '适配等级', true),
            textareaInput('adapter_spec', '适配规格', true),
            textInput('capability_impl', '能力实现', true),
            textInput('plugin_ref', '插件引用', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.sandbox.updateRuntime(valueText(values, 'runtime_id'), {
              code: valueText(values, 'code'),
              name: valueText(values, 'name'),
              eco: valueText(values, 'eco'),
              adapter_level: valueNumber(values, 'adapter_level'),
              adapter_spec: valueJson(values, 'adapter_spec'),
              capability_impl: valueText(values, 'capability_impl'),
              plugin_ref: valueText(values, 'plugin_ref'),
              status: valueNumber(values, 'status'),
            })
            return '运行时已更新'
          }),
          pageAction('read-runtime-selftest', '查看自测结果', '读取运行时最近一次接入自测结果。', [textInput('runtime_id', '运行时编号', true)], async (values) => {
            await api.sandbox.getRuntimeSelftest(valueText(values, 'runtime_id'))
            return '运行时自测结果已读取'
          }),
        ],
        rowActions: [
          rowAction('runtime-selftest', '接入自测', '触发运行时接入自测。', async (row) => {
            await api.sandbox.runRuntimeSelftest(row.id)
            return '运行时自测已触发'
          }),
        ],
      }),
    },
    {
      path: 'tools',
      label: '沙箱工具',
      description: '管理 code-server、浏览器工具和命令工具定义',
      icon: Wrench,
      group: '引擎',
      load: async (api) => ({
        ...arrayResult(await api.sandbox.listTools(), toolColumns(), '暂无工具', '登记沙箱工具后会在这里显示。'),
        actions: [
          pageAction('register-tool', '登记工具', '登记沙箱工具定义。', [
            textInput('code', '工具编码', true),
            textInput('name', '工具名称', true),
            numberInput('kind', '工具类型', true),
            textInput('eco_tags', '生态标签', false, '多个标签用英文逗号分隔。'),
            textareaInput('resource_spec', '资源规格', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.sandbox.registerTool({
              code: valueText(values, 'code'),
              name: valueText(values, 'name'),
              kind: valueNumber(values, 'kind'),
              eco_tags: valueStringArray(values, 'eco_tags'),
              resource_spec: valueJson(values, 'resource_spec'),
              status: valueNumber(values, 'status'),
            })
            return '沙箱工具已登记'
          }),
        ],
      }),
    },
    {
      path: 'judgers',
      label: '判题器',
      description: '管理判题器执行器、自测和运行约束',
      icon: ClipboardCheck,
      group: '引擎',
      load: async (api) => ({
        ...arrayResult(await api.judge.listJudgers(), judgerColumns(), '暂无判题器', '登记判题器后会在这里显示。'),
        actions: [
          pageAction('create-judger', '登记判题器', '登记判题器执行器和资源约束。', [
            textInput('code', '判题器编码', true),
            textInput('name', '判题器名称', true),
            numberInput('type', '类型', true),
            textInput('executor_ref', '执行器引用', true),
            numberInput('runtime_required', '需要运行时', true),
            numberInput('default_timeout_sec', '默认等待秒数', true),
            textareaInput('resource_spec', '资源规格', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.judge.createJudger({
              code: valueText(values, 'code'),
              name: valueText(values, 'name'),
              type: valueNumber(values, 'type'),
              executor_ref: valueText(values, 'executor_ref'),
              runtime_required: valueFlag(values, 'runtime_required'),
              default_timeout_sec: valueNumber(values, 'default_timeout_sec'),
              resource_spec: valueJson(values, 'resource_spec'),
              status: valueNumber(values, 'status'),
            })
            return '判题器已登记'
          }),
          pageAction('update-judger', '更新判题器', '更新判题器执行器和资源约束。', [
            textInput('judger_id', '判题器编号', true),
            textInput('code', '判题器编码', true),
            textInput('name', '判题器名称', true),
            numberInput('type', '类型', true),
            textInput('executor_ref', '执行器引用', true),
            numberInput('runtime_required', '需要运行时', true),
            numberInput('default_timeout_sec', '默认等待秒数', true),
            textareaInput('resource_spec', '资源规格', true),
            numberInput('status', '状态', true),
          ], async (values) => {
            await api.judge.updateJudger(valueText(values, 'judger_id'), {
              code: valueText(values, 'code'),
              name: valueText(values, 'name'),
              type: valueNumber(values, 'type'),
              executor_ref: valueText(values, 'executor_ref'),
              runtime_required: valueFlag(values, 'runtime_required'),
              default_timeout_sec: valueNumber(values, 'default_timeout_sec'),
              resource_spec: valueJson(values, 'resource_spec'),
              status: valueNumber(values, 'status'),
            })
            return '判题器已更新'
          }),
        ],
        rowActions: [
          rowAction('judger-selftest', '自测', '触发判题器自测。', async (row) => {
            await api.judge.runJudgerSelftest(row.id)
            return '判题器自测已触发'
          }),
        ],
      }),
    },
    {
      path: 'sim-review',
      label: '仿真治理',
      description: '审核仿真包、查看静态扫描和确定性校验结果',
      icon: PackageCheck,
      group: '引擎',
      load: async (api) => ({
        ...listResult(await api.sim.getReviews(defaultPageParams()), simReviewColumns(), '暂无审核任务', '有仿真包提交后会在这里显示。'),
        actions: simGovernanceActions(api),
      }),
    },
    {
      path: 'vulnerability',
      label: '漏洞题源',
      description: '管理漏洞来源、同步和预验证草稿',
      icon: Flag,
      group: '引擎',
      load: async (api) => ({
        ...listResult(await api.contest.listVulnProblems(defaultPageParams()), vulnProblemColumns(), '暂无漏洞题草稿', '同步或导入漏洞题后会在这里显示。'),
        actions: [
          pageAction('upsert-vuln-source', '保存漏洞源', '创建或更新漏洞来源配置。', [
            numberInput('type', '来源类型', true),
            textInput('name', '来源名称', true),
            textareaInput('config', '来源配置', true),
            numberInput('default_level', '默认等级', true),
            numberInput('enabled', '是否启用', true),
          ], async (values) => {
            await api.contest.upsertVulnSource({
              type: valueNumber(values, 'type'),
              name: valueText(values, 'name'),
              config: valueJson(values, 'config'),
              default_level: valueNumber(values, 'default_level'),
              enabled: valueFlag(values, 'enabled'),
            })
            return '漏洞源已保存'
          }),
          pageAction('sync-vuln-source', '同步漏洞源', '从指定漏洞源同步案例。', [textInput('source_id', '来源编号', true)], async (values) => {
            await api.contest.syncVulnSource(valueText(values, 'source_id'))
            return '漏洞源同步已触发'
          }),
          pageAction('import-vuln-problem', '导入漏洞题', '导入漏洞题草稿并进入预验证流程。', [
            textInput('external_ref', '外部引用'),
            textInput('title', '标题', true),
            numberInput('level', '等级', true),
            numberInput('runtime_mode', '运行模式', true),
            textareaInput('draft_body', '草稿正文', true),
          ], async (values) => {
            await api.contest.importVulnProblem({
              external_ref: optionalText(values, 'external_ref'),
              title: valueText(values, 'title'),
              level: valueNumber(values, 'level'),
              runtime_mode: valueNumber(values, 'runtime_mode'),
              draft_body: valueJson(values, 'draft_body'),
            })
            return '漏洞题草稿已导入'
          }),
        ],
      }),
    },
    {
      path: 'alerts',
      label: '告警中心',
      description: '查看业务告警事件并完成处理',
      icon: ShieldAlert,
      group: '运维',
      load: async (api) => ({
        ...listResult(await api.admin.listAlertEvents(defaultPageParams()), alertColumns(), '暂无告警', '触发告警规则后会在这里显示。'),
        actions: [
          pageAction('create-alert-rule', '创建告警规则', '创建业务告警规则。', [
            numberInput('scope', '作用范围', true),
            textInput('name', '规则名称', true),
            textInput('metric', '指标', true),
            textareaInput('condition', '触发条件', true),
            numberInput('level', '级别', true),
            numberInput('enabled', '是否启用', true),
          ], async (values) => {
            await api.admin.createAlertRule({
              scope: valueNumber(values, 'scope'),
              name: valueText(values, 'name'),
              metric: valueText(values, 'metric'),
              condition: valueJson(values, 'condition'),
              level: valueNumber(values, 'level'),
              enabled: valueFlag(values, 'enabled'),
            })
            return '告警规则已创建'
          }),
          pageAction('handle-alert', '处理告警', '处理或忽略一条告警事件。', [
            textInput('event_id', '告警编号', true),
            numberInput('status', '处理状态', true),
          ], async (values) => {
            await api.admin.handleAlertEvent(valueText(values, 'event_id'), { status: valueNumber(values, 'status') })
            return '告警事件已处理'
          }),
        ],
      }),
    },
    {
      path: 'alert-rules',
      label: '告警规则',
      description: '维护平台和学校业务告警规则',
      icon: ShieldAlert,
      group: '运维',
      load: async (api) => ({
        ...arrayResult(await api.admin.listAlertRules(), alertRuleColumns(), '暂无告警规则', '创建规则后会在这里显示。'),
        actions: [
          pageAction('update-alert-rule', '更新告警规则', '更新业务告警规则。', [
            textInput('rule_id', '规则编号', true),
            numberInput('scope', '作用范围', true),
            textInput('name', '规则名称', true),
            textInput('metric', '指标', true),
            textareaInput('condition', '触发条件', true),
            numberInput('level', '级别', true),
            numberInput('enabled', '是否启用', true),
          ], async (values) => {
            await api.admin.updateAlertRule(valueText(values, 'rule_id'), {
              scope: valueNumber(values, 'scope'),
              name: valueText(values, 'name'),
              metric: valueText(values, 'metric'),
              condition: valueJson(values, 'condition'),
              level: valueNumber(values, 'level'),
              enabled: valueFlag(values, 'enabled'),
            })
            return '告警规则已更新'
          }),
        ],
      }),
    },
    ...platformEngineDeepRoutes(),
    {
      path: 'config',
      label: '系统配置',
      description: '查看和更新平台配置项',
      icon: Settings,
      group: '运维',
      load: async (api) => ({
        ...arrayResult(await api.admin.listConfigs(), configColumns(), '暂无配置', '创建或同步配置后会在这里显示。'),
        actions: [
          pageAction('update-config', '更新配置', '按配置 key 和版本号更新配置。', [
            textInput('key', '配置键', true),
            numberInput('scope', '作用范围', true),
            textareaInput('value', '配置值', true),
            numberInput('version', '当前版本', true),
            textInput('change_log_id', '变更记录编号'),
          ], async (values) => {
            await api.admin.updateConfig(valueText(values, 'key'), {
              scope: valueNumber(values, 'scope'),
              value: valueJson(values, 'value'),
              version: valueNumber(values, 'version'),
              change_log_id: optionalText(values, 'change_log_id'),
            })
            return '系统配置已更新'
          }),
          pageAction('read-config-history', '查看配置历史', '按配置键读取变更历史。', [textInput('key', '配置键', true)], async (values) => {
            await api.admin.listConfigHistory(valueText(values, 'key'), defaultPageParams())
            return '配置历史已读取'
          }),
          pageAction('rollback-config', '回滚配置', '按变更记录回滚配置值。', [
            textInput('key', '配置键', true),
            numberInput('scope', '作用范围', true),
            numberInput('version', '当前版本', true),
            textInput('change_log_id', '变更记录编号', true),
          ], async (values) => {
            await api.admin.rollbackConfig(valueText(values, 'key'), {
              scope: valueNumber(values, 'scope'),
              version: valueNumber(values, 'version'),
              change_log_id: valueText(values, 'change_log_id'),
            })
            return '系统配置已回滚'
          }),
        ],
      }),
    },
    {
      path: 'monitoring',
      label: '监控面板',
      description: '进入受控外部监控入口',
      icon: MonitorCog,
      group: '运维',
      load: async (api) => arrayResult(await api.admin.monitoringPanels(), monitoringColumns(), '暂无监控面板', '配置监控入口后会在这里显示。'),
    },
    {
      path: 'backups',
      label: '备份记录',
      description: '查看受控备份任务执行状态',
      icon: Boxes,
      group: '运维',
      load: async (api) => {
        const result = listResult(await api.admin.listBackups(defaultPageParams()), backupColumns(), '暂无备份记录', '备份任务执行后会在这里显示。')
        return { ...result, rows: result.rows.map(normalizeBackupSize) }
      },
    },
    {
      path: 'audit',
      label: '平台审计',
      description: '查询平台级敏感操作审计记录',
      icon: ListChecks,
      group: '运维',
      load: async (api) => listResult(await api.admin.queryAudit(defaultPageParams()), auditColumns(), '暂无审计记录', '用户完成敏感操作后会在这里显示。'),
    },
    sharedNotificationRoute(),
    sharedAnnouncementRoute(),
    sharedTransferRoute(),
    sharedProfileRoute(),
  ],
}
/**
 * platformTenantDeepRoutes 补齐平台租户详情和平台统计页。
 */
function platformTenantDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('tenant-detail', '学校详情', '查看租户配置、状态和资源配额', Landmark, async (api, params) => {
      const tenantId = routeParam(params, 'tenant_id', 'id')
      return tenantId
        ? objectResult(await api.identity.getTenant(tenantId), tenantColumns(), '学校详情')
        : arrayResult(await api.admin.listTenants(), tenantColumns(), '暂无学校租户', '学校入驻后会显示。')
    }),
    resourceRoute('statistics', '平台统计', '查看平台运营趋势和租户增长', BarChart3, async (api) => arrayResult(await api.admin.getPlatformStatistics(defaultRange()), statisticsColumns(), '暂无统计', '平台产生业务数据后会显示。'), '概览'),
  ]
}

/**
 * platformApplicationDeepRoutes 补齐入驻申请详情审核页。
 */
function platformApplicationDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('application-review', '入驻审核详情', '核对学校入驻资料并完成通过或驳回', ClipboardCheck, async (api) => ({
      ...arrayResult(await api.admin.listApplications(), applicationColumns(), '暂无入驻申请', '学校提交申请后会显示。'),
      actions: [
        pageAction('approve-application-detail', '通过申请', '通过入驻申请并创建租户。', [
          textInput('application_id', '申请编号', true),
          textInput('tenant_code', '租户编码', true),
          textInput('admin_name', '管理员姓名', true),
          textInput('admin_phone', '管理员手机号', true),
        ], async (values) => {
          await api.identity.approveApplication(valueText(values, 'application_id'), {
            tenant_code: valueText(values, 'tenant_code'),
            admin_name: valueText(values, 'admin_name'),
            admin_phone: valueText(values, 'admin_phone'),
          })
          return '入驻申请已通过'
        }),
        pageAction('reject-application-detail', '驳回申请', '驳回申请并写入原因。', [
          textInput('application_id', '申请编号', true),
          textareaInput('reason', '驳回原因', true),
        ], async (values) => {
          await api.identity.rejectApplication(valueText(values, 'application_id'), { reason: valueText(values, 'reason') })
          return '入驻申请已驳回'
        }),
      ],
    })),
  ]
}

/**
 * platformEngineDeepRoutes 补齐运行时详情和平台配额治理页。
 */
function platformEngineDeepRoutes(): AppDefinition['routes'] {
  return [
    hiddenResourceRoute('runtime-detail', '运行时详情', '查看运行时镜像版本、自测和预拉取状态', ServerCog, async (api, params) => {
      const runtimeId = routeParam(params, 'runtime_id', 'id')
      return {
        ...(runtimeId ? arrayResult(await api.sandbox.listRuntimeImages(runtimeId), runtimeImageColumns(), '暂无镜像版本', '登记镜像后会显示。') : arrayResult(await api.sandbox.listRuntimes(), runtimeColumns(), '暂无运行时', '登记运行时后会显示。')),
        actions: [
          pageAction('prepull-runtime-image', '预拉取镜像', '触发运行时镜像预拉取。', [
            textInput('runtime_id', '运行时编号', true),
            textInput('image_id', '镜像编号', true),
          ], async (values) => {
            await api.sandbox.prepullRuntimeImage(valueText(values, 'runtime_id'), valueText(values, 'image_id'))
            return '镜像预拉取已触发'
          }),
          pageAction('read-prepull-status', '查看预拉取状态', '查看运行时镜像在目标节点上的预拉取状态。', [
            textInput('runtime_id', '运行时编号', true),
            textInput('image_id', '镜像编号', true),
          ], async (values) => {
            await api.sandbox.getRuntimeImagePrepull(valueText(values, 'runtime_id'), valueText(values, 'image_id'))
            return '预拉取状态已读取'
          }),
          pageAction('disable-runtime-image', '停用镜像版本', '停用不再使用的运行时镜像版本。', [
            textInput('runtime_id', '运行时编号', true),
            textInput('image_id', '镜像编号', true),
          ], async (values) => {
            await api.sandbox.disableRuntimeImage(valueText(values, 'runtime_id'), valueText(values, 'image_id'))
            return '镜像版本已停用'
          }),
        ],
      }
    }),
    resourceRoute('quota', '配额管理', '查看并调整沙箱资源配额', Gauge, async (api) => ({
      ...objectResult(await api.sandbox.getQuota(), quotaColumns(), '沙箱配额'),
      actions: [
        pageAction('update-quota', '更新配额', '调整沙箱资源上限。', [
          numberInput('tenant_id', '学校编号', true),
          numberInput('max_concurrent_sandbox', '并发沙箱上限', true),
          numberInput('max_cpu', 'CPU 上限', true),
          numberInput('max_memory_mb', '内存上限', true),
          numberInput('idle_timeout_min', '空闲等待分钟', true),
          numberInput('max_lifetime_min', '最长运行分钟', true),
          numberInput('max_keepalive_min', '保活分钟', true),
          numberInput('max_snapshot_retention_min', '快照保留分钟', true),
        ], async (values) => {
          await api.sandbox.updateQuota({
            tenant_id: valueNumber(values, 'tenant_id'),
            active_sandbox_count: undefined,
            max_concurrent_sandbox: valueNumber(values, 'max_concurrent_sandbox'),
            max_cpu: valueNumber(values, 'max_cpu'),
            max_memory_mb: valueNumber(values, 'max_memory_mb'),
            idle_timeout_min: valueNumber(values, 'idle_timeout_min'),
            max_lifetime_min: valueNumber(values, 'max_lifetime_min'),
            max_keepalive_min: valueNumber(values, 'max_keepalive_min'),
            max_snapshot_retention_min: valueNumber(values, 'max_snapshot_retention_min'),
          })
          return '沙箱配额已更新'
        }),
      ],
    }), '引擎'),
  ]
}
