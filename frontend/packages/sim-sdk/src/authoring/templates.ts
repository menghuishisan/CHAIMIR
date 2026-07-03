// 本文件提供开发者创建自定义仿真包的最小完整模板,避免绕过 M4 协议自行实现运行时。

import type { CheckpointResult, SimEvent, SimPackage, SimState, ViewSpec } from '../types';
import { defineSimPackage } from './manifest';

interface DeveloperTemplateState extends SimState {
  metrics: { progress: number };
  checkpointValues: { completed: boolean };
}

/**
 * createDeveloperTemplate 返回一个可运行的前端仿真包模板,作者应替换状态、阶段和渲染语义数据。
 */
export function createDeveloperTemplate(code: string): SimPackage<DeveloperTemplateState> {
  return defineSimPackage({
    meta: {
      code,
      name: '自定义仿真模板',
      category: 'consensus',
      version: '0.1.0',
      compute: 'frontend',
      summary: '用于演示 M4 仿真包需要提供的完整协议字段。',
      learningObjectives: ['理解自描述仿真包结构', '掌握确定性 reducer 与封闭模式输出'],
      scaleLimit: { nodes: 24, maxTick: 120, maxEvents: 240 },
    },
    initState: createTemplateInitialState,
    reducer: reduceTemplateState,
    interactions: [
      {
        id: 'complete',
        kind: 'button',
        label: '完成演示',
        description: '注入一个用户事件,让 reducer 更新状态。',
        emits: 'complete',
        labelTag: 'normal',
      },
    ],
    render: renderTemplateView,
    narrative: [
      {
        id: 'template-ready',
        title: '模板阶段',
        trigger: () => true,
        highlight: ['ready'],
        explain: '开发者需要用真实教学语义替换模板阶段。',
        defaultDurationMs: 1200,
      },
    ],
    codeTrace: {
      sourceCode: ['function reducer(state, event) {', '  return nextState;', '}'].join('\n'),
      language: 'pseudocode',
      lineMapping: [
        { line: 1, triggerCondition: 'init', annotation: '进入 reducer' },
        { line: 2, triggerCondition: 'update', annotation: '根据事件生成新状态' },
      ],
      variableWatch: [{ name: 'event', extract: 'state._trace.variables.event', format: 'string' }],
    },
    checkpoints: [
      {
        id: 'template-completed',
        label: '完成模板事件',
        evaluate: evaluateTemplateCompletion,
      },
    ],
  });
}

/**
 * createTemplateInitialState 生成模板包的可复现初始状态。
 */
function createTemplateInitialState(): DeveloperTemplateState {
  return {
    tick: 0,
    phase: '准备',
    explanation: {
      title: '准备仿真',
      effect: '初始化状态并等待学生推进。',
      reason: '所有仿真都必须从 seed 和参数生成可复现初始状态。',
      defaultDurationMs: 1200,
    },
    metrics: { progress: 0 },
    checkpointValues: { completed: false },
    _trace: { triggeredLines: [1], variables: { progress: 0 } },
  };
}

/**
 * reduceTemplateState 演示如何通过事件生成下一份不可变状态。
 */
function reduceTemplateState(state: DeveloperTemplateState, event: SimEvent): DeveloperTemplateState {
  const completed = event.type === 'complete';
  return {
    ...state,
    tick: event.source === 'tick' ? state.tick + 1 : state.tick,
    phase: completed ? '完成' : state.phase,
    metrics: { progress: completed ? 100 : state.metrics.progress },
    checkpointValues: { completed },
    _trace: { triggeredLines: completed ? [3] : [2], variables: { event: event.type } },
  };
}

/**
 * renderTemplateView 把模板状态映射为单一 pipeline 封闭模式。
 */
function renderTemplateView(state: DeveloperTemplateState): ViewSpec {
  return {
    summary: `当前阶段:${state.phase}`,
    patterns: [
      {
        id: 'template-pipeline',
        mode: 'pipeline',
        title: '模板流程',
        region: 'main',
        data: {
          currentStepId: state.phase === '完成' ? 'done' : 'ready',
          steps: [
            { id: 'ready', label: '准备', status: state.phase === '完成' ? 'complete' : 'running', detail: '生成初始状态' },
            { id: 'done', label: '完成', status: state.phase === '完成' ? 'complete' : 'pending', detail: '响应用户事件' },
          ],
        },
      },
    ],
  };
}

/**
 * evaluateTemplateCompletion 演示检查点如何只读取状态派生结果。
 */
function evaluateTemplateCompletion(state: SimState): CheckpointResult {
  return {
    achieved: state.checkpointValues.completed === true,
    answer: state.checkpointValues.completed,
    explanation: state.checkpointValues.completed === true ? '模板事件已完成。' : '等待执行模板事件。',
  };
}
