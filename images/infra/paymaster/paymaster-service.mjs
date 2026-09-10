// 本文件启动 ERC-4337 v0.6 paymaster 服务,为受 HMAC 鉴权的内部调用生成标准赞助数据。
import crypto from "node:crypto";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = requiredPositiveInteger("CHAIMIR_IMAGE_PORT");
const httpTimeoutSeconds = requiredPositiveInteger("CHAIMIR_HTTP_TIMEOUT_SECONDS");
const rpcUrl = requireHttpUrl("CHAIMIR_EVM_RPC_URL", process.env.CHAIMIR_EVM_RPC_URL);
const entryPointAddress = requireAddress("CHAIMIR_ENTRYPOINT_ADDRESS", process.env.CHAIMIR_ENTRYPOINT_ADDRESS);
const paymasterAddress = requireAddress("CHAIMIR_PAYMASTER_ADDRESS", process.env.CHAIMIR_PAYMASTER_ADDRESS);
const signerKey = requireConfig("CHAIMIR_PAYMASTER_PRIVATE_KEY", process.env.CHAIMIR_PAYMASTER_PRIVATE_KEY);
const configuredSignerAddress = requireAddress("CHAIMIR_PAYMASTER_SIGNER_ADDRESS", process.env.CHAIMIR_PAYMASTER_SIGNER_ADDRESS);
const serviceSecret = requireConfig("CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET", process.env.CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET);
const maxSkewSeconds = requiredPositiveInteger("CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS");
const bodyLimit = requiredPositiveInteger("CHAIMIR_MAX_BODY_BYTES");
const validitySeconds = requiredPositiveInteger("CHAIMIR_PAYMASTER_VALIDITY_SECONDS");
const maxUserOperationBytes = requiredPositiveInteger("CHAIMIR_PAYMASTER_MAX_USER_OPERATION_BYTES");
const rpcTimeoutSeconds = requiredPositiveInteger("CHAIMIR_PAYMASTER_RPC_TIMEOUT_SECONDS");
const seenNonces = new Map();

const userOperationTuple = "(address sender,uint256 nonce,bytes initCode,bytes callData,uint256 callGasLimit,uint256 verificationGasLimit,uint256 preVerificationGas,uint256 maxFeePerGas,uint256 maxPriorityFeePerGas,bytes paymasterAndData,bytes signature)";

function requireConfig(name, value) {
  if (value === undefined || value === null || String(value).trim() === "") throw new Error(`${name} is required`);
  return String(value).trim();
}

function requiredPositiveInteger(name) {
  const value = Number(requireConfig(name, process.env[name]));
  if (!Number.isInteger(value) || value <= 0) throw new Error(`${name} must be a positive integer`);
  return value;
}

function requireHttpUrl(name, value) {
  const raw = requireConfig(name, value);
  let parsed;
  try {
    parsed = new URL(raw);
  } catch {
    throw new Error(`${name} is invalid`);
  }
  const authority = raw.slice(raw.indexOf("://") + 3).split(/[/?#]/, 1)[0];
  const portText = authority.slice(authority.lastIndexOf(":") + 1);
  const numericPort = Number(portText);
  if (!/^https?:$/.test(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password || parsed.hash || !/^\d+$/.test(portText) || !Number.isInteger(numericPort) || numericPort < 1 || numericPort > 65535) throw new Error(`${name} is invalid`);
  return raw;
}

function requireAddress(name, value) {
  const raw = requireConfig(name, value);
  try {
    const address = ethers.getAddress(raw);
    if (address === ethers.ZeroAddress) throw new Error("zero address");
    return address;
  } catch {
    throw new Error(`${name} is invalid`);
  }
}

function timingSafeEqualHex(left, right) {
  const leftBuffer = Buffer.from(left || "", "hex");
  const rightBuffer = Buffer.from(right || "", "hex");
  return leftBuffer.length === rightBuffer.length && crypto.timingSafeEqual(leftBuffer, rightBuffer);
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

function requireHexBytes(name, value, maxBytes) {
  if (typeof value !== "string" || !/^0x(?:[0-9a-fA-F]{2})*$/.test(value) || (value.length - 2) / 2 > maxBytes) throw new Error(`${name} is invalid`);
  return value.toLowerCase();
}

function requireQuantity(name, value) {
  if (typeof value !== "string" || !/^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/.test(value)) throw new Error(`${name} is invalid`);
  const parsed = BigInt(value);
  if (parsed < 0n || parsed > ((1n << 256n) - 1n)) throw new Error(`${name} is invalid`);
  return ethers.toBeHex(parsed);
}

function validateUserOperation(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("userOperation must be an object");
  const allowed = new Set(["sender", "nonce", "initCode", "callData", "callGasLimit", "verificationGasLimit", "preVerificationGas", "maxFeePerGas", "maxPriorityFeePerGas", "paymasterAndData", "signature"]);
  for (const key of Object.keys(value)) if (!allowed.has(key)) throw new Error(`userOperation.${key} is not supported`);
  const required = [...allowed];
  for (const key of required) if (!(key in value)) throw new Error(`userOperation.${key} is required`);
  const sender = requireAddress("userOperation.sender", value.sender);
  const normalized = {
    sender,
    nonce: requireQuantity("userOperation.nonce", value.nonce),
    initCode: requireHexBytes("userOperation.initCode", value.initCode, 65536),
    callData: requireHexBytes("userOperation.callData", value.callData, 131072),
    callGasLimit: requireQuantity("userOperation.callGasLimit", value.callGasLimit),
    verificationGasLimit: requireQuantity("userOperation.verificationGasLimit", value.verificationGasLimit),
    preVerificationGas: requireQuantity("userOperation.preVerificationGas", value.preVerificationGas),
    maxFeePerGas: requireQuantity("userOperation.maxFeePerGas", value.maxFeePerGas),
    maxPriorityFeePerGas: requireQuantity("userOperation.maxPriorityFeePerGas", value.maxPriorityFeePerGas),
    paymasterAndData: requireHexBytes("userOperation.paymasterAndData", value.paymasterAndData, 131072),
    signature: requireHexBytes("userOperation.signature", value.signature, 131072),
  };
  if (normalized.paymasterAndData !== "0x") throw new Error("userOperation.paymasterAndData must be empty before sponsorship");
  if (Buffer.byteLength(JSON.stringify(normalized), "utf8") > maxUserOperationBytes) throw new Error("userOperation is too large");
  return normalized;
}

function standardPaymasterPrefix(validUntil, validAfter) {
  return ethers.solidityPacked(["address", "uint48", "uint48"], [paymasterAddress, validUntil, validAfter]);
}

const contractProbeUserOperation = {
  sender: "0x000000000000000000000000000000000000dEaD",
  nonce: "0x0",
  initCode: "0x",
  callData: "0x",
  callGasLimit: "0x1",
  verificationGasLimit: "0x1",
  preVerificationGas: "0x1",
  maxFeePerGas: "0x1",
  maxPriorityFeePerGas: "0x1",
  paymasterAndData: standardPaymasterPrefix(1n, 0n),
  signature: "0x",
};

async function verifyContractMaterials(provider) {
  const [entryPointCode, paymasterCode] = await Promise.all([provider.getCode(entryPointAddress), provider.getCode(paymasterAddress)]);
  if (!entryPointCode || entryPointCode === "0x") throw new Error("configured EntryPoint has no deployed code");
  if (!paymasterCode || paymasterCode === "0x") throw new Error("configured paymaster has no deployed code");
  const [userOperationHash, paymasterHash] = await Promise.all([
    entryPoint.getUserOpHash(contractProbeUserOperation),
    paymaster.getHash(contractProbeUserOperation, 1n, 0n),
  ]);
  if (!ethers.isHexString(userOperationHash, 32) || !ethers.isHexString(paymasterHash, 32)) throw new Error("configured ERC-4337 contracts do not expose the required v0.6 interface");
}

const rpcRequest = new ethers.FetchRequest(rpcUrl);
rpcRequest.timeout = rpcTimeoutSeconds * 1000;
const provider = new ethers.JsonRpcProvider(rpcRequest);
const signer = new ethers.Wallet(signerKey, provider);
if (signer.address !== configuredSignerAddress) throw new Error("CHAIMIR_PAYMASTER_SIGNER_ADDRESS does not match the supplied signer");
const entryPoint = new ethers.Contract(entryPointAddress, [`function getUserOpHash(${userOperationTuple} userOp) view returns (bytes32)`], provider);
const paymaster = new ethers.Contract(paymasterAddress, [`function getHash(${userOperationTuple} userOp,uint48 validUntil,uint48 validAfter) view returns (bytes32)`], provider);

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
  request.log.error({ err: error }, "paymaster request failed");
  const statusCode = error.statusCode === 413 ? 413 : 400;
  return reply.code(statusCode).send({ error: statusCode === 413 ? "request_too_large" : "request_invalid" });
});
app.get("/healthz", async () => ({ status: "ok", entrypoint: entryPointAddress, paymaster: paymasterAddress }));
app.post("/sponsor", { preHandler: verifyServiceSignature }, async (request, reply) => {
  let userOperation;
  try {
    userOperation = validateUserOperation(request.body);
  } catch {
    return reply.code(400).send({ error: "invalid_user_operation" });
  }
  const now = Math.floor(Date.now() / 1000);
  const validAfter = 0n;
  const validUntil = BigInt(now + validitySeconds);
  if (validUntil > 0xffffffffffffn) return reply.code(503).send({ error: "validity_window_unavailable" });
  const unsignedPaymasterData = standardPaymasterPrefix(validUntil, validAfter);
  const unsignedUserOperation = { ...userOperation, paymasterAndData: unsignedPaymasterData };
  try {
    const userOperationHash = await entryPoint.getUserOpHash(unsignedUserOperation);
    const paymasterHash = await paymaster.getHash(unsignedUserOperation, validUntil, validAfter);
    if (!ethers.isHexString(userOperationHash, 32) || !ethers.isHexString(paymasterHash, 32)) throw new Error("contract returned invalid hash");
    const signature = await signer.signMessage(ethers.getBytes(paymasterHash));
    return { version: "0.6", entryPoint: entryPointAddress, paymaster: paymasterAddress, userOperationHash, paymasterHash, validUntil: validUntil.toString(), validAfter: validAfter.toString(), paymasterAndData: `${unsignedPaymasterData}${signature.slice(2)}` };
  } catch (error) {
    request.log.error({ err: error }, "paymaster contract interaction failed");
    return reply.code(503).send({ error: "paymaster_unavailable" });
  }
});

await verifyContractMaterials(provider);
try {
  await app.listen({ host: "0.0.0.0", port });
} catch (error) {
  app.log.error(error);
  process.exit(1);
}
