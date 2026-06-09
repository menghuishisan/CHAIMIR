// 本文件启动账户抽象 paymaster 服务,只为通过 HMAC 鉴权的内部调用签名代付数据。
import crypto from "node:crypto";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const rpcUrl = process.env.CHAIMIR_EVM_RPC_URL;
const signerKey = process.env.CHAIMIR_PAYMASTER_PRIVATE_KEY;
const entryPointAddress = process.env.CHAIMIR_ENTRYPOINT_ADDRESS;
const serviceSecret = process.env.CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET;
const maxSkewSeconds = Number(process.env.CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS || "300");
const seenNonces = new Map();

function requireConfig(name, value) {
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function timingSafeEqualHex(left, right) {
  const leftBuffer = Buffer.from(left || "", "hex");
  const rightBuffer = Buffer.from(right || "", "hex");
  return leftBuffer.length === rightBuffer.length && crypto.timingSafeEqual(leftBuffer, rightBuffer);
}

function pruneNonces(now) {
  for (const [nonce, timestamp] of seenNonces.entries()) {
    if (Math.abs(now - timestamp) > maxSkewSeconds) {
      seenNonces.delete(nonce);
    }
  }
}

function verifyServiceSignature(request, reply) {
  const timestamp = Number(request.headers["x-chaimir-timestamp"]);
  const nonce = String(request.headers["x-chaimir-nonce"] || "");
  const signature = String(request.headers["x-chaimir-signature"] || "");
  const now = Math.floor(Date.now() / 1000);
  pruneNonces(now);
  if (!Number.isInteger(timestamp) || Math.abs(now - timestamp) > maxSkewSeconds || !nonce || seenNonces.has(nonce)) {
    return reply.code(401).send({ error: "unauthorized" });
  }
  const bodyHash = crypto.createHash("sha256").update(JSON.stringify(request.body ?? null)).digest("hex");
  const canonical = [request.method, request.url, timestamp, nonce, bodyHash].join("\n");
  const expected = crypto.createHmac("sha256", serviceSecret).update(canonical).digest("hex");
  if (!timingSafeEqualHex(signature, expected)) {
    return reply.code(401).send({ error: "unauthorized" });
  }
  seenNonces.set(nonce, timestamp);
}

function validateUserOperation(userOperation) {
  if (!userOperation || typeof userOperation !== "object") {
    throw new Error("userOperation must be an object");
  }
  for (const field of ["sender", "nonce", "callData"]) {
    if (!userOperation[field]) {
      throw new Error(`userOperation.${field} is required`);
    }
  }
  ethers.getAddress(userOperation.sender);
}

const provider = new ethers.JsonRpcProvider(requireConfig("CHAIMIR_EVM_RPC_URL", rpcUrl));
const signer = new ethers.Wallet(requireConfig("CHAIMIR_PAYMASTER_PRIVATE_KEY", signerKey), provider);
requireConfig("CHAIMIR_ENTRYPOINT_ADDRESS", entryPointAddress);
requireConfig("CHAIMIR_PAYMASTER_SERVICE_AUTH_SECRET", serviceSecret);

const app = Fastify({ logger: true, bodyLimit: Number(process.env.CHAIMIR_MAX_BODY_BYTES || "1048576") });

app.get("/healthz", async () => ({ status: "ok", address: signer.address }));
app.post("/sponsor", { preHandler: verifyServiceSignature }, async (request, reply) => {
  const userOperation = request.body;
  try {
    validateUserOperation(userOperation);
  } catch (error) {
    return reply.code(400).send({ error: "invalid_user_operation" });
  }
  const network = await provider.getNetwork();
  const payload = ethers.id(JSON.stringify({ chainId: network.chainId.toString(), entryPointAddress, userOperation }));
  const signature = await signer.signMessage(ethers.getBytes(payload));
  return { paymaster: signer.address, entryPoint: entryPointAddress, signature };
});

app.listen({ host: "0.0.0.0", port });
