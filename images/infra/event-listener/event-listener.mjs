// 本文件启动链上事件监听服务,只向通过 HMAC 鉴权的内部调用返回链上事件。
import crypto from "node:crypto";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const rpcUrl = process.env.CHAIMIR_EVENT_RPC_URL;
const contractAddress = process.env.CHAIMIR_EVENT_CONTRACT_ADDRESS;
const eventTopic = process.env.CHAIMIR_EVENT_TOPIC;
const serviceSecret = process.env.CHAIMIR_EVENT_LISTENER_SERVICE_AUTH_SECRET;
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

const provider = new ethers.JsonRpcProvider(requireConfig("CHAIMIR_EVENT_RPC_URL", rpcUrl));
requireConfig("CHAIMIR_EVENT_CONTRACT_ADDRESS", contractAddress);
requireConfig("CHAIMIR_EVENT_TOPIC", eventTopic);
requireConfig("CHAIMIR_EVENT_LISTENER_SERVICE_AUTH_SECRET", serviceSecret);

const app = Fastify({ logger: true });

app.get("/healthz", async () => ({ status: "ok", contract: contractAddress }));
app.get("/events/latest", { preHandler: verifyServiceSignature }, async () => {
  const blockNumber = await provider.getBlockNumber();
  const fromBlock = Math.max(blockNumber - Number(process.env.CHAIMIR_EVENT_LOOKBACK_BLOCKS || "100"), 0);
  const logs = await provider.getLogs({ address: contractAddress, topics: [eventTopic], fromBlock, toBlock: blockNumber });
  return { blockNumber, logs };
});

app.listen({ host: "0.0.0.0", port });
