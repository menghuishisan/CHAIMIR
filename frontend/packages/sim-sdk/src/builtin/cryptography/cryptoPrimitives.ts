// 本文件提供密码学仿真共享的教学级过程模型,统一规范化、摘要、签名、Merkle 路径和门限份额逻辑。

import { fnv1aHex } from '../../runtime/deterministic';

export const FIELD_PRIME = 97;
export const GROUP_GENERATOR = 5;

/**
 * canonicalEncode 将对象字段按字典序编码,避免同义输入得到不同摘要。
 */
export function canonicalEncode(value: Record<string, string | number | boolean>): string {
  return Object.keys(value)
    .sort()
    .map((key) => `${key}=${String(value[key])}`)
    .join('|');
}

/**
 * roundDigest 用三轮域分离压缩模拟真实摘要的吸收、混合和截断过程。
 */
export function roundDigest(domain: string, encoded: string, length = 16): string {
  const absorb = fnv1aHex(`${domain}:absorb:${encoded}`, length);
  const mix = fnv1aHex(`${domain}:mix:${absorb}:${encoded.length}`, length);
  return fnv1aHex(`${domain}:final:${mix}`, length);
}

/**
 * hashChainDigest 绑定记录序号、父哈希和规范化载荷,用于哈希链摘要。
 */
export function hashChainDigest(index: number, payload: string, parentHash: string): string {
  return roundDigest('hash-chain', canonicalEncode({ index, parentHash, payload }));
}

/**
 * derivePrivateKey 从标签派生教学用私钥。
 */
export function derivePrivateKey(label: string): string {
  return roundDigest('private-key', canonicalEncode({ label }), 12);
}

/**
 * derivePublicKey 从私钥派生教学用公钥承诺。
 */
export function derivePublicKey(privateKey: string): string {
  return roundDigest('public-key', canonicalEncode({ privateKey }), 12);
}

/**
 * messageDigest 绑定域、消息和 nonce,用于签名摘要。
 */
export function messageDigest(domain: string, message: string, nonce: number): string {
  return roundDigest('message-digest', canonicalEncode({ domain, message, nonce }));
}

/**
 * signDigest 生成教学用签名,显式绑定私钥派生的公钥和摘要。
 */
export function signDigest(digest: string, privateKey: string): string {
  const publicKey = derivePublicKey(privateKey);
  return roundDigest('signature', canonicalEncode({ digest, privateKey, publicKey }));
}

/**
 * recoverRegisteredPublicKey 通过登记公钥候选复算签名,模拟公钥恢复和身份匹配。
 */
export function recoverRegisteredPublicKey(digest: string, signature: string, registry: Record<string, string>): string {
  return Object.entries(registry).find(([, privateKey]) => signDigest(digest, privateKey) === signature)?.[0] ?? '';
}

/**
 * merkleLeafHash 计算带叶子索引和域分离的叶子摘要。
 */
export function merkleLeafHash(index: number, value: string): string {
  return roundDigest('merkle-leaf', canonicalEncode({ index, value }), 12);
}

/**
 * merkleParentHash 按左右方向合并两个子摘要。
 */
export function merkleParentHash(left: string, right: string): string {
  return roundDigest('merkle-parent', canonicalEncode({ left, right }), 12);
}

export interface MerkleProofStep {
  siblingId: string;
  siblingHash: string;
  siblingSide: 'left' | 'right';
}

/**
 * foldMerkleProof 用目标叶子和兄弟路径逐层折叠出根摘要。
 */
export function foldMerkleProof(leafHash: string, steps: MerkleProofStep[]): string {
  return steps.reduce((current, step) => (step.siblingSide === 'left' ? merkleParentHash(step.siblingHash, current) : merkleParentHash(current, step.siblingHash)), leafHash);
}

/**
 * polynomialShare 生成门限签名教学用多项式份额。
 */
export function polynomialShare(secret: number, coefficients: number[], x: number): number {
  return coefficients.reduce((sum, coefficient, power) => (sum + coefficient * x ** (power + 1)) % FIELD_PRIME, secret % FIELD_PRIME);
}

/**
 * partialThresholdSignature 生成绑定份额位置和消息摘要的部分签名。
 */
export function partialThresholdSignature(message: string, x: number, shareValue: number): string {
  return roundDigest('threshold-partial', canonicalEncode({ message, shareValue, x }), 12);
}

/**
 * lagrangeCoefficientAtZero 计算用于门限聚合的零点拉格朗日系数。
 */
export function lagrangeCoefficientAtZero(x: number, xs: number[]): number {
  const numerator = xs.filter((other) => other !== x).reduce((acc, other) => (acc * (0 - other + FIELD_PRIME)) % FIELD_PRIME, 1);
  const denominator = xs.filter((other) => other !== x).reduce((acc, other) => (acc * (x - other + FIELD_PRIME)) % FIELD_PRIME, 1);
  return (numerator * inverseMod(denominator)) % FIELD_PRIME;
}

/**
 * aggregateThresholdSignature 组合有效部分签名和拉格朗日系数。
 */
export function aggregateThresholdSignature(message: string, parts: Array<{ x: number; partial: string }>): string {
  const xs = parts.map((part) => part.x);
  const folded = parts
    .map((part) => `${part.x}:${lagrangeCoefficientAtZero(part.x, xs)}:${part.partial}`)
    .sort()
    .join('|');
  return roundDigest('threshold-aggregate', canonicalEncode({ folded, message }), 16);
}

/**
 * groupMul 表示教学用固定生成元上的标量乘法。
 */
export function groupMul(value: number): number {
  return (GROUP_GENERATOR * normalizeField(value)) % FIELD_PRIME;
}

/**
 * schnorrCommit 生成 Schnorr 风格群承诺。
 */
export function schnorrCommit(randomizer: number): string {
  return `c${groupMul(randomizer).toString(16).padStart(2, '0')}`;
}

/**
 * schnorrResponse 生成 Schnorr 风格响应 r + c * x。
 */
export function schnorrResponse(randomizer: number, challenge: number, secret: number): number {
  return normalizeField(randomizer + challenge * secret);
}

/**
 * verifySchnorrRelation 校验 g*s = R + c*X 的教学群关系。
 */
export function verifySchnorrRelation(commitment: string, challenge: number, publicKey: number, response: number): boolean {
  const left = groupMul(response);
  const right = normalizeField(commitmentValue(commitment) + challenge * publicKey);
  return left === right;
}

/**
 * normalizeField 将数值归约到教学有限域。
 */
function normalizeField(value: number): number {
  return ((value % FIELD_PRIME) + FIELD_PRIME) % FIELD_PRIME;
}

/**
 * inverseMod 计算教学有限域中的乘法逆元。
 */
function inverseMod(value: number): number {
  for (let candidate = 1; candidate < FIELD_PRIME; candidate += 1) {
    if ((normalizeField(value) * candidate) % FIELD_PRIME === 1) return candidate;
  }
  return 1;
}

/**
 * commitmentValue 从承诺字符串中取回群元素。
 */
function commitmentValue(commitment: string): number {
  return Number.parseInt(commitment.slice(1), 16) % FIELD_PRIME;
}
