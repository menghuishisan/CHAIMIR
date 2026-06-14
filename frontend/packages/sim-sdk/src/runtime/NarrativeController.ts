// 本文件负责根据仿真状态命中教学叙事步骤,供工作台展示步骤解释和检查点。

import type { NarrativeStep, SimEvent, SimPackage, SimState } from '../types';

export class NarrativeController<TState extends SimState = SimState> {
  private readonly simPackage: SimPackage<TState>;

  constructor(simPackage: SimPackage<TState>) {
    this.simPackage = simPackage;
  }

  /**
   * 返回当前状态命中的教学步骤,没有命中时使用第一步作为默认说明。
   */
  currentStep(state: TState, event?: SimEvent): NarrativeStep | undefined {
    const steps = this.simPackage.narrative ?? [];
    const matched = steps.find((step) => step.trigger(state, event));
    return matched ?? steps[0];
  }

  /**
   * 返回完整叙事步骤列表,用于左侧步骤导航。
   */
  allSteps(): NarrativeStep[] {
    return this.simPackage.narrative ?? [];
  }
}
