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
const serviceSecret = requireConfig("CHAIMIR_EVENT_LISTENER_SERVICE_AUTH_SECRET", process.env.CHAIMIR_EVENT_LISTENER_SERVICE_AUTH_SECRET);
const maxSkewSeconds = requiredPositiveInteger("CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS");
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
const provider = new ethers.JsonRpcProvider(rpcRequest);
const emptyState = () => ({ version: 1, initialized: false, cursor: -1, cursorHash: "", events: [], updatedAt: "", lastError: "" });
let state = emptyState();

function requireConfig(name, value) {
  if (value === undefined || value === null || String(value).trim() === "") throw new Error(`${name} is required`);
  return String(value).trim();
}

function requiredPositiveInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isInteger(value) || value <= 0) throw new Error(`${name} must be a positive integer`);
  return value;
}

function requiredNonNegativeInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isInteger(value) || value < 0) throw new Error(`${name} must be a non-negative integer`);
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
  if (!raw.startsWith("/runtime-state/") || raw.endsWith("/")) throw new Error(`${name} is invalid`);
  return raw;
}

function timingSafeEqualHex(left, right) {
  if (!/^[0-9a-f]{64}$/i.test(left || "") || !/^[0-9a-f]{64}$/i.test(right || "")) return false;
  return crypto.timingSafeEqual(Buffer.from(left, "hex"), Buffer.from(right, "hex"));
}

function pruneNonces(now) {
  for (const [nonce, timestamp] of seenNonces.entries()) if (Math.abs(now - timestamp) > maxSkewSeconds) seenNonces.delete(nonce);
  while (seenNonces.size > 4096) seenNonces.delete(seenNonces.keys().next().value);
}

async function verifyServiceSignature(request, reply) {
  const timestamp = Number(request.headers["x-chaimir-timestamp"]);
  const nonce = String(request.headers["x-chaimir-nonce"] || "");
  const signature = String(request.headers["x-chaimir-signature"] || "");
  const now = Math.floor(Date.now() / 1000);
  pruneNonces(now);
  if (!Number.isInteger(timestamp) || Math.abs(now - timestamp) > maxSkewSeconds || !nonce || seenNonces.has(nonce)) return reply.code(401).send({ error: "unauthorized" });
  const bodyHash = crypto.createHash("sha256").update(JSON.stringify(request.body ?? null)).digest("hex");
  const canonical = [request.method, request.url, timestamp, nonce, bodyHash].join("\n");
  const expected = crypto.createHmac("sha256", serviceSecret).update(canonical).digest("hex");
  if (!timingSafeEqualHex(signature, expected)) return reply.code(401).send({ error: "unauthorized" });
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

async function persistState() {
  await fs.mkdir(path.dirname(statePath), { recursive: true, mode: 0o700 });
  const tempPath = `${statePath}.tmp`;
  await fs.writeFile(tempPath, `${JSON.stringify(state)}\n`, { encoding: "utf8", mode: 0o600 });
  await fs.rename(tempPath, statePath);
}

function eventKey(log) { return `${String(log.transactionHash).toLowerCase()}:${log.index}`; }

function serializeLog(log) {
  return { key: eventKey(log), address: ethers.getAddress(log.address), blockNumber: log.blockNumber, blockHash: log.blockHash, transactionHash: log.transactionHash, transactionIndex: log.transactionIndex, index: log.index, data: log.data, topics: log.topics };
}

async function rewindIfReorged() {
  if (!state.initialized || state.cursor < 0 || !state.cursorHash) return;
  const block = await provider.getBlock(state.cursor);
  if (block?.hash && block.hash.toLowerCase() !== state.cursorHash.toLowerCase()) {
    const rewindTo = Math.max(-1, state.cursor - reorgDepth);
    state.cursor = rewindTo;
    state.cursorHash = "";
    state.events = state.events.filter((item) => item.blockNumber <= rewindTo);
  }
}

async function pollOnce() {
  await rewindIfReorged();
  const latest = await provider.getBlockNumber();
  const confirmed = latest - confirmations;
  if (confirmed < 0) return;
  if (!state.initialized) { state.cursor = Math.max(-1, confirmed - lookbackBlocks); state.initialized = true; }
  if (confirmed <= state.cursor) {
    state.lastError = "";
    state.updatedAt = new Date().toISOString();
    await persistState();
    return;
  }
  const fromBlock = state.cursor + 1;
  const toBlock = Math.min(confirmed, fromBlock + maxBlockRange - 1);
  const logs = await provider.getLogs({ address: contractAddress, topics: [eventTopic], fromBlock, toBlock });
  const existing = new Set(state.events.map((item) => item.key));
  for (const log of logs) {
    const item = serializeLog(log);
    if (!existing.has(item.key)) { state.events.push(item); existing.add(item.key); }
  }
  if (state.events.length > maxEvents) state.events = state.events.slice(-maxEvents);
  const cursorBlock = await provider.getBlock(toBlock);
  state.cursor = toBlock;
  state.cursorHash = cursorBlock?.hash || "";
  state.lastError = "";
  state.updatedAt = new Date().toISOString();
  await persistState();
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
app.get("/events/latest", { preHandler: verifyServiceSignature }, async () => ({ contract_address: contractAddress, event_topic: eventTopic, cursor: state.cursor, events: state.events, updated_at: state.updatedAt }));

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
