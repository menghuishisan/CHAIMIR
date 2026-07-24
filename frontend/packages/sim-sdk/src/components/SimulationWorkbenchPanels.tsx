// 本文件渲染仿真工作台的教学说明、指标与时间轴面板。

import React from 'react';
import { AlertTriangle, Clock3 } from 'lucide-react';
import type { NarrativeStepDescriptor, RuntimeSnapshot, SimPackageDescriptor } from '../types';
import './SimulationWorkbenchPanels.css';

/**
 * ProtocolIssuePanel 在仿真包未声明主画面时给出可恢复的用户向提示,避免静默错画。
 */
export function ProtocolIssuePanel(): React.ReactElement {
  return (
    <section className="sim-protocol-issue" role="alert">
      <AlertTriangle size={22} />
      <div>
        <h2>暂时无法展示仿真画面</h2>
        <p>这个仿真包缺少主要画面配置。请联系管理员检查仿真包配置后再试。</p>
      </div>
    </section>
  );
}
/**
 * InspectorSection 提供右侧说明区的原生折叠分组,让补充信息按需展开且键盘可达。
 */
export function InspectorSection({
  title,
  summary,
  defaultOpen = false,
  children,
}: {
  title: string;
  summary: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}): React.ReactElement {
  return (
    <details className="sim-inspector-section" open={defaultOpen}>
      <summary>
        <span>{title}</span>
        <small>{summary}</small>
      </summary>
      <div className="sim-inspector-section__body">{children}</div>
    </details>
  );
}

/**
 * LearningGoalPanel 展示当前仿真包自己的教学目标,避免所有工作台只呈现统一壳层。
 */
export function LearningGoalPanel({ descriptor }: { descriptor: SimPackageDescriptor }): React.ReactElement {
  return (
    <section className="sim-side-section sim-learning">
      <h2>学习目标</h2>
      <p>{descriptor.meta.summary}</p>
      <ul>
        {descriptor.meta.learningObjectives.map((objective) => (
          <li key={objective}>{objective}</li>
        ))}
      </ul>
    </section>
  );
}

/**
 * MetricPanel 把仿真状态中的关键指标显式展示,让不同机制的运行差异可扫描。
 */
export function MetricPanel({ metrics }: { metrics: RuntimeSnapshot['state']['metrics'] }): React.ReactElement | null {
  const entries = Object.entries(metrics).slice(0, 12);
  if (!entries.length) {
    return null;
  }
  return (
    <section className="sim-side-section sim-metrics">
      <h2>状态指标</h2>
      <dl>
        {entries.map(([key, value]) => (
          <div key={key}>
            <dt>{metricLabel(key)}</dt>
            <dd>{String(value)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/**
 * metricLabel 将常见内部指标名转成用户向文案,未知指标只做安全格式化。
 */
function metricLabel(key: string): string {
  const labels: Record<string, string> = {
    accountNonce: '账户 Nonce',
    activeStake: '活跃权益',
    attackerProfit: '攻击收益',
    attestedStake: '见证权益',
    challenge: '挑战值',
    commitIndex: '提交位置',
    coverage: '覆盖率',
    difficulty: '难度',
    dirty: '变更账户',
    entries: '条目数',
    failedCases: '失败用例',
    finalizedEpoch: '最终周期',
    progress: '进度',
    height: '高度',
    hops: '跳数',
    invalidCount: '异常数量',
    leaves: '叶子数量',
    latency: '延迟',
    nonce: 'Nonce',
    pathLength: '路径长度',
    quorum: '法定人数',
    result: '结果',
    risk: '风险',
    round: '轮次',
    shortlistSize: '候选列表',
    term: '任期',
    throughput: '吞吐',
    ts: '时间',
    validShares: '有效份额',
    validSignatures: '有效签名',
    vaultBalance: '金库余额',
    versionGap: '版本差距',
    view: '视图',
    votes: '投票数',
    work: '工作量',
    finalized: '已最终确认',
    committed: '已提交',
    failed: '失败次数',
    gasLeft: '剩余 Gas',
    gasUsed: '已用 Gas',
    balance: '余额',
    confirmations: '确认数',
  };
  if (labels[key]) {
    return labels[key];
  }
  return '指标';
}

/**
 * TimelinePanel 展示所有仿真共享的逻辑时间、事件进度和播放参数。
 */
export function TimelinePanel({
  descriptor,
  snapshot,
  stepDuration,
  speed,
}: {
  descriptor: SimPackageDescriptor;
  snapshot: RuntimeSnapshot;
  stepDuration: number;
  speed: number;
}): React.ReactElement {
  const maxTick = Math.max(1, descriptor.meta.scaleLimit.maxTick);
  const progress = Math.min(100, Math.round((snapshot.tick / maxTick) * 100));
  const latestEvent = snapshot.events[snapshot.events.length - 1];
  return (
    <section className="sim-timeline" aria-label="仿真时间轴">
      <header>
        <Clock3 size={16} />
        <strong>时间轴</strong>
        <span>{progress}%</span>
      </header>
      <div className="sim-timeline__track" aria-hidden="true">
        <span style={{ transform: `scaleX(${progress / 100})` }} />
      </div>
      <dl>
        <div>
          <dt>当前步进</dt>
          <dd>
            {snapshot.tick}/{maxTick}
          </dd>
        </div>
        <div>
          <dt>当前阶段</dt>
          <dd>{snapshot.state.phase}</dd>
        </div>
        <div>
          <dt>事件数量</dt>
          <dd>
            {snapshot.events.length}/{descriptor.meta.scaleLimit.maxEvents}
          </dd>
        </div>
        <div>
          <dt>最近事件</dt>
          <dd>{latestEvent ? latestEvent.type : '等待开始'}</dd>
        </div>
        <div>
          <dt>播放参数</dt>
          <dd>
            {stepDuration}ms / {speed}x
          </dd>
        </div>
      </dl>
    </section>
  );
}

/**
 * 展示叙事步骤列表,帮助学生知道当前处于哪一个教学阶段。
 */
export function StepList({ steps, currentStep }: { steps: NarrativeStepDescriptor[]; currentStep?: NarrativeStepDescriptor }): React.ReactElement {
  return (
    <ol className="sim-step-list" aria-label="教学步骤">
      {steps.map((step) => (
        <li className={step.id === currentStep?.id ? 'is-current' : undefined} key={step.id}>
          <span>{step.title}</span>
          <small>{step.defaultDurationMs}ms</small>
        </li>
      ))}
    </ol>
  );
}
