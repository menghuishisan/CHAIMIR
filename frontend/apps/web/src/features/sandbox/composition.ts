// features/sandbox/composition 提供组合声明的读写换算,实验、竞赛、题库与判题器共用一套。
//
// 「教师声明什么」与「服务端编译出什么」是两组字段:声明只有命名运行时实例、组件与连接,
// 编译结果里的镜像地址、digest、启动命令与安全上下文由服务端产出,前端只读
// (docs/对齐-后端待补齐清单-2026-08-23.md §6.3 / §7.5)。

import {
  COMPOSITION_SELECTION,
  type CompositionComponentRef,
  type CompositionLink,
  type CompositionPreviewRequest,
  type SandboxAccessProfile,
  type SandboxCompositionSpec,
  type ScenarioNeutralCompositionSpec,
} from '@chaimir/api-client'

/**
 * CompositionDeclaration 是编排表单里的可编辑部分。
 * 用编码数组而不是引用对象:表单只管「勾了哪些」,selection 标记由本模块统一补齐,
 * 不让每个页面各写一遍。
 */
export interface CompositionDeclaration {
	runtimes: RuntimeDeclaration[]
	workspaceRuntimeInstance: string
	toolCodes: string[]
	infraCodes: string[]
	links: CompositionLink[]
	componentParams?: Record<string, Record<string, unknown>>
	/** 仅存在于表单内存;绑定本次选择和连接,不进入保存请求。 */
	connectionCheck?: { key: string; complete: boolean }
}

/** RuntimeDeclaration 是一个可被连接图引用的运行时实例,别名在组合内必须唯一。 */
export interface RuntimeDeclaration {
	instanceCode: string
	runtimeCode: string
	imageVersion: string
	params?: Record<string, unknown>
	/** 仅供编辑时迁移材料引用,不提交为执行配置。 */
	originalInstanceCode?: string
	initAssetBound?: boolean
}

/** emptyCompositionDeclaration 给出新建时的空声明。 */
export function emptyCompositionDeclaration(): CompositionDeclaration {
	return { runtimes: [{ instanceCode: 'chain-1', runtimeCode: '', imageVersion: '' }], workspaceRuntimeInstance: 'chain-1', toolCodes: [], infraCodes: [], links: [] }
}

/** isExplicitComponent 判断组件引用是否为教师显式声明(而非编译器按依赖补齐)。 */
export function isExplicitComponent(ref: CompositionComponentRef): boolean {
  return ref.selection === COMPOSITION_SELECTION.EXPLICIT
}

/** explicitComponentRef 把勾选到的组件编码包成教师显式声明的引用。 */
export function explicitComponentRef(code: string): CompositionComponentRef {
  return { code, selection: COMPOSITION_SELECTION.EXPLICIT }
}

/**
 * declarationFromSpec 从已保存的组合声明里取出可编辑部分。
 * 编译器自动补齐的基础设施不进勾选框 —— 它不是教师的选择,重新保存时会再算一遍。
 */
export function declarationFromSpec(
	spec: Pick<SandboxCompositionSpec, 'runtimes' | 'workspace_runtime_instance' | 'tools' | 'infra' | 'links' | 'init_assets'> | undefined,
): CompositionDeclaration {
	if (spec === undefined) return emptyCompositionDeclaration()
	return {
		runtimes: (spec.runtimes ?? []).map((item) => ({
		instanceCode: item.instance_code,
		runtimeCode: item.runtime_code,
		imageVersion: item.image_version,
		params: item.params,
		originalInstanceCode: item.instance_code,
		initAssetBound: spec.init_assets?.some((asset) => asset.target_runtime_instance === item.instance_code),
	})),
		workspaceRuntimeInstance: spec.workspace_runtime_instance,
    toolCodes: (spec.tools ?? []).filter(isExplicitComponent).map((item) => item.code),
    infraCodes: (spec.infra ?? []).filter(isExplicitComponent).map((item) => item.code),
    links: spec.links ?? [],
    componentParams: Object.fromEntries([...(spec.infra ?? []), ...(spec.tools ?? [])]
      .filter((item) => isExplicitComponent(item) && item.params !== undefined)
      .map((item) => [item.code, item.params!])),
  }
}

/** compositionPreviewRequest 只提交显式选择,自动依赖由唯一编译器重新展开。 */
export function compositionPreviewRequest(declaration: CompositionDeclaration): CompositionPreviewRequest {
  const component = (code: string): CompositionComponentRef => ({ ...explicitComponentRef(code), params: declaration.componentParams?.[code] })
  return {
    runtimes: declaration.runtimes.map((item) => ({ instance_code: item.instanceCode, runtime_code: item.runtimeCode, image_version: item.imageVersion, params: item.params })),
    workspace_runtime_instance: declaration.workspaceRuntimeInstance,
    infra: declaration.infraCodes.map(component),
    tools: declaration.toolCodes.map(component),
    links: declaration.links,
  }
}

/** compositionSelectionKey 标识需重读目录的选择,不让预览返回的连接触发无限请求。 */
export function compositionSelectionKey(declaration: CompositionDeclaration): string {
  return JSON.stringify({ ...compositionPreviewRequest(declaration), links: undefined })
}

/** compositionConnectionKey 将预览完整性绑定到当前选择及明确连接。 */
export function compositionConnectionKey(declaration: CompositionDeclaration): string {
  return JSON.stringify(compositionPreviewRequest(declaration))
}

/** compositionSettings 只保留可编辑配置,不把冻结摘要或执行快照混回声明。 */
function compositionSettings(base: Partial<ScenarioNeutralCompositionSpec> | undefined, declaration: CompositionDeclaration) {
  return {
    resource_profile: base?.resource_profile,
    network_profile: base?.network_profile,
    init_assets: base?.init_assets?.map((asset) => ({ ...asset,
      target_runtime_instance: declaration.runtimes.find((runtime) => runtime.originalInstanceCode === asset.target_runtime_instance)?.instanceCode ?? asset.target_runtime_instance,
    })),
    init_code_ref: base?.init_code_ref,
    init_script_ref: base?.init_script_ref,
  }
}

export interface CompositionSpecInput {
  /** 组合标识:同一实验/题目内唯一,服务端按它区分多个环境 */
  id: string
  declaration: CompositionDeclaration
  accessProfile: SandboxAccessProfile
  /** 保留未编辑的初始化、资源与网络配置,选择字段始终由本次声明覆盖。 */
  base?: Partial<ScenarioNeutralCompositionSpec>
  initCodeRef?: string
  initScriptRef?: string
}

/**
 * compositionSpecFromDeclaration 把表单声明组装成后端组合声明。
 * 使用经过连接预览的显式声明,自动依赖不作为教师输入重提。
 */
export function compositionSpecFromDeclaration({
  id,
  declaration,
  accessProfile,
  base,
  initCodeRef,
  initScriptRef,
}: CompositionSpecInput): SandboxCompositionSpec {
	return {
    ...compositionSettings(base, declaration),
    ...compositionPreviewRequest(declaration),
		id,
    access_profile: accessProfile,
    ...(initCodeRef !== undefined && initCodeRef !== '' ? { init_code_ref: initCodeRef } : {}),
    ...(initScriptRef !== undefined && initScriptRef !== ''
      ? { init_script_ref: initScriptRef }
      : {}),
  }
}

/**
 * compositionDeclarationError 校验声明的必填项,返回用户向文案;通过时返回 undefined。
 * 每个 runtime 实例的别名、运行时和镜像版本都是后端硬要求:缺一个都编译不出可调度的环境。
 */
export function compositionDeclarationError(
  declaration: CompositionDeclaration,
): string | undefined {
  const selectionError = compositionSelectionError(declaration)
  if (selectionError !== undefined) return selectionError
  if (declaration.connectionCheck?.key !== compositionConnectionKey(declaration)) return '请等待组件连接检查完成,失败时可重新加载'
  if (!declaration.connectionCheck.complete) return '请完成组件连接选择后再保存'
  return undefined
}

/** compositionSelectionError 在发起连接预览前检查基本选择,不依赖预览结果。 */
export function compositionSelectionError(declaration: CompositionDeclaration): string | undefined {
	if (declaration.runtimes.length === 0) return '请至少添加一个运行时实例'
	if (declaration.workspaceRuntimeInstance.trim() === '') return '请选择工作区运行时实例'
	const aliases = new Set<string>()
	let workspaceRuntimeDeclared = false
	for (const runtime of declaration.runtimes) {
		if (runtime.instanceCode.trim() === '') return '请填写运行时实例名称'
		const instanceCode = runtime.instanceCode.trim()
		if (aliases.has(instanceCode)) return '运行时实例名称不能重复'
		aliases.add(instanceCode)
		if (instanceCode === declaration.workspaceRuntimeInstance.trim()) workspaceRuntimeDeclared = true
		if (runtime.runtimeCode === '') return '请选择运行时'
		if (runtime.imageVersion === '') return '请选择镜像版本,发布后按这个版本固定下来'
	}
	if (!workspaceRuntimeDeclared) return '工作区运行时实例必须来自已声明的运行时'
	return undefined
}

/**
 * scenarioNeutralSpecFromDeclaration 组装题库正文里的环境声明。
 * 不写 access_profile:同一道题进解题赛还是对抗赛决定访问边界,由服务端按场景写入。
 */
export function scenarioNeutralSpecFromDeclaration(
  id: string,
  declaration: CompositionDeclaration,
  base?: ScenarioNeutralCompositionSpec,
): ScenarioNeutralCompositionSpec {
	return {
    ...compositionSettings(base, declaration),
    ...compositionPreviewRequest(declaration),
		id,
  }
}

/** readScenarioNeutralSpec 从题库正文里读出环境声明;形状不符即回 undefined。 */
export function readScenarioNeutralSpec(
  body: Record<string, unknown>,
  key: string,
): ScenarioNeutralCompositionSpec | undefined {
  const raw = body[key]
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return undefined
  const record = raw as Record<string, unknown>
	const runtimes = record.runtimes
	if (!Array.isArray(runtimes) || runtimes.length === 0) return undefined
	if (typeof record.workspace_runtime_instance !== 'string' || record.workspace_runtime_instance === '') return undefined
	for (const item of runtimes) {
		if (typeof item !== 'object' || item === null) return undefined
		const runtime = item as Record<string, unknown>
		if (typeof runtime.instance_code !== 'string' || runtime.instance_code === '') return undefined
		if (typeof runtime.runtime_code !== 'string' || runtime.runtime_code === '') return undefined
		if (typeof runtime.image_version !== 'string' || runtime.image_version === '') return undefined
	}
  return record as unknown as ScenarioNeutralCompositionSpec
}
