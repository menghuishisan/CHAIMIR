// 本文件启动 DApp 钱包后端,提供受 HMAC 鉴权的地址绑定挑战和一次性外部签名校验。
import crypto from "node:crypto";
import fs from "node:fs/promises";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = requiredPositiveInteger("CHAIMIR_IMAGE_PORT");
const httpTimeoutSeconds = requiredPositiveInteger("CHAIMIR_HTTP_TIMEOUT_SECONDS");
const sessionSecret = requireConfig("CHAIMIR_WALLET_SESSION_SECRET", process.env.CHAIMIR_WALLET_SESSION_SECRET);
const maxSkewSeconds = requiredPositiveInteger("CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS");
const bodyLimit = requiredPositiveInteger("CHAIMIR_MAX_BODY_BYTES");
const challengeTtlSeconds = requiredPositiveInteger("CHAIMIR_WALLET_CHALLENGE_TTL_SECONDS");
const maxChallenges = requiredPositiveInteger("CHAIMIR_WALLET_MAX_CHALLENGES");
const statePath = requireStatePath("CHAIMIR_WALLET_STATE_PATH", process.env.CHAIMIR_WALLET_STATE_PATH);
const allowedDomains = parseAllowedDomains(requireConfig("CHAIMIR_WALLET_ALLOWED_DOMAINS", process.env.CHAIMIR_WALLET_ALLOWED_DOMAINS));
const seenNonces = new Map();
const challenges = new Map();
let persistQueue = Promise.resolve();

function requireConfig(name, value) {
  if (value === undefined || value === null || String(value).trim() === "") throw new Error(`${name} is required`);
  return String(value).trim();
}

function requiredPositiveInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isInteger(value) || value <= 0) throw new Error(`${name} must be a positive integer`);
  return value;
}

// parseAllowedDomains 保证签名消息域白名单与 CompositionSpec 提供的值完全一致;
// 格式错误必须阻止启动。
function parseAllowedDomains(value) {
  const domains = value.split(",");
  const result = new Set();
  for (const domain of domains) {
    if (!domain || domain !== domain.trim() || result.has(domain)) throw new Error("CHAIMIR_WALLET_ALLOWED_DOMAINS contains an empty, padded, or duplicate domain");
    result.add(domain);
  }
  return result;
}

function requireStatePath(name, value) {
  const path = requireConfig(name, value);
  if (!/^\/runtime-state\/[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*$/.test(path) || path.includes("..")) throw new Error(`${name} must be a normalized file path under /runtime-state`);
  return path;
}

function timingSafeEqualHex(left, right) {
  if (!/^[0-9a-f]{64}$/i.test(left || "") || !/^[0-9a-f]{64}$/i.test(right || "")) return false;
  return crypto.timingSafeEqual(Buffer.from(left, "hex"), Buffer.from(right, "hex"));
}

function pruneExpired(now) {
  let changed = false;
  for (const [nonce, record] of challenges.entries()) {
    if (record.expiresAt <= now || record.used) {
      challenges.delete(nonce);
      changed = true;
    }
  }
  for (const [nonce, timestamp] of seenNonces.entries()) {
    if (Math.abs(now - timestamp) > maxSkewSeconds) {
      seenNonces.delete(nonce);
      changed = true;
    }
  }
  while (seenNonces.size > 4096) {
    seenNonces.delete(seenNonces.keys().next().value);
    changed = true;
  }
  return changed;
}

function stateDocument() {
  return {
    version: 1,
    challenges: [...challenges.values()],
    service_nonces: [...seenNonces.entries()].map(([nonce, timestamp]) => ({ nonce, timestamp })),
    updated_at: new Date().toISOString(),
  };
}

async function persistState() {
  persistQueue = persistQueue.then(async () => {
    const parent = statePath.slice(0, statePath.lastIndexOf("/"));
    await fs.mkdir(parent, { recursive: true, mode: 0o700 });
    const temporaryPath = `${statePath}.${process.pid}.${crypto.randomBytes(8).toString("hex")}.tmp`;
    try {
      await fs.writeFile(temporaryPath, JSON.stringify(stateDocument()), { encoding: "utf8", mode: 0o600 });
      await fs.rename(temporaryPath, statePath);
    } catch (error) {
      await fs.rm(temporaryPath, { force: true }).catch(() => {});
      throw error;
    }
  });
  return persistQueue;
}

function validateState(value) {
  if (!value || value.version !== 1 || !Array.isArray(value.challenges) || !Array.isArray(value.service_nonces)) throw new Error("wallet state is invalid");
  for (const record of value.challenges) {
    if (!record || typeof record.nonce !== "string" || !/^[0-9a-f]{64}$/.test(record.nonce) || typeof record.domain !== "string" || typeof record.address !== "string" || typeof record.challenge !== "string" || !Number.isInteger(record.issuedAt) || !Number.isInteger(record.expiresAt) || typeof record.used !== "boolean") throw new Error("wallet challenge state is invalid");
    if (!allowedDomains.has(record.domain) || ethers.getAddress(record.address) !== record.address || record.expiresAt <= record.issuedAt || record.challenge.length > 4096) throw new Error("wallet challenge state is invalid");
  }
  for (const item of value.service_nonces) {
    if (!item || typeof item.nonce !== "string" || item.nonce.length < 16 || !Number.isInteger(item.timestamp)) throw new Error("wallet service nonce state is invalid");
  }
  if (value.challenges.length > maxChallenges || value.service_nonces.length > 4096) throw new Error("wallet state capacity exceeded");
}

async function loadState() {
  let raw;
  try {
    raw = await fs.readFile(statePath, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return;
    throw error;
  }
  const value = JSON.parse(raw);
  validateState(value);
  const now = Math.floor(Date.now() / 1000);
  for (const record of value.challenges) {
    if (record.expiresAt > now && !record.used) challenges.set(record.nonce, record);
  }
  for (const item of value.service_nonces) seenNonces.set(item.nonce, item.timestamp);
  if (pruneExpired(now)) await persistState();
}

async function verifyServiceSignature(request, reply) {
  const timestamp = Number(request.headers["x-chaimir-timestamp"]);
  const nonce = String(request.headers["x-chaimir-nonce"] || "");
  const signature = String(request.headers["x-chaimir-signature"] || "");
  const now = Math.floor(Date.now() / 1000);
  const changed = pruneExpired(now);
  if (changed) await persistState();
  if (!Number.isInteger(timestamp) || Math.abs(now - timestamp) > maxSkewSeconds || !nonce || seenNonces.has(nonce)) return reply.code(401).send({ error: "unauthorized" });
  const bodyHash = crypto.createHash("sha256").update(JSON.stringify(request.body ?? null)).digest("hex");
  const canonical = [request.method, request.url, timestamp, nonce, bodyHash].join("\n");
  const expected = crypto.createHmac("sha256", sessionSecret).update(canonical).digest("hex");
  if (!timingSafeEqualHex(signature, expected)) return reply.code(401).send({ error: "unauthorized" });
  seenNonces.set(nonce, timestamp);
  await persistState();
}

function requestString(value) {
  const text = String(value ?? "").trim();
  return text && text.length <= 4096 ? text : null;
}

function requestAddress(value) {
  const text = requestString(value);
  if (!text) return null;
  try {
    const address = ethers.getAddress(text);
    return address === ethers.ZeroAddress ? null : address;
  } catch {
    return null;
  }
}

function createChallenge(domain, address, nonce, issuedAt) {
  const expiresAt = issuedAt + challengeTtlSeconds;
  const challenge = [
    "Chaimir DApp wallet sign-in",
    `Domain: ${domain}`,
    `Address: ${address}`,
    `Nonce: ${nonce}`,
    `Issued At: ${issuedAt}`,
    `Expiration Time: ${expiresAt}`,
  ].join("\n");
  return { nonce, domain, address, challenge, issuedAt, expiresAt, used: false };
}

function invalidRequest(reply, error) {
  return reply.code(400).send({ valid: false, error });
}

if (allowedDomains.size === 0) throw new Error("CHAIMIR_WALLET_ALLOWED_DOMAINS must contain at least one domain");

const httpTimeoutMilliseconds = httpTimeoutSeconds * 1000;
const app = Fastify({
  logger: true,
  bodyLimit,
  connectionTimeout: httpTimeoutMilliseconds,
  requestTimeout: httpTimeoutMilliseconds,
  keepAliveTimeout: httpTimeoutMilliseconds,
  headersTimeout: httpTimeoutMilliseconds,
});
app.setErrorHandler((error, request, reply) => {
  request.log.error({ err: error }, "wallet request failed");
  const statusCode = error.statusCode === 413 ? 413 : 400;
  return reply.code(statusCode).send({ valid: false, error: statusCode === 413 ? "request_too_large" : "request_invalid" });
});
app.get("/healthz", async () => ({ status: "ok", state_path: statePath }));
app.post("/challenge", { preHandler: verifyServiceSignature }, async (request, reply) => {
  const domain = requestString(request.body?.domain);
  const address = requestAddress(request.body?.address);
  if (!domain || !allowedDomains.has(domain)) return invalidRequest(reply, "domain_not_allowed");
  if (!address) return invalidRequest(reply, "address_invalid");
  const now = Math.floor(Date.now() / 1000);
  if (pruneExpired(now)) await persistState();
  if (challenges.size >= maxChallenges) return reply.code(503).send({ valid: false, error: "challenge_capacity_exceeded" });
  const record = createChallenge(domain, address, crypto.randomBytes(32).toString("hex"), now);
  challenges.set(record.nonce, record);
  await persistState();
  return { challenge: record.challenge, expires_at: record.expiresAt, address: record.address };
});
app.post("/verify", { preHandler: verifyServiceSignature }, async (request, reply) => {
  const challenge = requestString(request.body?.challenge);
  const signature = requestString(request.body?.signature);
  const address = requestAddress(request.body?.address);
  if (!challenge || !signature || !address) return invalidRequest(reply, "request_invalid");
  const nonceMatch = challenge.match(/\nNonce: ([0-9a-f]{64})\n/);
  const nonce = nonceMatch?.[1] || "";
  const record = challenges.get(nonce);
  const now = Math.floor(Date.now() / 1000);
  if (!record || record.challenge !== challenge || record.expiresAt <= now || record.used || record.address !== address || !allowedDomains.has(record.domain)) return invalidRequest(reply, "challenge_invalid");
  let recovered;
  try {
    recovered = ethers.verifyMessage(challenge, signature);
  } catch {
    return invalidRequest(reply, "signature_invalid");
  }
  const valid = recovered === record.address;
  if (!valid) return { valid: false, address: record.address, recovered };
  record.used = true;
  await persistState();
  return { valid, address: record.address, recovered };
});

await loadState();
try {
  await app.listen({ host: "0.0.0.0", port });
} catch (error) {
  app.log.error(error);
  process.exit(1);
}
