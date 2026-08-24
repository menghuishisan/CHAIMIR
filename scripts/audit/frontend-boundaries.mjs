/**
 * 前端部署边界静态门禁:校验公共 origin、工具 iframe origin、CAS 回调白名单、
 * Ingress base、运行时配置初始化和前端安全响应头是否仍保持单一契约。
 * 真实 TLS 响应头和登录态浏览器验收仍需在运行中的部署环境执行。
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const PUBLIC_HOST = "www.chaimir.io";
const TOOL_ORIGIN = "https://tools.chaimir.io";
const failures = [];

/** read 读取仓库文件,缺失时记录失败而不是静默跳过。 */
function read(relativePath) {
  const absolutePath = path.join(ROOT, relativePath);
  if (!fs.existsSync(absolutePath)) {
    failures.push(`${relativePath}: 文件不存在`);
    return "";
  }
  return fs.readFileSync(absolutePath, "utf8");
}

/** requireMatch 校验文件必须满足指定契约。 */
function requireMatch(relativePath, source, pattern, message) {
  if (!pattern.test(source)) failures.push(`${relativePath}: ${message}`);
}

/** parseEnv 解析受控环境文件,只用于比较键和值是否漂移,不会输出敏感值。 */
function parseEnv(source) {
  const values = new Map();
  for (const rawLine of source.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === "" || line.startsWith("#")) continue;
    const separator = line.indexOf("=");
    if (separator <= 0) continue;
    values.set(
      line.slice(0, separator).trim(),
      line.slice(separator + 1).trim(),
    );
  }
  return values;
}

/** escapeRegex 将固定配置值转为可安全拼接的正则片段。 */
function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

for (const relativePath of ["frontend/.env", "frontend/.env.example"]) {
  const source = read(relativePath);
  requireMatch(
    relativePath,
    source,
    /^VITE_API_BASE_URL=\/api\/v1\s*$/m,
    "API 必须使用同源 /api/v1",
  );
  requireMatch(
    relativePath,
    source,
    /^VITE_WS_BASE_URL=\s*$/m,
    "WebSocket 默认必须使用同源票据流",
  );
  if (/^VITE_(?:SANDBOX_TOOL_ORIGIN|DEPLOY_MODE)=/m.test(source)) {
    failures.push(
      `${relativePath}: 部署形态与工具 origin 必须由初始化容器生成 /runtime-config.js,不能写入 Vite 构建环境`,
    );
  }
}

const deployConfigPath = "deploy/config/chaimir.env";
const deployConfigSource = read(deployConfigPath);
requireMatch(
  deployConfigPath,
  deployConfigSource,
  /^DEPLOY_MODE=saas\s*$/m,
  "默认部署形态必须为 saas",
);
requireMatch(
  deployConfigPath,
  deployConfigSource,
  new RegExp(`^SANDBOX_TOOL_ORIGIN=${escapeRegex(TOOL_ORIGIN)}\\s*$`, "m"),
  `工具 origin 必须为 ${TOOL_ORIGIN}`,
);

const runtimeScriptPath = "images/service/frontend/runtime-config.sh";
const runtimeScript = read(runtimeScriptPath);
for (const [pattern, message] of [
  [/case "\$\{DEPLOY_MODE:-\}"/, "必须校验 DEPLOY_MODE"],
  [/saas\|school/, "必须只允许 saas 或 school"],
  [/origin="\$\{SANDBOX_TOOL_ORIGIN:-\}"/, "必须读取 SANDBOX_TOOL_ORIGIN"],
  [/https:\/\/\*/, "工具 origin 必须只允许 HTTPS"],
  [/window\.__CHAIMIR_RUNTIME_CONFIG__/, "必须生成浏览器运行时配置对象"],
  [/> \/runtime-config\/runtime-config\.js/, "必须写入挂载的运行时配置卷"],
])
  requireMatch(runtimeScriptPath, runtimeScript, pattern, message);

const frontendDeploymentPath = "deploy/base/frontend/deployment.yaml";
const frontendDeployment = read(frontendDeploymentPath);
for (const [pattern, message] of [
  [
    /- name: runtime-config[\s\S]*command: \["\/usr\/local\/bin\/chaimir-runtime-config"\]/,
    "必须由前端镜像初始化容器生成运行时配置",
  ],
  [/envFrom:[\s\S]*name: chaimir-config/, "初始化容器必须读取统一 ConfigMap"],
  [
    /mountPath: \/usr\/share\/nginx\/html\/runtime-config\.js[\s\S]*subPath: runtime-config\.js/,
    "Nginx 必须挂载初始化结果",
  ],
  [
    /- name: frontend-runtime-config\s*\n\s*emptyDir: \{\}/,
    "运行时配置必须使用 Pod 内临时卷交接",
  ],
])
  requireMatch(frontendDeploymentPath, frontendDeployment, pattern, message);

for (const name of ["acceptance", "staging", "prod-saas", "prod-school"]) {
  const stalePath = path.join(
    ROOT,
    `deploy/overlays/${name}/frontend-runtime-config.js`,
  );
  if (fs.existsSync(stalePath))
    failures.push(
      `deploy/overlays/${name}/frontend-runtime-config.js: 不得恢复 overlay 专用运行时配置`,
    );
}

const schoolOverlayPath = "deploy/overlays/prod-school/kustomization.yaml";
const schoolOverlay = read(schoolOverlayPath);
requireMatch(
  schoolOverlayPath,
  schoolOverlay,
  /^\s*- DEPLOY_MODE=school\s*$/m,
  "私有化形态必须覆盖为 school",
);
requireMatch(
  schoolOverlayPath,
  schoolOverlay,
  /^\s*- PLATFORM_LAYER_ENABLED=false\s*$/m,
  "私有化形态必须关闭平台层",
);

for (const relativePath of ["backend/.env.example", deployConfigPath]) {
  const source = read(relativePath);
  requireMatch(
    relativePath,
    source,
    new RegExp(
      `^IDENTITY_SSO_ALLOWED_SERVICE_ORIGINS=https://${escapeRegex(PUBLIC_HOST)}\\s*$`,
      "m",
    ),
    `CAS service origin 默认必须为 https://${PUBLIC_HOST}`,
  );
}

// deploy/config 是 Kubernetes 运行时权威源;后端示例只登记后端实际读取的同名非密配置。
const deployConfig = parseEnv(deployConfigSource);
const backendExample = parseEnv(read("backend/.env.example"));
for (const [key, value] of backendExample) {
  if (!deployConfig.has(key)) {
    failures.push(
      `backend/.env.example: ${key} 未在 deploy/config/chaimir.env 定义`,
    );
  } else if (
    key !== "PLATFORM_IMAGE_ATTESTATIONS_JSON" &&
    deployConfig.get(key) !== value
  ) {
    failures.push(
      `backend/.env.example: ${key} 与 deploy/config/chaimir.env 不一致`,
    );
  }
}

const headersPath = "images/service/frontend/security-headers.conf";
const headers = read(headersPath);
for (const [pattern, message] of [
  [
    /add_header Content-Security-Policy[\s\S]*connect-src 'self'/,
    "必须限制 API/WS 为同源 connect-src",
  ],
  [
    new RegExp(`frame-src 'self' ${escapeRegex(TOOL_ORIGIN)}`),
    `必须只允许同源和 ${TOOL_ORIGIN} 的 iframe`,
  ],
  [/add_header X-Content-Type-Options "nosniff"/, "必须启用 nosniff"],
  [/add_header Referrer-Policy "no-referrer"/, "必须启用 no-referrer"],
  [
    /add_header Strict-Transport-Security "max-age=31536000; includeSubDomains"/,
    "必须启用 HSTS",
  ],
  [/add_header Permissions-Policy/, "必须显式限制浏览器高风险权限"],
])
  requireMatch(headersPath, headers, pattern, message);

const ingressPath = "deploy/base/ingress/ingress.yaml";
const ingress = read(ingressPath);
const publicHostCount = (
  ingress.match(
    new RegExp(`(?:host:|-)\\s*${escapeRegex(PUBLIC_HOST)}`, "g"),
  ) ?? []
).length;
if (publicHostCount !== 4)
  failures.push(
    `${ingressPath}: 主 Ingress 与拒绝 Ingress 的 rules/TLS 必须各引用一次 ${PUBLIC_HOST}(当前 ${publicHostCount} 次)`,
  );
if (/host:\s*chaimir\s*$/m.test(ingress))
  failures.push(`${ingressPath}: 不能保留旧主域名 chaimir`);

for (const name of ["acceptance", "staging", "prod-saas", "prod-school"]) {
  const relativePath = `deploy/overlays/${name}/kustomization.yaml`;
  const source = read(relativePath);
  if (/^images:\s*$/m.test(source))
    failures.push(`${relativePath}: overlay 不得复制镜像 digest`);
  if (new RegExp(`value:\\s*${escapeRegex(PUBLIC_HOST)}`).test(source))
    failures.push(`${relativePath}: 正式域名只能在 base 维护`);
  if (name !== "prod-school" && /^\s*- DEPLOY_MODE=/m.test(source))
    failures.push(`${relativePath}: 不得重复覆盖默认部署形态`);
}

const stalePatterns = [
  /https?:\/\/chaimir(?:[/:]|$)/i,
  /chaimir\.local/i,
  /harbor\.chaimir\.local/i,
  /chaimir\.example\.edu/i,
  /registry\.chaimir\.local/i,
  /127\.0\.0\.1:5000/i,
];
const scanRoots = [
  "frontend/apps",
  "frontend/packages",
  "backend/internal",
  "backend/cmd",
  "deploy/base",
  "deploy/overlays",
  "images/service/frontend",
  "scripts",
];
const ignoredNames = new Set(["node_modules", "dist", ".tmp"]);

/** scan 递归检查代码与清单中是否恢复旧公共域名或 registry 占位。 */
function scan(relativeDirectory) {
  const directory = path.join(ROOT, relativeDirectory);
  if (!fs.existsSync(directory)) return;
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (ignoredNames.has(entry.name)) continue;
    const relativePath = path.join(relativeDirectory, entry.name);
    if (entry.isDirectory()) scan(relativePath);
    else if (
      /\.(?:ts|tsx|js|jsx|go|yaml|yml|env|conf|ps1|sh|md)$/i.test(entry.name)
    ) {
      const source = fs.readFileSync(path.join(ROOT, relativePath), "utf8");
      for (const pattern of stalePatterns) {
        if (pattern.test(source)) {
          failures.push(
            `${relativePath}: 检出旧公共域名/registry 占位值 ${pattern}`,
          );
          break;
        }
      }
    }
  }
}
for (const root of scanRoots) scan(root);

if (failures.length > 0) {
  console.error("前端部署边界检查失败:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exitCode = 1;
} else {
  console.log(
    `前端部署边界检查通过:公共入口 ${PUBLIC_HOST},工具 origin ${TOOL_ORIGIN},初始化容器、base 与四个 overlay 契约一致。`,
  );
}
