// 本文件定义整数边界仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface IntegerCase {
  id: string;
  label: string;
  input: number;
  result: number;
  checked: boolean;
  failed: boolean;
}

export interface IntegerBoundaryState extends SimState {
  phaseIndex: number;
  maxValue: number;
  cases: IntegerCase[];
  checkedMath: boolean;
  cappedInput: boolean;
  lastTransition: string;
}

export const integerPhases = [
  { id: 'input', label: '接收数值输入', detail: '读取金额或数量', effect: '合约接收用户传入的金额、数量或比例。', reason: '边界风险首先来自未限制的外部输入。' },
  { id: 'range', label: '检查范围', detail: '限制最大值', effect: '合约检查输入是否超过业务允许范围。', reason: '业务上不可能的数值应在入口被拒绝。' },
  { id: 'compute', label: '执行算术运算', detail: '乘法加法', effect: '合约对输入执行乘法、加法或比例计算。', reason: '溢出和精度截断通常发生在计算步骤。' },
  { id: 'checked', label: '启用 checked 运算', detail: '失败即回滚', effect: '溢出或下溢直接失败,不会产生回绕结果。', reason: '显式失败优于产生看似合法的错误数值。' },
  { id: 'test', label: '边界用例覆盖', detail: '最大最小值测试', effect: '测试最大值、零值和临界值。', reason: '边界测试能防止修复后再次引入数值漏洞。' },
];
