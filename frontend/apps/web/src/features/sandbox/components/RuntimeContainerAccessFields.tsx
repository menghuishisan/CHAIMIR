// 运行时主容器和附加组件共用的隔离字段,显式维护学生访问、共享卷权限和临时目录。

import { Button, Checkbox, FormField, Input } from '@chaimir/ui'

export interface RuntimeContainerAccess {
  mountWorkspace: boolean
  studentAccess: boolean
  mountDomains: string[]
  writableMountDomains: string[]
  ephemeralMounts: { name: string; mountPath: string }[]
}

/** readRuntimeContainerAccess 读取真实声明;缺失权限不推断为学生可访问。 */
export function readRuntimeContainerAccess(raw: Record<string, unknown>): RuntimeContainerAccess {
  const labels = raw.labels as Record<string, unknown> | undefined
  const strings = (value: unknown) => Array.isArray(value)
    ? value.filter((part): part is string => typeof part === 'string')
    : []
  return {
    mountWorkspace: raw.mount_workspace === true,
    studentAccess: typeof labels?.['chaimir.io/student-access'] === 'string'
      && labels['chaimir.io/student-access'].trim().toLowerCase() === 'true',
    mountDomains: strings(raw.mount_domains),
    writableMountDomains: strings(raw.writable_mount_domains),
    ephemeralMounts: Array.isArray(raw.ephemeral_mounts) ? raw.ephemeral_mounts.map((value) => {
      const mount = value as Record<string, unknown>
      return {
        name: typeof mount.name === 'string' ? mount.name : '',
        mountPath: typeof mount.mount_path === 'string' ? mount.mount_path : '',
      }
    }) : [],
  }
}

/** runtimeContainerAccessFields 只从本组明确输入写回权限,保留已有其它容器标签。 */
export function runtimeContainerAccessFields(raw: Record<string, unknown>, value: RuntimeContainerAccess) {
  const labels = raw.labels as Record<string, unknown> | undefined
  return {
    labels: { ...labels, 'chaimir.io/student-access': String(value.studentAccess) },
    mount_workspace: value.mountWorkspace,
    mount_domains: value.mountDomains.map((name) => name.trim()),
    writable_mount_domains: value.writableMountDomains.map((name) => name.trim()),
    ephemeral_mounts: value.ephemeralMounts.map((mount) => ({
      name: mount.name.trim(), mount_path: mount.mountPath.trim(),
    })),
  }
}

/** validRuntimeContainerAccess 在提交前拒绝空域、重复域及未挂载的写权限。 */
export function validRuntimeContainerAccess(value: RuntimeContainerAccess): boolean {
  const names = value.mountDomains.map((name) => name.trim())
  const writable = value.writableMountDomains.map((name) => name.trim())
  return names.every(Boolean) && new Set(names).size === names.length
    && writable.every((name) => names.includes(name)) && new Set(writable).size === writable.length
    && !(value.studentAccess && names.includes('judge-private'))
    && value.ephemeralMounts.every((mount) => mount.name.trim() !== '' && mount.mountPath.startsWith('/'))
}

/** RuntimeContainerAccessFields 用同一组字段编辑容器权限,不提供原始声明文本入口。 */
export function RuntimeContainerAccessFields({ label, value, onChange }: {
  label: string
  value: RuntimeContainerAccess
  onChange: (value: RuntimeContainerAccess) => void
}) {
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="text-sm font-medium text-ink">{label}的访问与挂载</legend>
      <div className="flex flex-wrap gap-4">
        <Checkbox label="允许学生进入" checked={value.studentAccess}
          onCheckedChange={(checked) => onChange({ ...value, studentAccess: checked === true })} />
        <Checkbox label="挂载学生工作区" checked={value.mountWorkspace}
          onCheckedChange={(checked) => onChange({ ...value, mountWorkspace: checked === true })} />
      </div>
      <DomainFields label={`${label}挂载的数据域`} values={value.mountDomains}
        onChange={(mountDomains) => onChange({ ...value, mountDomains })} />
      <DomainFields label={`${label}允许写入的数据域`} values={value.writableMountDomains}
        onChange={(writableMountDomains) => onChange({ ...value, writableMountDomains })} />
      <FormField label="容器私有临时目录" helper="脚本执行容器必须独享 /tmp,不能用共享工作区代替。">
        <div className="flex flex-col gap-2">
          {value.ephemeralMounts.map((mount, index) => (
            <div key={index} className="flex flex-wrap gap-2">
              <Input aria-label={`${label}临时目录名称 ${index + 1}`} value={mount.name}
                onChange={(event) => onChange({ ...value, ephemeralMounts: value.ephemeralMounts.map((item, i) =>
                  i === index ? { ...item, name: event.target.value } : item) })} />
              <Input aria-label={`${label}临时目录路径 ${index + 1}`} value={mount.mountPath}
                onChange={(event) => onChange({ ...value, ephemeralMounts: value.ephemeralMounts.map((item, i) =>
                  i === index ? { ...item, mountPath: event.target.value } : item) })} />
              <Button type="button" variant="ghost" size="sm" onClick={() => onChange({ ...value,
                ephemeralMounts: value.ephemeralMounts.filter((_, i) => i !== index) })}>移除临时目录</Button>
            </div>
          ))}
          <Button type="button" variant="outline" size="sm" onClick={() => onChange({ ...value,
            ephemeralMounts: [...value.ephemeralMounts, { name: '', mountPath: '' }] })}>添加临时目录</Button>
        </div>
      </FormField>
    </fieldset>
  )
}

/** DomainFields 逐项维护域名称;写权限与挂载使用同一输入规则。 */
function DomainFields({ label, values, onChange }: {
  label: string; values: string[]; onChange: (values: string[]) => void
}) {
  return (
    <FormField label={label}>
      <div className="flex flex-col gap-2">
        {values.map((value, index) => (
          <div key={index} className="flex gap-2">
            <Input aria-label={`${label} ${index + 1}`} value={value}
              onChange={(event) => onChange(values.map((item, i) => i === index ? event.target.value : item))} />
            <Button type="button" variant="ghost" size="sm"
              onClick={() => onChange(values.filter((_, i) => i !== index))}>移除数据域</Button>
          </div>
        ))}
        <Button type="button" variant="outline" size="sm" onClick={() => onChange([...values, ''])}>添加数据域</Button>
      </div>
    </FormField>
  )
}
