// Workload 类型定义沙箱、工具和判题器共用的声明式容器契约。

export interface WorkloadEnvVar {
  name: string
  value: string
}

export interface WorkloadPort {
  name: string
  container_port: number
  service_port: number
  protocol: 'TCP' | 'UDP'
}

export interface WorkloadResources {
  requests: Record<string, string>
  limits: Record<string, string>
}

export interface WorkloadProbe {
  type?: 'tcp' | 'http' | 'exec'
  path?: string
  port?: string
  command?: string[]
  period_seconds?: number
  failure_threshold?: number
}

export interface WorkloadEphemeralMount {
  name: string
  mount_path: string
}

export interface WorkloadComponent {
  name: string
  image_url?: string
  command?: string[]
  args?: string[]
  env?: WorkloadEnvVar[]
  ports?: WorkloadPort[]
  resources?: WorkloadResources
  readiness_probe?: WorkloadProbe
  liveness_probe?: WorkloadProbe
  workdir?: string
  read_only_root_filesystem?: boolean
  labels?: Record<string, string>
  mount_workspace?: boolean
  ephemeral_mounts?: WorkloadEphemeralMount[]
  prepull_command?: string[]
  prepull_hold?: boolean
}

export interface WorkloadService {
  name: string
  component: string
  ports: Array<{ name: string; port: number; target_port: string; protocol: 'TCP' | 'UDP' }>
}

export interface WorkloadRoute {
  path_prefix: string
  service: string
  port: string
}

export interface WorkloadNetworkRule {
  name: string
  from: string
  to: string
  ports: Array<{ name?: string; port?: number }>
}
