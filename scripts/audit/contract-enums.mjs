// 跨语言枚举审查:只比较 HTTP 契约中的闭集 Go 常量与 TypeScript SDK 枚举,并阻止其退化为宽泛类型。
// 用法:node scripts/audit/contract-enums.mjs
// 数据库内部状态、服务间协议、前端展示选项和后端未封闭的数字字段不进入本清单。

import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

const ROOT = process.cwd();

/** read 读取仓库内文本文件。 */
function read(path) {
  return readFileSync(join(ROOT, path), "utf8");
}

/** screaming 把 Go 的 CamelCase 常量后缀转换成 TypeScript 枚举成员名。 */
function screaming(value) {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1_$2")
    .toUpperCase()
    .replace(/SAA_S/g, "SAAS");
}

/** goNumericConstants 提取单个 Go 文件里的数字常量。 */
function goNumericConstants(path) {
  const values = new Map();
  const pattern = /^\s*([A-Z][A-Za-z0-9_]*)\s+(?:int16\s+)?=\s*(-?\d+)\s*$/gm;
  for (const match of read(path).matchAll(pattern))
    values.set(match[1], Number(match[2]));
  return values;
}

/** tsNumericEnum 提取 TypeScript 数字枚举。 */
function tsNumericEnum(path, enumName) {
  const source = read(path);
  const block = source.match(
    new RegExp(`export enum ${enumName}\\s*\\{([\\s\\S]*?)\\n\\}`),
  );
  if (!block) return undefined;
  const values = new Map();
  for (const match of block[1].matchAll(
    /^\s*([A-Z][A-Z0-9_]*)\s*=\s*(-?\d+),?\s*$/gm,
  )) {
    values.set(match[1], Number(match[2]));
  }
  return values;
}

/** goStringConstants 提取 Go 文件中的字符串常量。 */
function goStringConstants(path) {
  const values = new Map();
  const pattern =
    /^\s*([A-Za-z][A-Za-z0-9_]*)\s+(?:[A-Za-z0-9_.]+\s+)?=\s*"([^"]*)"\s*$/gm;
  for (const match of read(path).matchAll(pattern))
    values.set(match[1], match[2]);
  return values;
}

/** tsStringConst 提取 TypeScript `as const` 对象中的字符串成员。 */
function tsStringConst(path, constName) {
  const source = read(path);
  const block = source.match(
    new RegExp(
      `export const ${constName}\\s*=\\s*\\{([\\s\\S]*?)\\}\\s*as const`,
    ),
  );
  if (!block) return undefined;
  const values = new Map();
  for (const match of block[1].matchAll(
    /^\s*([A-Z][A-Z0-9_]*)\s*:\s*'([^']*)',?\s*$/gm,
  )) {
    values.set(match[1], match[2]);
  }
  return values;
}

/** tsInterfaceStringUnion 提取接口字段上的字符串联合类型。 */
function tsInterfaceStringUnion(path, interfaceName, field) {
  const source = read(path);
  const block = source.match(
    new RegExp(`export interface ${interfaceName}\\s*\\{([\\s\\S]*?)\\n\\}`),
  );
  if (!block) return undefined;
  const fieldLine = block[1].match(
    new RegExp(`^\\s*${field}\\??:\\s*([^\\n]+)$`, "m"),
  );
  if (!fieldLine) return undefined;
  const literals = [...fieldLine[1].matchAll(/'([^']+)'/g)].map(
    (match) => match[1],
  );
  if (literals.length > 0) return new Set(literals);
  const alias = fieldLine[1].trim().replace(/\[\]$/, "");
  const aliasMatch = source.match(
    new RegExp(
      `export type ${alias}\\s*=\\s*\\(typeof ([A-Z][A-Z0-9_]*)\\)\\[keyof typeof \\1\\]`,
    ),
  );
  let values = aliasMatch ? tsStringConst(path, aliasMatch[1]) : undefined;
  if (!values) {
    const importMatch = source.match(
      new RegExp(`\\b${alias}\\b[\\s\\S]{0,300}?from ['\"]([^'\"]+)['\"]`),
    );
    if (importMatch) {
      const target = `${join(path.split('/').slice(0, -1).join('/'), importMatch[1])}.ts`;
      const targetSource = read(target);
      const targetAlias = targetSource.match(
        new RegExp(
          `export type ${alias}\\s*=\\s*\\(typeof ([A-Z][A-Z0-9_]*)\\)\\[keyof typeof \\1\\]`,
        ),
      );
      if (targetAlias) values = tsStringConst(target, targetAlias[1]);
    }
  }
  return values ? new Set(values.values()) : undefined;
}

/** goCaseStringMap 提取 `case NumericPrefixX: return "value"` 的公开字符串映射。 */
function goCaseStringMap(path, prefix) {
  const values = new Map();
  const pattern = new RegExp(
    `case\\s+${prefix}([A-Za-z0-9_]+):\\s*return\\s+"([^"]+)"`,
    "g",
  );
  for (const match of read(path).matchAll(pattern))
    values.set(screaming(match[1]), match[2]);
  return values;
}

/** compareGroup 比较一个后端常量前缀与一个前端枚举的完整成员和值。 */
function compareGroup(group) {
  const backend = new Map();
  for (const [name, value] of goNumericConstants(group.go)) {
    if (name.startsWith(group.prefix))
      backend.set(screaming(name.slice(group.prefix.length)), value);
  }
  const frontend = tsNumericEnum(group.ts, group.enum);
  if (backend.size === 0)
    return [
      `${group.enum}:后端 ${group.go} 缺少前缀 ${group.prefix} 的数字常量`,
    ];
  if (!frontend) return [`${group.enum}:前端 ${group.ts} 缺少该枚举`];

  const problems = [];
  for (const [name, value] of backend) {
    if (!frontend.has(name))
      problems.push(`前端缺少 ${group.enum}.${name}=${value}`);
    else if (frontend.get(name) !== value)
      problems.push(
        `${group.enum}.${name}:后端=${value},前端=${frontend.get(name)}`,
      );
  }
  for (const [name, value] of frontend) {
    if (!backend.has(name))
      problems.push(`后端缺少 ${group.prefix}${name}=${value}`);
  }
  return problems.map((item) => `${group.enum}: ${item}`);
}

/** prefixedGoStrings 返回指定前缀的 Go 字符串常量,可排除仅供后端表示默认态的成员。 */
function prefixedGoStrings(group) {
  const omitted = new Set(group.omit ?? []);
  const values = new Map();
  for (const [name, value] of goStringConstants(group.go)) {
    if (!name.startsWith(group.prefix)) continue;
    const member = screaming(name.slice(group.prefix.length));
    if (!omitted.has(member)) values.set(member, value);
  }
  return values;
}

/** compareStringGroup 比较 Go 字符串常量与 TypeScript `as const` 对象。 */
function compareStringGroup(group) {
  const backend = prefixedGoStrings(group);
  const frontend = tsStringConst(group.ts, group.constName);
  if (backend.size === 0)
    return [
      `${group.constName}:后端 ${group.go} 缺少前缀 ${group.prefix} 的字符串常量`,
    ];
  if (!frontend) return [`${group.constName}:前端 ${group.ts} 缺少该常量对象`];

  const problems = [];
  for (const [name, value] of backend) {
    if (!frontend.has(name))
      problems.push(`前端缺少 ${group.constName}.${name}='${value}'`);
    else if (frontend.get(name) !== value)
      problems.push(
        `${group.constName}.${name}:后端='${value}',前端='${frontend.get(name)}'`,
      );
  }
  for (const [name, value] of frontend) {
    if (!backend.has(name))
      problems.push(`后端缺少 ${group.prefix}${name}='${value}'`);
  }
  return problems.map((item) => `${group.constName}: ${item}`);
}

/** compareCaseStringGroup 比较 Go 数字状态转字符串的 switch 与 TypeScript 常量对象。 */
function compareCaseStringGroup(group) {
  const backend = goCaseStringMap(group.go, group.prefix);
  const frontend = tsStringConst(group.ts, group.constName);
  if (backend.size === 0)
    return [
      `${group.constName}:后端 ${group.go} 缺少 ${group.prefix}* 的字符串映射`,
    ];
  if (!frontend) return [`${group.constName}:前端 ${group.ts} 缺少该常量对象`];
  const problems = [];
  for (const [name, value] of backend) {
    if (!frontend.has(name))
      problems.push(`前端缺少 ${group.constName}.${name}='${value}'`);
    else if (frontend.get(name) !== value)
      problems.push(
        `${group.constName}.${name}:后端='${value}',前端='${frontend.get(name)}'`,
      );
  }
  for (const [name, value] of frontend) {
    if (!backend.has(name))
      problems.push(
        `后端 ${group.go} 缺少 ${group.prefix}${name} -> '${value}'`,
      );
  }
  return problems.map((item) => `${group.constName}: ${item}`);
}

/** compareAuditActorRole 比较由账号角色和系统角色共同组成的审计主体枚举。 */
function compareAuditActorRole() {
  const backend = new Map();
  for (const [name, value] of goNumericConstants(
    "backend/internal/contracts/roles.go",
  )) {
    if (name.startsWith("RoleNum"))
      backend.set(screaming(name.slice("RoleNum".length)), value);
  }
  const system = goNumericConstants(
    "backend/internal/platform/audit/actor_role.go",
  ).get("ActorRoleSystem");
  if (system !== undefined) backend.set("SYSTEM", system);
  const frontend = tsNumericEnum(
    "frontend/packages/api-client/src/constants/identity.ts",
    "AuditActorRole",
  );
  if (!frontend) return ["AuditActorRole:前端缺少该枚举"];
  const problems = [];
  for (const [name, value] of backend) {
    if (!frontend.has(name))
      problems.push(`前端缺少 AuditActorRole.${name}=${value}`);
    else if (frontend.get(name) !== value)
      problems.push(
        `AuditActorRole.${name}:后端=${value},前端=${frontend.get(name)}`,
      );
  }
  for (const [name, value] of frontend) {
    if (!backend.has(name)) problems.push(`后端缺少审计角色 ${name}=${value}`);
  }
  return problems;
}

/** compareStringUnionGroup 比较 Go 字符串常量值与公开接口字段的字符串联合类型。 */
function compareStringUnionGroup(group) {
  const backend = new Set(prefixedGoStrings(group).values());
  const frontend = tsInterfaceStringUnion(
    group.ts,
    group.interfaceName,
    group.field,
  );
  const label = `${group.interfaceName}.${group.field}`;
  if (backend.size === 0)
    return [`${label}:后端 ${group.go} 缺少前缀 ${group.prefix} 的字符串常量`];
  if (!frontend) return [`${label}:前端 ${group.ts} 缺少字符串联合类型`];
  const problems = [];
  for (const value of backend)
    if (!frontend.has(value)) problems.push(`前端缺少 '${value}'`);
  for (const value of frontend)
    if (!backend.has(value)) problems.push(`后端缺少 '${value}'`);
  return problems.map((item) => `${label}: ${item}`);
}

const groups = [
  [
    "backend/internal/contracts/roles.go",
    "RoleNum",
    "frontend/packages/api-client/src/constants/identity.ts",
    "UserRole",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "TenantStatus",
    "frontend/packages/api-client/src/constants/identity.ts",
    "TenantStatus",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "DeployMode",
    "frontend/packages/api-client/src/constants/identity.ts",
    "DeployMode",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "AuthMode",
    "frontend/packages/api-client/src/constants/identity.ts",
    "AuthMode",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "ApplicationStatus",
    "frontend/packages/api-client/src/constants/identity.ts",
    "ApplicationStatus",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "BaseIdentity",
    "frontend/packages/api-client/src/constants/identity.ts",
    "BaseIdentity",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "AccountStatus",
    "frontend/packages/api-client/src/constants/identity.ts",
    "AccountStatus",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "SessionStatus",
    "frontend/packages/api-client/src/constants/identity.ts",
    "SessionStatus",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "SMSScene",
    "frontend/packages/api-client/src/constants/identity.ts",
    "SmsScene",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "SSOType",
    "frontend/packages/api-client/src/constants/identity.ts",
    "SsoType",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "SSOMatch",
    "frontend/packages/api-client/src/constants/identity.ts",
    "SsoMatchField",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "ImportTarget",
    "frontend/packages/api-client/src/constants/identity.ts",
    "ImportTarget",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "ImportBatch",
    "frontend/packages/api-client/src/constants/identity.ts",
    "ImportBatchStatus",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "ClassStatus",
    "frontend/packages/api-client/src/constants/identity.ts",
    "ClassStatus",
  ],

  [
    "backend/internal/modules/content/enum.go",
    "Type",
    "frontend/packages/api-client/src/constants/content.ts",
    "ContentType",
  ],
  [
    "backend/internal/modules/content/enum.go",
    "Difficulty",
    "frontend/packages/api-client/src/constants/content.ts",
    "ContentDifficulty",
  ],
  [
    "backend/internal/modules/content/enum.go",
    "Author",
    "frontend/packages/api-client/src/constants/content.ts",
    "ContentAuthorType",
  ],
  [
    "backend/internal/modules/content/enum.go",
    "Visibility",
    "frontend/packages/api-client/src/constants/content.ts",
    "ContentVisibility",
  ],
  [
    "backend/internal/modules/content/enum.go",
    "Status",
    "frontend/packages/api-client/src/constants/content.ts",
    "ContentStatus",
  ],
  [
    "backend/internal/modules/content/enum.go",
    "PaperMode",
    "frontend/packages/api-client/src/constants/content.ts",
    "PaperMode",
  ],

  [
    "backend/internal/modules/experiment/enum.go",
    "CollabMode",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "ExperimentCollabMode",
  ],
  [
    "backend/internal/modules/experiment/enum.go",
    "ExperimentStatus",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "ExperimentStatus",
  ],
  [
    "backend/internal/modules/experiment/enum.go",
    "InstanceStatus",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "ExperimentInstanceStatus",
  ],
  [
    "backend/internal/modules/experiment/enum.go",
    "ReportStatus",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "ExperimentReportStatus",
  ],

  [
    "backend/internal/modules/teaching/enum.go",
    "CourseType",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "CourseType",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "Difficulty",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "TeachingDifficulty",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "CourseStatus",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "CourseStatus",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "CourseVisibility",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "CourseVisibility",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "LessonContent",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "LessonContentType",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "JoinMode",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "JoinMode",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "AssignmentStatus",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "AssignmentStatus",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "LatePolicy",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "LatePolicy",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "GradingMode",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "GradingMode",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "SubmissionStatus",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "SubmissionStatus",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "Progress",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "ProgressStatus",
  ],
  [
    "backend/internal/modules/teaching/enum.go",
    "GradeSource",
    "frontend/packages/api-client/src/constants/teaching.ts",
    "GradeSource",
  ],

  [
    "backend/internal/contracts/engine_sandbox.go",
    "SandboxPhase",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "SandboxPhase",
  ],
  [
    "backend/internal/contracts/engine_sandbox.go",
    "SandboxStatus",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "SandboxStatus",
  ],
  [
    "backend/internal/contracts/engine_sandbox.go",
    "SandboxToolKind",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "SandboxToolKind",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "RuntimeAdapterLevel",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "RuntimeAdapterLevel",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "RuntimeStatus",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "RuntimeStatus",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "RuntimeSelftest",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "RuntimeSelftestStatus",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "RuntimeImageStatus",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "RuntimeImageStatus",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "ImagePrepull",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "ImagePrepullStatus",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "ToolStatus",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "ToolStatus",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "SandboxToolStatus",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "SandboxToolStatus",
  ],

  [
    "backend/internal/modules/judge/enum.go",
    "JudgerType",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JudgerType",
  ],
  [
    "backend/internal/modules/judge/enum.go",
    "JudgerSelftest",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JudgerSelftestStatus",
  ],
  [
    "backend/internal/modules/judge/enum.go",
    "JudgerStatus",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JudgerStatus",
  ],

  [
    "backend/internal/modules/contest/enum.go",
    "ContestMode",
    "frontend/packages/api-client/src/constants/contest.ts",
    "ContestMode",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "MatchMode",
    "frontend/packages/api-client/src/constants/contest.ts",
    "MatchMode",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "TeamMode",
    "frontend/packages/api-client/src/constants/contest.ts",
    "TeamMode",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "ContestStatus",
    "frontend/packages/api-client/src/constants/contest.ts",
    "ContestStatus",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "TeamStatus",
    "frontend/packages/api-client/src/constants/contest.ts",
    "TeamStatus",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "BattleRule",
    "frontend/packages/api-client/src/constants/contest.ts",
    "BattleRule",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "BattleRole",
    "frontend/packages/api-client/src/constants/contest.ts",
    "BattleRole",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "BattleMatchStatus",
    "frontend/packages/api-client/src/constants/contest.ts",
    "BattleMatchStatus",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "BattleResult",
    "frontend/packages/api-client/src/constants/contest.ts",
    "BattleResult",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "CheatType",
    "frontend/packages/api-client/src/constants/contest.ts",
    "CheatType",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "CheatAction",
    "frontend/packages/api-client/src/constants/contest.ts",
    "CheatAction",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnSourceType",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VulnSourceType",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnLevel",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VulnLevel",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnRuntime",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VulnRuntimeMode",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnPrevalidate",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VulnPrevalidateStatus",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnProblemStatus",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VulnProblemStatus",
  ],

  [
    "backend/internal/modules/grade/enum.go",
    "ReviewStatus",
    "frontend/packages/api-client/src/constants/grade.ts",
    "GradeReviewStatus",
  ],
  [
    "backend/internal/modules/grade/enum.go",
    "AppealStatus",
    "frontend/packages/api-client/src/constants/grade.ts",
    "GradeAppealStatus",
  ],
  [
    "backend/internal/modules/grade/enum.go",
    "WarningType",
    "frontend/packages/api-client/src/constants/grade.ts",
    "GradeWarningType",
  ],
  [
    "backend/internal/modules/grade/enum.go",
    "WarningStatus",
    "frontend/packages/api-client/src/constants/grade.ts",
    "GradeWarningStatus",
  ],
  [
    "backend/internal/modules/grade/enum.go",
    "TranscriptScope",
    "frontend/packages/api-client/src/constants/grade.ts",
    "TranscriptScope",
  ],

  [
    "backend/internal/modules/admin/enum.go",
    "Scope",
    "frontend/packages/api-client/src/constants/admin.ts",
    "AdminScope",
  ],
  [
    "backend/internal/modules/admin/enum.go",
    "AlertLevel",
    "frontend/packages/api-client/src/constants/admin.ts",
    "AlertLevel",
  ],
  [
    "backend/internal/modules/admin/enum.go",
    "AlertStatus",
    "frontend/packages/api-client/src/constants/admin.ts",
    "AlertStatus",
  ],
  [
    "backend/internal/modules/admin/enum.go",
    "BackupType",
    "frontend/packages/api-client/src/constants/admin.ts",
    "BackupType",
  ],
  [
    "backend/internal/modules/admin/enum.go",
    "BackupStatus",
    "frontend/packages/api-client/src/constants/admin.ts",
    "BackupStatus",
  ],
  [
    "backend/internal/modules/notify/enum.go",
    "AnnouncementScope",
    "frontend/packages/api-client/src/constants/notify.ts",
    "AnnouncementScope",
  ],
].map(([go, prefix, ts, enumName]) => ({ go, prefix, ts, enum: enumName }));

const stringGroups = [
  [
    "backend/internal/modules/identity/service_import.go",
    "importTemplateFormat",
    "frontend/packages/api-client/src/constants/identity.ts",
    "IMPORT_TEMPLATE_FORMAT",
  ],
  [
    "backend/internal/modules/identity/enum.go",
    "TenantModule",
    "frontend/packages/api-client/src/constants/identity.ts",
    "TENANT_MODULE",
  ],
  [
    "backend/internal/contracts/engine_judge.go",
    "JudgeSandboxMode",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JUDGE_SANDBOX_MODE",
  ],
  [
    "backend/internal/modules/experiment/enum.go",
    "ValidationLevel",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "EXPERIMENT_VALIDATION_LEVEL",
  ],
  [
    "backend/internal/modules/experiment/service_stage.go",
    "stageStatus",
    "frontend/packages/api-client/src/constants/experiment.ts",
    "EXPERIMENT_STAGE_STATUS",
  ],
  [
    "backend/internal/modules/judge/enum.go",
    "TaskState",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JUDGE_TASK_STATE",
    ["ALL"],
  ],
  [
    "backend/internal/modules/sim/enum.go",
    "validation",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_VALIDATION_STATUS",
  ],
  [
    "backend/internal/modules/sim/enum.go",
    "backendStream",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_STREAM_FRAME",
  ],
  [
    "backend/internal/modules/sim/enum.go",
    "BackendCommand",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_STREAM_COMMAND",
  ],
  [
    "backend/internal/platform/transfer/transfer.go",
    "Status",
    "frontend/packages/api-client/src/constants/transfer.ts",
    "TRANSFER_STATUS",
  ],
  [
    "backend/internal/platform/transfer/transfer.go",
    "Channel",
    "frontend/packages/api-client/src/constants/transfer.ts",
    "TRANSFER_CHANNEL",
  ],
  [
    "backend/internal/modules/sandbox/enum.go",
    "ChainOperation",
    "frontend/packages/api-client/src/constants/sandbox.ts",
    "SANDBOX_CHAIN_OPERATION",
  ],
  [
    "backend/internal/modules/contest/enum.go",
    "VulnChainOperation",
    "frontend/packages/api-client/src/constants/contest.ts",
    "VULN_CHAIN_OPERATION",
  ],
  [
    "backend/pkg/chainassert/chainassert.go",
    "Operation",
    "frontend/packages/api-client/src/constants/contest.ts",
    "CHAIN_ASSERT_OPERATION",
  ],
  [
    "backend/internal/contracts/engine_composition.go",
    "SandboxAccess",
    "frontend/packages/api-client/src/constants/composition.ts",
    "SANDBOX_ACCESS_PROFILE",
  ],
].map(([go, prefix, ts, constName, omit]) => ({
  go,
  prefix,
  ts,
  constName,
  omit,
}));

const caseStringGroups = [
  [
    "backend/internal/modules/judge/rules.go",
    "JudgeTaskStatus",
    "frontend/packages/api-client/src/constants/judge.ts",
    "JUDGE_TASK_STATUS",
  ],
  [
    "backend/internal/modules/sim/rules.go",
    "Compute",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_COMPUTE",
  ],
  [
    "backend/internal/modules/sim/convert.go",
    "PackageStatus",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_PACKAGE_STATUS",
  ],
  [
    "backend/internal/modules/sim/convert.go",
    "Review",
    "frontend/packages/api-client/src/constants/sim.ts",
    "SIM_REVIEW_RESULT",
  ],
].map(([go, prefix, ts, constName]) => ({ go, prefix, ts, constName }));

const problems = [
  ...groups.flatMap(compareGroup),
  ...compareAuditActorRole(),
  ...stringGroups.flatMap(compareStringGroup),
  ...caseStringGroups.flatMap(compareCaseStringGroup),
];

// 每个前端数字枚举都必须有后端对照登记,防止新增枚举绕过本审查。
const coveredNumericEnums = new Set(
  groups.map((group) => `${group.ts}:${group.enum}`),
);
coveredNumericEnums.add(
  "frontend/packages/api-client/src/constants/identity.ts:AuditActorRole",
);
for (const fileName of readdirSync(
  join(ROOT, "frontend/packages/api-client/src/constants"),
)) {
  if (!fileName.endsWith(".ts")) continue;
  const path = `frontend/packages/api-client/src/constants/${fileName}`;
  for (const match of read(path).matchAll(
    /^export enum ([A-Za-z][A-Za-z0-9_]*)/gm,
  )) {
    if (!coveredNumericEnums.has(`${path}:${match[1]}`))
      problems.push(`${path}: 数字枚举 ${match[1]} 未登记后端对照`);
  }
}

// HTTP 传输层文案是前端专属;API_ERROR_CODES 是前端实际交互分支的最小白名单,
// 不复制后端全部错误码。除此之外,API Client 的字符串常量对象都必须登记后端来源。
const frontendOnlyStringConsts = new Set([
  "API_ERROR_CODES",
  "API_TRANSPORT_ERROR_MESSAGES",
  // BOOL_FILTER 是查询串上的三态编码(0 不限 / 1 是 / 2 否),对应后端
  // httpx.QueryInt16(Min:0, Max:2) 的取值域而不是某个 Go 常量组,故按前端边界登记。
  "BOOL_FILTER",
]);

// 后端目前用裸字符串字面量表达、尚未命名成 Go 常量组的契约值:
// 本脚本无法做逐项对照,故单独登记并在 docs/对齐-后端待补齐清单-2026-08-23.md 记为后端待办 ——
// 后端把它们提成命名常量后,应移入 stringGroups 做真实对照,不留在这里。
const pendingBackendNamedConsts = new Set([
  // sandbox 组合编译器按 "tool" / "infra" 判定组件类别(composition.go expectedCategory)
  "SANDBOX_COMPONENT_CATEGORY",
  // 组合组件引用来源按 "explicit" / "auto" 判定(composition.go selection 校验)
  "COMPOSITION_SELECTION",
]);
const coveredStringConsts = new Set([
  ...stringGroups.map((group) => `${group.ts}:${group.constName}`),
  ...caseStringGroups.map((group) => `${group.ts}:${group.constName}`),
]);
for (const fileName of readdirSync(
  join(ROOT, "frontend/packages/api-client/src/constants"),
)) {
  if (!fileName.endsWith(".ts")) continue;
  const path = `frontend/packages/api-client/src/constants/${fileName}`;
  for (const match of read(path).matchAll(
    /^export const ([A-Z][A-Z0-9_]*)\s*=\s*\{/gm,
  )) {
    if (
      !coveredStringConsts.has(`${path}:${match[1]}`) &&
      !frontendOnlyStringConsts.has(match[1]) &&
      !pendingBackendNamedConsts.has(match[1])
    ) {
      problems.push(
        `${path}: 字符串常量 ${match[1]} 未登记后端来源或前端专属边界`,
      );
    }
  }
}

// 枚举语义字段默认不得退化为 number。学校类型是后端明确未封闭的数字域,
// 申请表单的 1-3 只是前端选项而非全平台枚举,故作为唯一白名单保留。
const openNumericFields = new Set([
  "identity.ts:school_type",
  "identity.ts:type",
]);
const enumLikeNumber =
  /^\s*([A-Za-z][A-Za-z0-9_]*(?:status|type|mode|scope|role|kind|level|phase)|status|type|mode|scope|role|kind|level|phase)\??:\s*number\b/gm;
for (const fileName of readdirSync(
  join(ROOT, "frontend/packages/api-client/src/types"),
)) {
  if (!fileName.endsWith(".ts")) continue;
  for (const match of read(
    `frontend/packages/api-client/src/types/${fileName}`,
  ).matchAll(enumLikeNumber)) {
    if (!openNumericFields.has(`${fileName}:${match[1]}`)) {
      problems.push(
        `frontend/packages/api-client/src/types/${fileName}: ${match[1]} 必须使用已登记的契约枚举`,
      );
    }
  }
}

const forbidden = [
  [
    "frontend/packages/api-client/src/types/sim.ts",
    /status\?:\s*string/,
    "仿真校验 status 必须使用 SimValidationStatusValue",
  ],
  [
    "frontend/packages/api-client/src/types/experiment.ts",
    /mode\?:\s*string/,
    "检查点 mode 必须使用 JudgeSandboxMode",
  ],
  [
    "frontend/packages/api-client/src/types/experiment.ts",
    /level:\s*string/,
    "实验校验 level 必须使用 ExperimentValidationLevel",
  ],
  [
    "frontend/apps/web/src/features/identity/pages/platform-admin/school-detail.tsx",
    /tenant\.status\s*===\s*1/,
    "租户状态判断禁止魔法值 1",
  ],
  [
    "frontend/apps/web/src/features/sandbox/pages/platform-admin/runtime-detail.tsx",
    /runtime\.status\s*===\s*1/,
    "运行时状态判断禁止魔法值 1",
  ],
  [
    "frontend/apps/web/src/features/experiment/pages/student/workspace.tsx",
    /stage\.status\s*===\s*'active'/,
    "实验阶段判断必须使用 EXPERIMENT_STAGE_STATUS",
  ],
  [
    "frontend/apps/web/src/features/experiment/pages/teacher/experiments.tsx",
    /issue\.level\s*===\s*'error'/,
    "实验校验判断必须使用 EXPERIMENT_VALIDATION_LEVEL",
  ],
  [
    "frontend/apps/web/src/features/experiment/pages/teacher/experiment-wizard.tsx",
    /issue\.level\s*===\s*'error'/,
    "实验校验判断必须使用 EXPERIMENT_VALIDATION_LEVEL",
  ],
  [
    "frontend/apps/web/src/features/sim/pages/platform-admin/simulations.tsx",
    /status\s*===\s*'passed'/,
    "仿真校验判断必须使用 SIM_VALIDATION_STATUS",
  ],
  [
    "frontend/apps/web/src/features/sim/pages/teacher/package-preview.tsx",
    /status\s*===\s*'(?:passed|failed)'/,
    "仿真校验判断必须使用 SIM_VALIDATION_STATUS",
  ],
];

for (const [path, pattern, message] of forbidden) {
  if (pattern.test(read(path))) problems.push(`${path}: ${message}`);
}

if (problems.length > 0) {
  console.error(`前后端枚举契约审查失败(${problems.length}):`);
  for (const problem of problems) console.error(`- ${problem}`);
  process.exit(1);
}

console.log(
  `前后端枚举契约审查通过:${groups.length + 1} 组数字枚举、${stringGroups.length + caseStringGroups.length} 组字符串协议,公开字段未退化为宽泛类型。`,
);
