// 本文件启动 DApp 钱包后端,提供基于真实签名密钥的挑战签名接口。
import crypto from "node:crypto";
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const signerKey = process.env.CHAIMIR_WALLET_SIGNER_PRIVATE_KEY;
const sessionSecret = process.env.CHAIMIR_WALLET_SESSION_SECRET;

function requireConfig(name, value) {
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

const wallet = new ethers.Wallet(requireConfig("CHAIMIR_WALLET_SIGNER_PRIVATE_KEY", signerKey));
requireConfig("CHAIMIR_WALLET_SESSION_SECRET", sessionSecret);

const app = Fastify({ logger: true, bodyLimit: Number(process.env.CHAIMIR_MAX_BODY_BYTES || "1048576") });

app.get("/healthz", async () => ({ status: "ok", address: wallet.address }));
app.post("/challenge", async (request) => {
  const nonce = crypto.randomBytes(32).toString("hex");
  const domain = request.body?.domain || "chaimir";
  const challenge = `${domain}:${nonce}`;
  const signature = await wallet.signMessage(challenge);
  return { challenge, signature, address: wallet.address };
});

app.listen({ host: "0.0.0.0", port });
