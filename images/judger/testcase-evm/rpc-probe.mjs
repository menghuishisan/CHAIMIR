// 本文件用 Node 内置 fetch 严格探测判题组合提供的 EVM JSON-RPC。
const url = process.argv[2];
if (!url || !/^https?:\/\//.test(url)) {
  process.stderr.write("EVM RPC URL 必须使用 HTTP 或 HTTPS\n");
  process.exit(64);
}

const response = await fetch(url, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_chainId", params: [] }),
  signal: AbortSignal.timeout(5000),
});
if (!response.ok) {
  process.stderr.write(`EVM RPC 返回 HTTP ${response.status}\n`);
  process.exit(66);
}
process.stdout.write(await response.text());
