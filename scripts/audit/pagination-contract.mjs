// 一次性审查脚本:核对后端 pagex 与前端 api-client 的公开分页默认值和上限。
// 用法:node scripts/audit/pagination-contract.mjs
import { readFileSync } from "node:fs";
import { join } from "node:path";

const ROOT = process.cwd();

// readNumericConstant 从指定源码中读取单个十进制常量,缺失时直接终止审查。
function readNumericConstant(source, pattern, label) {
  const match = pattern.exec(source);
  if (!match) {
    throw new Error(`未找到 ${label},请检查分页常量声明是否变更`);
  }
  return Number.parseInt(match[1], 10);
}

const backend = readFileSync(
  join(ROOT, "backend/internal/platform/pagex/pagex.go"),
  "utf8",
);
const frontend = readFileSync(
  join(ROOT, "frontend/packages/api-client/src/constants/pagination.ts"),
  "utf8",
);

const backendContract = {
  defaultSize: readNumericConstant(
    backend,
    /\bdefaultSize\s*=\s*(\d+)\b/,
    "backend pagex.defaultSize",
  ),
  maxSize: readNumericConstant(
    backend,
    /\bmaxSize\s*=\s*(\d+)\b/,
    "backend pagex.maxSize",
  ),
};
const frontendContract = {
  defaultSize: readNumericConstant(
    frontend,
    /\bPAGINATION_DEFAULT_SIZE\s*=\s*(\d+)\b/,
    "api-client PAGINATION_DEFAULT_SIZE",
  ),
  maxSize: readNumericConstant(
    frontend,
    /\bPAGINATION_MAX_SIZE\s*=\s*(\d+)\b/,
    "api-client PAGINATION_MAX_SIZE",
  ),
};

const mismatches = Object.keys(backendContract).filter(
  (key) => backendContract[key] !== frontendContract[key],
);

console.log(
  `分页契约:默认 ${frontendContract.defaultSize} 条,单页最多 ${frontendContract.maxSize} 条。`,
);
if (mismatches.length > 0) {
  for (const key of mismatches) {
    console.error(
      `${key} 不一致:backend=${backendContract[key]}, api-client=${frontendContract[key]}`,
    );
  }
  process.exitCode = 1;
} else {
  console.log("后端 pagex 与 api-client 分页常量一致。");
}
