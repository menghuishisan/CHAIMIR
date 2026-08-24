// features/sandbox/composition 提供组合声明的读写换算,实验、竞赛、题库与判题器共用一套。
//
// 「教师声明什么」与「服务端编译出什么」是两组字段:声明只有命名运行时实例、组件与连接,
// 编译结果里的镜像地址、digest、启动命令与安全上下文由服务端产出,前端只读
// (docs/对齐-后端待补齐清单-2026-08-23.md §6.3 / §7.5)。

import {
  COMPOSITION_SELECTION,
  type CompositionComponentRef,
  type CompositionLink,
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
}

/** RuntimeDeclaration 是一个可被连接图引用的运行时实例,别名在组合内必须唯一。 */
export interface RuntimeDeclaration {
	instanceCode: string
	runtimeCode: string
	imageVersion: string
}

/** emptyCompositionDeclaration 给出新建时的空声明。 */
export function emptyCompositionDeclaration(): CompositionDeclaration {
	return { runtimes: [{ instanceCode: 'chain-1', runtimeCode: '', imageVersion: '' }], workspaceRuntimeInstance: 'chain-1', toolCodes: [], infraCodes: [] }
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
	spec: Pick<SandboxCompositionSpec, 'runtimes' | 'workspace_runtime_instance' | 'tools' | 'infra'> | undefined,
): CompositionDeclaration {
	if (spec === undefined) return emptyCompositionDeclaration()
	return {
		runtimes: (spec.runtimes ?? []).map((item) => ({
		instanceCode: item.instance_code,
		runtimeCode: item.runtime_code,
		imageVersion: item.image_version,
	})),
		workspaceRuntimeInstance: spec.workspace_runtime_instance,
    toolCodes: (spec.tools ?? []).filter(isExplicitComponent).map((item) => item.code),
    infraCodes: (spec.infra ?? []).filter(isExplicitComponent).map((item) => item.code),
  }
}

/** derivedInfraFromSpec 取出编译器自动补齐的基础设施,页面只读展示并原样带回。 */
export function derivedInfraFromSpec(
  spec: Pick<SandboxCompositionSpec, 'infra'> | undefined,
): CompositionComponentRef[] {
  return (spec?.infra ?? []).filter((item) => !isExplicitComponent(item))
}

export interface CompositionSpecInput {
  /** 组合标识:同一实验/题目内唯一,服务端按它区分多个环境 */
  id: string
  declaration: CompositionDeclaration
  accessProfile: SandboxAccessProfile
  /** 上次编译补齐的基础设施,原样带回不改写 */
  derivedInfra?: CompositionComponentRef[]
  /** 上次编译冻结的连接,原样带回不改写 */
  links?: CompositionLink[]
  initCodeRef?: string
  initScriptRef?: string
}

/**
 * compositionSpecFromDeclaration 把表单声明组装成后端组合声明。
 * 连接与自动补齐的基础设施原样带回:它们由服务端编译器产出,前端不凭猜测新建或改写。
 */
export function compositionSpecFromDeclaration({
  id,
  declaration,
  accessProfile,
  derivedInfra = [],
  links = [],
  initCodeRef,
  initScriptRef,
}: CompositionSpecInput): SandboxCompositionSpec {
	return {
		id,
		runtimes: declaration.runtimes.map((item) => ({
			instance_code: item.instanceCode,
			runtime_code: item.runtimeCode,
			image_version: item.imageVersion,
		})),
		workspace_runtime_instance: declaration.workspaceRuntimeInstance,
    infra: [...declaration.infraCodes.map(explicitComponentRef), ...derivedInfra],
    tools: declaration.toolCodes.map(explicitComponentRef),
    links,
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
  derivedInfra: CompositionComponentRef[] = [],
  links: CompositionLink[] = [],
): ScenarioNeutralCompositionSpec {
	return {
		id,
		runtimes: declaration.runtimes.map((item) => ({
			instance_code: item.instanceCode,
			runtime_code: item.runtimeCode,
			image_version: item.imageVersion,
		})),
		workspace_runtime_instance: declaration.workspaceRuntimeInstance,
    infra: [...declaration.infraCodes.map(explicitComponentRef), ...derivedInfra],
    tools: declaration.toolCodes.map(explicitComponentRef),
    links,
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
