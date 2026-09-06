import { readdirSync } from "node:fs";
import { dirname } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const originalConfigPath = process.env.CHAIMIR_HARDHAT_ORIGINAL_CONFIG;
const originalModule = await import(pathToFileURL(originalConfigPath).href);
const originalConfig = originalModule.default ?? originalModule;
const originalRoot = dirname(fileURLToPath(pathToFileURL(originalConfigPath)));
const localCompilers = Object.fromEntries(
  readdirSync("/usr/local/bin")
    .map((name) => /^solc-(\d+\.\d+\.\d+)$/.exec(name))
    .filter(Boolean)
    .map((match) => [match[1], `/usr/local/bin/solc-${match[1]}`]),
);

function patchCompiler(compiler) {
  const version = typeof compiler === "string" ? compiler : compiler?.version;
  const path = localCompilers[version];
  if (!version || !path) {
    throw new Error(`Solidity 编译器版本未被 testcase-evm 镜像声明: ${version ?? "<missing>"}`);
  }
  if (typeof compiler === "string") return { version, path };
  return { ...compiler, path };
}

function patchSolidity(solidity) {
  if (typeof solidity === "string") return patchCompiler(solidity);
  if (Array.isArray(solidity)) {
    if (solidity.length === 0) throw new Error("Hardhat solidity.compilers 不能为空");
    return { compilers: solidity.map(patchCompiler) };
  }
  if (solidity && typeof solidity === "object") {
    if ("version" in solidity) return patchCompiler(solidity);
    if (Array.isArray(solidity.compilers)) {
      if (solidity.compilers.length === 0) throw new Error("Hardhat solidity.compilers 不能为空");
      return { ...solidity, compilers: solidity.compilers.map(patchCompiler) };
    }
    if (solidity.profiles) {
      return {
        ...solidity,
        profiles: Object.fromEntries(Object.entries(solidity.profiles).map(([name, profile]) => {
          if (!profile || !Array.isArray(profile.compilers) || profile.compilers.length === 0) {
            throw new Error(`Hardhat profile ${name} 必须显式声明 compilers`);
          }
          return [name, { ...profile, compilers: profile.compilers.map(patchCompiler) }];
        })),
      };
    }
  }
  throw new Error("Hardhat 项目必须显式声明 solidity 编译器版本");
}

if (originalConfig.solidity === undefined) {
  throw new Error("Hardhat 项目必须显式声明 solidity 编译器版本");
}

export default {
  ...originalConfig,
  paths: {
    ...originalConfig.paths,
    root: originalRoot,
    sources: {
      ...originalConfig.paths?.sources,
      solidity: originalConfig.paths?.sources?.solidity ?? `${originalRoot}/contracts`,
    },
  },
  solidity: patchSolidity(originalConfig.solidity),
};
