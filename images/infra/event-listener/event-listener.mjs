// 本文件启动链上事件消费服务,把固定绑定的合约事件持久化到 runtime-state。
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = requiredPositiveInteger("CHAIMIR_IMAGE_PORT");
const httpTimeoutSeconds = requiredPositiveInteger("CHAIMIR_HTTP_TIMEOUT_SECONDS");
const rpcUrl = requireHttpUrl("CHAIMIR_EVENT_RPC_URL", process.env.CHAIMIR_EVENT_RPC_URL);
const contractAddress = requireNonZeroAddress("CHAIMIR_EVENT_CONTRACT_ADDRESS", process.env.CHAIMIR_EVENT_CONTRACT_ADDRESS);
const eventTopic = requireNonZeroTopic("CHAIMIR_EVENT_TOPIC", process.env.CHAIMIR_EVENT_TOPIC);
const serviceSecret = requiredSecret("CHAIMIR_EVENT_LISTENER_SERVICE_AUTH_SECRET");
const maxSkewSeconds = requiredPositiveInteger("CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS");
const maxNonces = requiredPositiveInteger("CHAIMIR_IMAGE_SERVICE_AUTH_MAX_NONCES");
const lookbackBlocks = requiredNonNegativeInteger("CHAIMIR_EVENT_LOOKBACK_BLOCKS");
const confirmations = requiredNonNegativeInteger("CHAIMIR_EVENT_CONFIRMATIONS");
const maxBlockRange = requiredPositiveInteger("CHAIMIR_EVENT_MAX_BLOCK_RANGE");
const maxEvents = requiredPositiveInteger("CHAIMIR_EVENT_MAX_EVENTS");
const reorgDepth = requiredPositiveInteger("CHAIMIR_EVENT_REORG_DEPTH");
const pollIntervalSeconds = requiredPositiveInteger("CHAIMIR_EVENT_POLL_INTERVAL_SECONDS");
const rpcTimeoutSeconds = requiredPositiveInteger("CHAIMIR_EVENT_RPC_TIMEOUT_SECONDS");
const statePath = requireStatePath("CHAIMIR_EVENT_STATE_PATH", process.env.CHAIMIR_EVENT_STATE_PATH);
const seenNonces = new Map();
const rpcRequest = new ethers.FetchRequest(rpcUrl);
rpcRequest.timeout = rpcTimeoutSeconds * 1000;
// 查询前后必须重新观察链状态，不能用短时缓存掩盖重组。
const provider = new ethers.JsonRpcProvider(rpcRequest, undefined, { cacheTimeout: -1 });
const emptyState = () => ({ version: 1, initialized: false, cursor: -1, cursorHash: "", events: [], updatedAt: "", lastError: "" });
let state = emptyState();
let pollInFlight = false;

function requireConfig(name, value) {
  if (value === undefined || value === null || String(value).trim() === "") throw new Error(`${name} is required`);
  return String(value).trim();
}

function requiredPositiveInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isSafeInteger(value) || value <= 0) throw new Error(`${name} must be a positive safe integer`);
  return value;
}

function requiredSecret(name) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function requiredNonNegativeInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`${name} must be a non-negative safe integer`);
  return value;
}

function requireHttpUrl(name, value) {
  const raw = requireConfig(name, value);
  let parsed;
  try { parsed = new URL(raw); } catch { throw new Error(`${name} is invalid`); }
  const authority = raw.slice(raw.indexOf("://") + 3).split(/[/?#]/, 1)[0];
  const portText = authority.slice(authority.lastIndexOf(":") + 1);
  const parsedPort = Number(portText);
  if (!/^https?:$/.test(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password || parsed.hash || !/^\d+$/.test(portText) || !Number.isInteger(parsedPort) || parsedPort < 1 || parsedPort > 65535) throw new Error(`${name} is invalid`);
  return raw;
}

function requireNonZeroAddress(name, value) {
  try {
    const address = ethers.getAddress(requireConfig(name, value));
    if (address === ethers.ZeroAddress) throw new Error("zero address");
    return address;
  } catch { throw new Error(`${name} is invalid`); }
}

function requireNonZeroTopic(name, value) {
  const topic = requireConfig(name, value);
  if (!/^0x[0-9a-fA-F]{64}$/.test(topic) || /^0x0{64}$/.test(topic)) throw new Error(`${name} is invalid`);
  return topic.toLowerCase();
}

function requireStatePath(name, value) {
  const raw = requireConfig(name, value);
  if (!raw.startsWith("/runtime-state/") || raw.endsWith("/") || path.posix.normalize(raw) !== raw) throw new Error(`${name} is invalid`);
  return raw;
}

function timingSafeEqualHex(left, right) {
  if (!/^[0-9a-f]{64}$/i.test(left || "") || !/^[0-9a-f]{64}$/i.test(right || "")) return false;
  return crypto.timingSafeEqual(Buffer.from(left, "hex"), Buffer.from(right, "hex"));
}

function pruneNonces(now) {
  for (const [nonce, timestamp] of seenNonces.entries()) if (now - timestamp > maxSkewSeconds) seenNonces.delete(nonce);
}

async function verifyServiceSignature(request, reply) {
  const timestamp = Number(request.headers["x-chaimir-timestamp"]);
  const nonce = String(request.headers["x-chaimir-nonce"] || "");
  const signature = String(request.headers["x-chaimir-signature"] || "");
  const now = Math.floor(Date.now() / 1000);
  pruneNonces(now);
  if (!Number.isSafeInteger(timestamp) || Math.abs(now - timestamp) > maxSkewSeconds || !/^[A-Za-z0-9_-]{16,128}$/.test(nonce) || seenNonces.has(nonce)) return reply.code(401).send({ error: "unauthorized" });
  const bodyHash = crypto.createHash("sha256").update(JSON.stringify(request.body ?? null)).digest("hex");
  const canonical = [request.method, request.url, timestamp, nonce, bodyHash].join("\n");
  const expected = crypto.createHmac("sha256", serviceSecret).update(canonical).digest("hex");
  if (!timingSafeEqualHex(signature, expected)) return reply.code(401).send({ error: "unauthorized" });
  if (seenNonces.size >= maxNonces) return reply.code(503).send({ error: "authentication_capacity_exceeded" });
  seenNonces.set(nonce, timestamp);
}

function validateState(value) {
  if (!value || value.version !== 1 || typeof value.initialized !== "boolean" || !Number.isInteger(value.cursor) || !Array.isArray(value.events)) throw new Error("event state is invalid");
  if (value.events.length > maxEvents) value.events = value.events.slice(-maxEvents);
  value.cursorHash = typeof value.cursorHash === "string" ? value.cursorHash : "";
  value.updatedAt = typeof value.updatedAt === "string" ? value.updatedAt : "";
  value.lastError = typeof value.lastError === "string" ? value.lastError : "";
  return value;
}

async function loadState() {
  try { state = validateState(JSON.parse(await fs.readFile(statePath, "utf8"))); }
  catch (error) {
    if (error?.code === "ENOENT") { state = emptyState(); return; }
    throw error;
  }
}

async function persistState(next) {
  await fs.mkdir(path.dirname(statePath), { recursive: true, mode: 0o700 });
  const tempPath = `${statePath}.tmp`;
  try {
    await fs.writeFile(tempPath, `${JSON.stringify(next)}\n`, { encoding: "utf8", mode: 0o600 });
    await fs.rename(tempPath, statePath);
  } catch (error) {
    try {
      await fs.rm(tempPath, { force: true });
    } catch (cleanupError) {
      throw new AggregateError([error, cleanupError], "event state write and cleanup failed");
    }
    throw error;
  }
}

function eventKey(log) { return `${String(log.transactionHash).toLowerCase()}:${log.index}`; }

function serializeLog(log) {
  return { key: eventKey(log), address: ethers.getAddress(log.address), blockNumber: log.blockNumber, blockHash: log.blockHash, transactionHash: log.transactionHash, transactionIndex: log.transactionIndex, index: log.index, data: log.data, topics: log.topics };
}

async function rewindIfReorged(next) {
  if (!next.initialized || next.cursor < 0 || !next.cursorHash) return;
  const block = await provider.getBlock(next.cursor);
  if (!block?.hash) throw new Error("event checkpoint block is unavailable");
  if (block.hash.toLowerCase() !== next.cursorHash.toLowerCase()) {
    const rewindTo = Math.max(-1, next.cursor - reorgDepth);
    const retained = next.events.filter((item) => item.blockNumber <= rewindTo);
    const checked = new Map();
    for (const item of retained) {
      if (!checked.has(item.blockNumber)) checked.set(item.blockNumber, await provider.getBlock(item.blockNumber));
      if (checked.get(item.blockNumber)?.hash?.toLowerCase() !== item.blockHash.toLowerCase()) throw new Error("event reorganization exceeds the configured replay depth");
    }
    next.cursor = rewindTo;
    next.cursorHash = "";
    next.events = retained;
  }
}

async function pollOnce() {
  if (pollInFlight) return;
  pollInFlight = true;
  try {
    const latest = await provider.getBlockNumber();
    const confirmed = latest - confirmations;
    const anchor = confirmed >= 0 ? await provider.getBlock(confirmed) : null;
    if (confirmed >= 0 && !anchor?.hash) throw new Error("event confirmed block is unavailable");
    const next = latest < state.cursor ? emptyState() : { ...state, events: [...state.events] };
    await rewindIfReorged(next);
    if (confirmed < 0) {
      Object.assign(next, emptyState());
    } else {
      if (!next.initialized) {
        next.cursor = Math.max(-1, confirmed - lookbackBlocks);
        next.initialized = true;
      }
      if (next.cursor < confirmed) {
        const fromBlock = next.cursor + 1;
        const toBlock = Math.min(confirmed, fromBlock + maxBlockRange - 1);
        const checkpoint = toBlock === confirmed ? anchor : await provider.getBlock(toBlock);
        if (!checkpoint?.hash) throw new Error("event range end block is unavailable");
        const logs = await provider.getLogs({ address: contractAddress, topics: [eventTopic], fromBlock, toBlock });
        const existing = new Set(next.events.map((item) => item.key));
        for (const log of logs) {
          const item = serializeLog(log);
          if (!existing.has(item.key)) { next.events.push(item); existing.add(item.key); }
        }
        next.events.sort((a, b) => a.blockNumber - b.blockNumber || a.transactionIndex - b.transactionIndex || a.index - b.index);
        if (next.events.length > maxEvents) next.events = next.events.slice(-maxEvents);
        next.cursor = toBlock;
        next.cursorHash = checkpoint.hash;
      } else if (!next.cursorHash && next.cursor >= 0) {
        const block = await provider.getBlock(next.cursor);
        if (!block?.hash) throw new Error("event checkpoint block is unavailable");
        next.cursorHash = block.hash;
      }
    }
    if (anchor) {
      const after = await provider.getBlock(confirmed);
      if (after?.hash?.toLowerCase() !== anchor.hash.toLowerCase()) throw new Error("event chain changed during collection");
    }
    next.lastError = "";
    next.updatedAt = new Date().toISOString();
    await persistState(next);
    state = next;
  } finally {
    pollInFlight = false;
  }
}

const httpTimeoutMilliseconds = httpTimeoutSeconds * 1000;
const app = Fastify({
  logger: true,
  bodyLimit: 64 * 1024,
  connectionTimeout: httpTimeoutMilliseconds,
  requestTimeout: httpTimeoutMilliseconds,
  keepAliveTimeout: httpTimeoutMilliseconds,
  headersTimeout: httpTimeoutMilliseconds,
});
app.get("/healthz", async (_request, reply) => {
  if (!state.initialized || state.lastError) return reply.code(503).send({ status: "not_ready", configured: true });
  return { status: "ok", configured: true, cursor: state.cursor, event_count: state.events.length };
});
app.get("/events/latest", { preHandler: verifyServiceSignature }, async (_request, reply) => {
  if (!state.initialized || state.lastError) return reply.code(503).send({ error: "event_listener_unavailable" });
  return { contract_address: contractAddress, event_topic: eventTopic, cursor: state.cursor, events: state.events, updated_at: state.updatedAt };
});

try {
  await loadState();
  await pollOnce();
  await app.listen({ host: "0.0.0.0", port });
  const timer = setInterval(() => pollOnce().catch((error) => { state.lastError = "event polling failed"; app.log.error(error); }), pollIntervalSeconds * 1000);
  timer.unref();
} catch (error) {
  app.log.error(error);
  process.exit(1);
}
