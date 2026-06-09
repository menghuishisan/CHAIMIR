// 本文件启动账户抽象 paymaster 服务,只在运行期凭据和 RPC 配置完整时提供真实代付接口。
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const rpcUrl = process.env.CHAIMIR_EVM_RPC_URL;
const signerKey = process.env.CHAIMIR_PAYMASTER_PRIVATE_KEY;
const entryPointAddress = process.env.CHAIMIR_ENTRYPOINT_ADDRESS;

function requireConfig(name, value) {
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

const provider = new ethers.JsonRpcProvider(requireConfig("CHAIMIR_EVM_RPC_URL", rpcUrl));
const signer = new ethers.Wallet(requireConfig("CHAIMIR_PAYMASTER_PRIVATE_KEY", signerKey), provider);
requireConfig("CHAIMIR_ENTRYPOINT_ADDRESS", entryPointAddress);

const app = Fastify({ logger: true, bodyLimit: Number(process.env.CHAIMIR_MAX_BODY_BYTES || "1048576") });

app.get("/healthz", async () => ({ status: "ok", address: signer.address }));
app.post("/sponsor", async (request) => {
  const userOperation = request.body;
  const payload = ethers.id(JSON.stringify({ entryPointAddress, userOperation }));
  const signature = await signer.signMessage(ethers.getBytes(payload));
  return { paymaster: signer.address, signature };
});

app.listen({ host: "0.0.0.0", port });
