// 本文件启动 DApp 钱包后端,提供受 HMAC 调用鉴权保护的挑战签名接口。
import crypto from "node:crypto";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const signerKey = process.env.CHAIMIR_WALLET_SIGNER_PRIVATE_KEY;
const sessionSecret = process.env.CHAIMIR_WALLET_SESSION_SECRET;
const maxSkewSeconds = Number(process.env.CHAIMIR_IMAGE_SERVICE_AUTH_MAX_SKEW_SECONDS || "300");
const allowedDomains = new Set((process.env.CHAIMIR_WALLET_ALLOWED_DOMAINS || "").split(",").map((item) => item.trim()).filter(Boolean));
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
  const expected = crypto.createHmac("sha256", sessionSecret).update(canonical).digest("hex");
  if (!timingSafeEqualHex(signature, expected)) {
    return reply.code(401).send({ error: "unauthorized" });
  }
  seenNonces.set(nonce, timestamp);
}

const wallet = new ethers.Wallet(requireConfig("CHAIMIR_WALLET_SIGNER_PRIVATE_KEY", signerKey));
requireConfig("CHAIMIR_WALLET_SESSION_SECRET", sessionSecret);
if (allowedDomains.size === 0) {
  throw new Error("CHAIMIR_WALLET_ALLOWED_DOMAINS is required");
}

const app = Fastify({ logger: true, bodyLimit: Number(process.env.CHAIMIR_MAX_BODY_BYTES || "1048576") });

app.get("/healthz", async () => ({ status: "ok", address: wallet.address }));
app.post("/challenge", { preHandler: verifyServiceSignature }, async (request, reply) => {
  const nonce = crypto.randomBytes(32).toString("hex");
  const domain = request.body?.domain || "chaimir";
  if (!allowedDomains.has(domain)) {
    return reply.code(400).send({ error: "domain_not_allowed" });
  }
  const challenge = `${domain}:${nonce}`;
  const signature = await wallet.signMessage(challenge);
  return { challenge, signature, address: wallet.address };
});

app.listen({ host: "0.0.0.0", port });
