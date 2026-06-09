// 本文件启动链上事件监听服务,从运行期声明的 RPC 与合约地址读取真实事件。
import process from "node:process";
import { ethers } from "ethers";
import Fastify from "fastify";

const port = Number(process.env.CHAIMIR_IMAGE_PORT || "8080");
const rpcUrl = process.env.CHAIMIR_EVENT_RPC_URL;
const contractAddress = process.env.CHAIMIR_EVENT_CONTRACT_ADDRESS;
const eventTopic = process.env.CHAIMIR_EVENT_TOPIC;

function requireConfig(name, value) {
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

const provider = new ethers.JsonRpcProvider(requireConfig("CHAIMIR_EVENT_RPC_URL", rpcUrl));
requireConfig("CHAIMIR_EVENT_CONTRACT_ADDRESS", contractAddress);
requireConfig("CHAIMIR_EVENT_TOPIC", eventTopic);

const app = Fastify({ logger: true });

app.get("/healthz", async () => ({ status: "ok", contract: contractAddress }));
app.get("/events/latest", async () => {
  const blockNumber = await provider.getBlockNumber();
  const fromBlock = Math.max(blockNumber - Number(process.env.CHAIMIR_EVENT_LOOKBACK_BLOCKS || "100"), 0);
  const logs = await provider.getLogs({ address: contractAddress, topics: [eventTopic], fromBlock, toBlock: blockNumber });
  return { blockNumber, logs };
});

app.listen({ host: "0.0.0.0", port });
