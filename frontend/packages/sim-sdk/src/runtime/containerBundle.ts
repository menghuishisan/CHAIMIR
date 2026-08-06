// 本文件在容器内校验归档摘要、解包并按 meta.entry 装配扩展仿真包。
//
// bundle 不经网络:容器网络 deny-all、根文件系统只读,归档由后端经 k8s exec stdin 推入,
// 落到唯一可写的 emptyDir 后就地解包(见 docs/总-镜像与容器设计.md §六之一)。
// 装配前逐字节校验 sha256 与后端登记的 bundle_hash 一致 —— 投递通道可信不等于内容可信,
// 哈希是"跑的就是审核过的那份代码"的唯一凭据。
//
// ZIP/TAR 用 Node 内置能力自解而非引三方库:runner 是承载不可信代码的最小镜像,
// 依赖越少可审计面越小,而 store/deflate 两种压缩方法用内置 zlib 即可完整覆盖
// (后端 upload.DetectArchiveFormat 也只接受这两种归档)。

import { createHash } from 'node:crypto';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import { inflateRawSync } from 'node:zlib';

import type { SimPackage } from '../types';

/** WORKDIR 是容器内唯一可写目录(emptyDir 挂载点),随会话命名空间删除一并消失。 */
const WORKDIR = process.env.SIM_RUNNER_WORKDIR ?? '/tmp/sim-bundle';
/**
 * ENTRY_PATTERN 只接受 `.mjs`。
 * 不接受 `.js`:归档内没有 package.json 时 Node 按 CJS 解析 `.js`,而扩展包必须默认导出
 * SimPackage(ESM 语义)。允许 `.js` 会让"能不能装配"取决于归档里有没有 type:module,
 * 同一份代码在不同打包方式下表现不同 —— 收敛成单一扩展名,消掉这类不确定性。
 */
const ENTRY_PATTERN = /\.mjs$/;

export interface LoadBundleRequest {
  bundleBase64: string;
  bundleHash: string;
  format: 'zip' | 'tar';
  entry: string;
}

/**
 * loadPackageFromBundle 校验摘要、解包归档并 import 入口模块,返回未经协议校验的包对象。
 * 协议校验由 SimEngine 构造时统一执行,避免两处各校一遍产生口径分叉。
 */
export async function loadPackageFromBundle(request: LoadBundleRequest): Promise<unknown> {
  const bytes = Buffer.from(request.bundleBase64, 'base64');
  const actual = createHash('sha256').update(bytes).digest('hex');
  const expected = String(request.bundleHash ?? '').trim().toLowerCase();
  if (actual !== expected) {
    throw new Error('仿真包内容校验失败:归档摘要与登记值不一致');
  }

  const files = request.format === 'tar' ? extractTar(bytes) : extractZip(bytes);
  const manifest = readManifest(files);
  const entryPath = safeEntryPath(request.entry || manifestEntry(manifest));
  if (!files.has(entryPath)) {
    throw new Error(`仿真包入口模块不存在: ${entryPath}`);
  }

  const root = join(WORKDIR, actual.slice(0, 16));
  for (const [name, content] of files) {
    const target = join(root, name);
    mkdirSync(dirname(target), { recursive: true });
    writeFileSync(target, content);
  }

  const loaded = (await import(pathToFileURL(join(root, entryPath)).href)) as {
    default?: unknown;
    simPackage?: unknown;
  };
  const simPackage = (loaded.default ?? loaded.simPackage) as SimPackage | undefined;
  if (!simPackage) {
    throw new Error('仿真包入口模块必须默认导出 SimPackage');
  }
  const meta = manifest.meta as { code?: unknown; version?: unknown };
  if (simPackage.meta?.code !== meta.code || simPackage.meta?.version !== meta.version) {
    throw new Error('仿真包入口模块的编号或版本与 manifest 声明不一致');
  }
  return simPackage;
}

/**
 * manifestEntry 从 manifest 读取入口声明,供命令未显式带 entry 时回落。
 */
function manifestEntry(manifest: Record<string, unknown>): string {
  const meta = manifest.meta as { entry?: unknown } | undefined;
  return typeof meta?.entry === 'string' ? meta.entry : '';
}

/**
 * safeEntryPath 规范化入口路径并拒绝逃逸写法。
 * 归档成员名与 entry 都来自不可信上传,`..`、绝对路径与盘符会把写入点带出工作目录。
 */
function safeEntryPath(entry: string): string {
  const value = String(entry ?? '').trim().replace(/\\/g, '/');
  if (value === '') {
    throw new Error('仿真包未声明入口模块');
  }
  if (value.startsWith('/') || /^[A-Za-z]:/.test(value) || value.split('/').includes('..')) {
    throw new Error(`仿真包入口模块路径非法: ${value}`);
  }
  if (!ENTRY_PATTERN.test(value)) {
    throw new Error('仿真包入口模块必须是 .mjs');
  }
  const normalized = resolve(WORKDIR, value);
  if (!normalized.startsWith(resolve(WORKDIR) + sep)) {
    throw new Error(`仿真包入口模块路径越界: ${value}`);
  }
  return value;
}

/**
 * readManifest 读取归档内 sim-package.json;允许归档工具自动包的唯一顶层目录。
 * 命中顶层目录时把全部成员名剥去该前缀,后续路径解析只面对一种形态。
 */
function readManifest(files: Map<string, Buffer>): Record<string, unknown> {
  const direct = files.get('sim-package.json');
  if (direct) {
    return parseManifest(direct);
  }
  const topLevel = [...files.keys()].filter(
    (name) => name.endsWith('/sim-package.json') && name.split('/').length === 2,
  );
  if (topLevel.length !== 1) {
    throw new Error('仿真包归档根目录缺少 sim-package.json');
  }
  const prefix = topLevel[0].slice(0, -'sim-package.json'.length);
  for (const name of [...files.keys()]) {
    if (name.startsWith(prefix)) {
      const content = files.get(name);
      if (content) {
        files.set(name.slice(prefix.length), content);
      }
      files.delete(name);
    }
  }
  const stripped = files.get('sim-package.json');
  if (!stripped) {
    throw new Error('仿真包归档根目录缺少 sim-package.json');
  }
  return parseManifest(stripped);
}

/**
 * parseManifest 严格解析 manifest,拒绝空对象与缺少 meta 的结构。
 */
function parseManifest(content: Buffer): Record<string, unknown> {
  const manifest = JSON.parse(content.toString('utf8')) as Record<string, unknown> | null;
  if (!manifest || typeof manifest !== 'object' || !manifest.meta) {
    throw new Error('仿真包 manifest 结构无效');
  }
  return manifest;
}

/**
 * extractZip 解出 ZIP 归档的全部普通文件,按中央目录遍历。
 */
function extractZip(bytes: Buffer): Map<string, Buffer> {
  const files = new Map<string, Buffer>();
  const eocd = findEndOfCentralDirectory(bytes);
  const entryCount = bytes.readUInt16LE(eocd + 10);
  let cursor = bytes.readUInt32LE(eocd + 16);

  for (let index = 0; index < entryCount; index += 1) {
    if (bytes.readUInt32LE(cursor) !== 0x02014b50) {
      throw new Error('仿真包归档中央目录结构损坏');
    }
    const method = bytes.readUInt16LE(cursor + 10);
    const compressedSize = bytes.readUInt32LE(cursor + 20);
    const nameLength = bytes.readUInt16LE(cursor + 28);
    const extraLength = bytes.readUInt16LE(cursor + 30);
    const commentLength = bytes.readUInt16LE(cursor + 32);
    const localOffset = bytes.readUInt32LE(cursor + 42);
    const name = bytes.subarray(cursor + 46, cursor + 46 + nameLength).toString('utf8');
    cursor += 46 + nameLength + extraLength + commentLength;

    if (name.endsWith('/')) {
      continue;
    }
    const localNameLength = bytes.readUInt16LE(localOffset + 26);
    const localExtraLength = bytes.readUInt16LE(localOffset + 28);
    const dataStart = localOffset + 30 + localNameLength + localExtraLength;
    const raw = bytes.subarray(dataStart, dataStart + compressedSize);
    files.set(normalizeMemberName(name), inflateMember(method, raw));
  }
  if (files.size === 0) {
    throw new Error('仿真包归档为空');
  }
  return files;
}

/**
 * findEndOfCentralDirectory 从尾部反向定位 EOCD 记录。
 */
function findEndOfCentralDirectory(bytes: Buffer): number {
  for (let offset = bytes.length - 22; offset >= 0; offset -= 1) {
    if (bytes.readUInt32LE(offset) === 0x06054b50) {
      return offset;
    }
  }
  throw new Error('仿真包归档不是有效的 ZIP');
}

/**
 * inflateMember 按压缩方式还原成员内容;只接受 store 与 deflate。
 * 同步解压:runner 是单命令短命进程,没有并发需要,同步实现更易审计。
 */
function inflateMember(method: number, raw: Buffer): Buffer {
  if (method === 0) {
    return Buffer.from(raw);
  }
  if (method === 8) {
    return inflateRawSync(raw);
  }
  throw new Error(`仿真包归档使用了不支持的压缩方式: ${method}`);
}

/**
 * extractTar 解出 TAR 归档的全部普通文件。
 */
function extractTar(bytes: Buffer): Map<string, Buffer> {
  const files = new Map<string, Buffer>();
  let offset = 0;
  while (offset + 512 <= bytes.length) {
    const header = bytes.subarray(offset, offset + 512);
    const name = header.subarray(0, 100).toString('utf8').replace(/\0.*$/, '');
    if (name === '') {
      break;
    }
    const size =
      parseInt(header.subarray(124, 136).toString('utf8').replace(/\0.*$/, '').trim(), 8) || 0;
    const typeFlag = header.subarray(156, 157).toString('utf8');
    const dataStart = offset + 512;
    if (typeFlag === '0' || typeFlag === '\0') {
      files.set(normalizeMemberName(name), Buffer.from(bytes.subarray(dataStart, dataStart + size)));
    }
    offset = dataStart + Math.ceil(size / 512) * 512;
  }
  if (files.size === 0) {
    throw new Error('仿真包归档不是有效的 TAR');
  }
  return files;
}

/**
 * normalizeMemberName 统一分隔符并拒绝逃逸成员名。
 */
function normalizeMemberName(name: string): string {
  const value = name.replace(/\\/g, '/').replace(/^\.\//, '');
  if (value.startsWith('/') || value.split('/').includes('..')) {
    throw new Error(`仿真包归档包含非法成员路径: ${name}`);
  }
  return value;
}
