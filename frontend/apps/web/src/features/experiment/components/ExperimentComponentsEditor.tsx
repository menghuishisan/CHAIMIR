// ExperimentComponentsEditor 编辑实验环境、仿真、检查点、阶段和协作规则的完整类型化结构。

import React from 'react'
import type { CheckpointConfig, ComponentConfig, EnvComponent, GroupConfig, ParamBinding, SimComponent, StageConfig } from '@chaimir/api-client'
import { Button, Checkbox, Input, Select, Textarea, FormField } from '@chaimir/ui'
import { Plus, Trash2 } from 'lucide-react'
import styles from '../pages/experiment.module.css'

export interface ExperimentComponentsEditorProps {
  components: ComponentConfig
  group: GroupConfig
  onComponentsChange: (value: ComponentConfig) => void
  onGroupChange: (value: GroupConfig) => void
}

/** ExperimentComponentsEditor 按组件类型分区编辑实验编排。 */
export function ExperimentComponentsEditor({ components, group, onComponentsChange, onGroupChange }: ExperimentComponentsEditorProps): React.ReactElement {
  return <div className={styles.componentEditor}>
    <EditorSection title="实验环境" onAdd={() => onComponentsChange({ ...components, envs: [...components.envs, emptyEnv(components.envs.length)] })}>
      {components.envs.map((env, index) => <div className={styles.componentRow} key={`${env.id}-${index}`}>
        <div className={styles.componentRowHeader}><strong>环境 {index + 1}</strong><DeleteButton label={`删除环境 ${index + 1}`} onClick={() => onComponentsChange({ ...components, envs: removeAt(components.envs, index) })} /></div>
        <div className={styles.formGrid}>
          <Field label="组件编号" value={env.id} onChange={(id) => updateEnv(components, index, { ...env, id }, onComponentsChange)} />
          <Field label="运行环境" value={env.runtime_code} onChange={(runtime_code) => updateEnv(components, index, { ...env, runtime_code }, onComponentsChange)} />
          <Field label="环境版本" value={env.runtime_image_version || ''} onChange={(runtime_image_version) => updateEnv(components, index, { ...env, runtime_image_version }, onComponentsChange)} />
          <Field label="辅助工具" value={env.tools.join(', ')} onChange={(value) => updateEnv(components, index, { ...env, tools: splitList(value) }, onComponentsChange)} hint="多个值用逗号分隔" />
          <Field label="初始化代码" value={env.init_code_ref || ''} onChange={(init_code_ref) => updateEnv(components, index, { ...env, init_code_ref }, onComponentsChange)} />
          <Field label="初始化脚本" value={env.init_script_ref || ''} onChange={(init_script_ref) => updateEnv(components, index, { ...env, init_script_ref }, onComponentsChange)} />
          <NumberField label="保留时长（分钟）" value={env.keep_alive_minutes || 0} min={0} onChange={(keep_alive_minutes) => updateEnv(components, index, { ...env, keep_alive_minutes }, onComponentsChange)} />
          <NumberField label="快照保留（分钟）" value={env.snapshot_retention_minutes || 0} min={0} onChange={(snapshot_retention_minutes) => updateEnv(components, index, { ...env, snapshot_retention_minutes }, onComponentsChange)} />
        </div>
        <div className={styles.checkRow}><Checkbox label="保持环境运行" checked={Boolean(env.keep_alive)} onChange={(event) => updateEnv(components, index, { ...env, keep_alive: event.target.checked }, onComponentsChange)} /><Checkbox label="启用快照" checked={Boolean(env.snapshot_enabled)} onChange={(event) => updateEnv(components, index, { ...env, snapshot_enabled: event.target.checked }, onComponentsChange)} /></div>
      </div>)}
    </EditorSection>

    <EditorSection title="仿真组件" onAdd={() => onComponentsChange({ ...components, sims: [...components.sims, emptySim(components.sims.length)] })}>
      {components.sims.map((sim, index) => <div className={styles.componentRow} key={`${sim.id}-${index}`}>
        <div className={styles.componentRowHeader}><strong>仿真 {index + 1}</strong><DeleteButton label={`删除仿真 ${index + 1}`} onClick={() => onComponentsChange({ ...components, sims: removeAt(components.sims, index) })} /></div>
        <div className={styles.formGrid}>
          <Field label="组件编号" value={sim.id} onChange={(id) => updateSim(components, index, { ...sim, id }, onComponentsChange)} />
          <Field label="仿真包" value={sim.package_code} onChange={(package_code) => updateSim(components, index, { ...sim, package_code }, onComponentsChange)} />
          <Field label="版本" value={sim.version} onChange={(version) => updateSim(components, index, { ...sim, version }, onComponentsChange)} />
          <NumberField label="随机种子" value={sim.seed} onChange={(seed) => updateSim(components, index, { ...sim, seed }, onComponentsChange)} />
        </div>
      </div>)}
    </EditorSection>

    <EditorSection title="判分检查点" onAdd={() => onComponentsChange({ ...components, checkpoints: [...components.checkpoints, emptyCheckpoint(components.checkpoints.length)] })}>
      {components.checkpoints.map((checkpoint, index) => <div className={styles.componentRow} key={`${checkpoint.id}-${index}`}>
        <div className={styles.componentRowHeader}><strong>检查点 {index + 1}</strong><DeleteButton label={`删除检查点 ${index + 1}`} onClick={() => onComponentsChange({ ...components, checkpoints: removeAt(components.checkpoints, index) })} /></div>
        <div className={styles.formGrid}>
          <Field label="检查点编号" value={checkpoint.id} onChange={(id) => updateCheckpoint(components, index, { ...checkpoint, id }, onComponentsChange)} />
          <Field label="判题器" value={checkpoint.judger} onChange={(judger) => updateCheckpoint(components, index, { ...checkpoint, judger }, onComponentsChange)} />
          <Field label="题目编号" value={checkpoint.item_code} onChange={(item_code) => updateCheckpoint(components, index, { ...checkpoint, item_code }, onComponentsChange)} />
          <Field label="题目版本" value={checkpoint.item_version} onChange={(item_version) => updateCheckpoint(components, index, { ...checkpoint, item_version }, onComponentsChange)} />
          <NumberField label="分值" value={checkpoint.score} min={1} onChange={(score) => updateCheckpoint(components, index, { ...checkpoint, score }, onComponentsChange)} />
          <Field label="判分模式" value={checkpoint.mode || ''} onChange={(mode) => updateCheckpoint(components, index, { ...checkpoint, mode }, onComponentsChange)} />
          <Field label="关联环境" value={checkpoint.env_id || ''} onChange={(env_id) => updateCheckpoint(components, index, { ...checkpoint, env_id }, onComponentsChange)} />
          <Field label="关联仿真" value={checkpoint.sim_id || ''} onChange={(sim_id) => updateCheckpoint(components, index, { ...checkpoint, sim_id }, onComponentsChange)} />
        </div>
      </div>)}
    </EditorSection>

    <EditorSection title="实验阶段" onAdd={() => onComponentsChange({ ...components, stages: [...components.stages, emptyStage(components.stages.length)] })}>
      {components.stages.map((stage, index) => <StageEditor key={`${stage.stage}-${index}`} stage={stage} index={index} components={components} onChange={onComponentsChange} />)}
    </EditorSection>

    <section className={styles.componentSection}>
      <h3>协作规则</h3>
      <div className={styles.formGrid}>
        <NumberField label="每组人数" value={group.size} min={1} onChange={(size) => onGroupChange({ ...group, size })} />
        <Field label="小组角色" value={group.roles.join(', ')} onChange={(value) => onGroupChange({ ...group, roles: splitList(value) })} hint="多个角色用逗号分隔" />
      </div>
    </section>
  </div>
}

/** StageEditor 编辑阶段、解锁条件和跨组件参数绑定。 */
function StageEditor({ stage, index, components, onChange }: { stage: StageConfig; index: number; components: ComponentConfig; onChange: (value: ComponentConfig) => void }): React.ReactElement {
  const update = (next: StageConfig) => onChange({ ...components, stages: replaceAt(components.stages, index, next) })
  const bindings = stage.param_bindings || []
  return <div className={styles.componentRow}>
    <div className={styles.componentRowHeader}><strong>阶段 {index + 1}</strong><DeleteButton label={`删除阶段 ${index + 1}`} onClick={() => onChange({ ...components, stages: removeAt(components.stages, index) })} /></div>
    <div className={styles.formGrid}>
      <NumberField label="阶段序号" value={stage.stage} min={1} onChange={(value) => update({ ...stage, stage: value })} />
      <Field label="阶段标题" value={stage.title} onChange={(title) => update({ ...stage, title })} />
      <Field label="包含环境" value={(stage.components.envs || []).join(', ')} onChange={(value) => update({ ...stage, components: { ...stage.components, envs: splitList(value) } })} />
      <Field label="包含仿真" value={(stage.components.sims || []).join(', ')} onChange={(value) => update({ ...stage, components: { ...stage.components, sims: splitList(value) } })} />
      <FormField className={styles.field} label="解锁方式"><Select value={stage.unlock_condition?.type || 'manual'} options={[{ value: 'manual', label: '教师手动' }, { value: 'checkpoint', label: '检查点通过' }]} onChange={(type) => update({ ...stage, unlock_condition: type === 'checkpoint' ? { type: 'checkpoint', checkpoint_id: '', min_score: 0 } : { type: 'manual' } })} /></FormField>
      {stage.unlock_condition?.type === 'checkpoint' && <><Field label="解锁检查点" value={stage.unlock_condition.checkpoint_id || ''} onChange={(checkpoint_id) => update({ ...stage, unlock_condition: { ...stage.unlock_condition!, checkpoint_id } })} /><NumberField label="最低分" value={stage.unlock_condition.min_score || 0} min={0} onChange={(min_score) => update({ ...stage, unlock_condition: { ...stage.unlock_condition!, min_score } })} /></>}
    </div>
    <FormField className={styles.field} label="阶段说明"><Textarea rows={3} value={stage.description || ''} onChange={(event) => update({ ...stage, description: event.target.value })} /></FormField>
    <div className={styles.componentRowHeader}><strong>参数绑定</strong><Button type="button" variant="outline" size="sm" icon={<Plus size={14} />} onClick={() => update({ ...stage, param_bindings: [...bindings, emptyBinding()] })}>添加绑定</Button></div>
    {bindings.map((binding, bindingIndex) => <BindingEditor key={bindingIndex} value={binding} index={bindingIndex} onChange={(value) => update({ ...stage, param_bindings: replaceAt(bindings, bindingIndex, value) })} onDelete={() => update({ ...stage, param_bindings: removeAt(bindings, bindingIndex) })} />)}
  </div>
}

/** BindingEditor 编辑一条阶段参数来源映射。 */
function BindingEditor({ value, index, onChange, onDelete }: { value: ParamBinding; index: number; onChange: (value: ParamBinding) => void; onDelete: () => void }): React.ReactElement {
  return <div className={styles.bindingRow}>
    <Field label="目标组件" value={value.target_component} onChange={(target_component) => onChange({ ...value, target_component })} />
    <Field label="目标参数" value={value.target_param} onChange={(target_param) => onChange({ ...value, target_param })} />
    <FormField className={styles.field} label="来源类型"><Select value={value.source_type} options={[{ value: 'checkpoint', label: '检查点输出' }, { value: 'constant', label: '固定值' }]} onChange={(source_type) => onChange({ ...value, source_type: source_type as ParamBinding['source_type'] })} /></FormField>
    {value.source_type === 'checkpoint' ? <><Field label="来源检查点" value={value.source_ref || ''} onChange={(source_ref) => onChange({ ...value, source_ref })} /><Field label="来源字段" value={value.source_path || ''} onChange={(source_path) => onChange({ ...value, source_path })} /></> : <Field label="固定值" value={String(value.constant_value ?? '')} onChange={(constant_value) => onChange({ ...value, constant_value })} />}
    <DeleteButton label={`删除参数绑定 ${index + 1}`} onClick={onDelete} />
  </div>
}

/** EditorSection 提供组件分区标题和添加动作。 */
function EditorSection({ title, onAdd, children }: { title: string; onAdd: () => void; children: React.ReactNode }): React.ReactElement {
  return <section className={styles.componentSection}><div className={styles.componentRowHeader}><h3>{title}</h3><Button type="button" variant="outline" size="sm" icon={<Plus size={14} />} onClick={onAdd}>添加</Button></div>{children}</section>
}

/** Field 渲染统一文本字段。 */
function Field({ label, value, onChange, hint }: { label: string; value: string; onChange: (value: string) => void; hint?: string }): React.ReactElement {
  return <FormField className={styles.field} label={label}><Input fullWidth value={value} placeholder={hint} onChange={(event) => onChange(event.target.value)} /></FormField>
}

/** NumberField 渲染统一数值字段。 */
function NumberField({ label, value, min, onChange }: { label: string; value: number; min?: number; onChange: (value: number) => void }): React.ReactElement {
  return <FormField className={styles.field} label={label}><Input fullWidth type="number" min={min} value={value} onChange={(event) => onChange(Number(event.target.value))} /></FormField>
}

/** DeleteButton 渲染具有可访问名称的删除动作。 */
function DeleteButton({ label, onClick }: { label: string; onClick: () => void }): React.ReactElement {
  return <Button type="button" variant="ghost" size="sm" icon={<Trash2 size={14} />} aria-label={label} onClick={onClick} />
}

/** updateEnv 替换指定环境。 */
function updateEnv(config: ComponentConfig, index: number, value: EnvComponent, onChange: (value: ComponentConfig) => void): void { onChange({ ...config, envs: replaceAt(config.envs, index, value) }) }
/** updateSim 替换指定仿真。 */
function updateSim(config: ComponentConfig, index: number, value: SimComponent, onChange: (value: ComponentConfig) => void): void { onChange({ ...config, sims: replaceAt(config.sims, index, value) }) }
/** updateCheckpoint 替换指定检查点。 */
function updateCheckpoint(config: ComponentConfig, index: number, value: CheckpointConfig, onChange: (value: ComponentConfig) => void): void { onChange({ ...config, checkpoints: replaceAt(config.checkpoints, index, value) }) }
/** replaceAt 不改变其他元素地替换数组项。 */
function replaceAt<T>(items: T[], index: number, value: T): T[] { return items.map((item, itemIndex) => itemIndex === index ? value : item) }
/** removeAt 删除指定数组项。 */
function removeAt<T>(items: T[], index: number): T[] { return items.filter((_, itemIndex) => itemIndex !== index) }
/** splitList 归一化逗号分隔列表。 */
function splitList(value: string): string[] { return value.split(/[,，]/).map((item) => item.trim()).filter(Boolean) }
/** emptyEnv 创建新环境。 */
function emptyEnv(index: number): EnvComponent { return { id: `env-${index + 1}`, runtime_code: '', runtime_image_version: '', tools: [], keep_alive: false, snapshot_enabled: false, keep_alive_minutes: 0, snapshot_retention_minutes: 0 } }
/** emptySim 创建新仿真。 */
function emptySim(index: number): SimComponent { return { id: `sim-${index + 1}`, package_code: '', version: '', seed: 1, params: {} } }
/** emptyCheckpoint 创建新检查点。 */
function emptyCheckpoint(index: number): CheckpointConfig { return { id: `checkpoint-${index + 1}`, judger: '', item_code: '', item_version: '', score: 10, extra_input: {} } }
/** emptyStage 创建新阶段。 */
function emptyStage(index: number): StageConfig { return { stage: index + 1, title: '', description: '', components: { envs: [], sims: [] }, unlock_condition: { type: 'manual' }, param_bindings: [] } }
/** emptyBinding 创建新参数绑定。 */
function emptyBinding(): ParamBinding { return { target_component: '', target_param: '', source_type: 'constant', constant_value: '' } }
