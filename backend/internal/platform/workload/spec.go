// workload spec 文件定义跨引擎复用的运行期工作负载声明结构。
package workload

// EnvVarSpec 描述允许注入容器的非敏感字面量环境变量。
type EnvVarSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PortSpec 描述组件容器端口和平台代理暴露口径。
type PortSpec struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"container_port"`
	ServicePort   int32  `json:"service_port"`
	Protocol      string `json:"protocol"`
}

// ResourceSpec 描述组件 requests/limits。
type ResourceSpec struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// ProbeSpec 描述组件声明的探活探针。
type ProbeSpec struct {
	Type             string   `json:"type"`
	Path             string   `json:"path"`
	Port             string   `json:"port"`
	Command          []string `json:"command"`
	PeriodSeconds    int32    `json:"period_seconds"`
	FailureThreshold int32    `json:"failure_threshold"`
}

// EphemeralMountSpec 描述组件在只读根文件系统下需要的临时可写目录。
type EphemeralMountSpec struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

// ComponentSpec 描述一个运行期组件的镜像、启动、安全和探活配置。
type ComponentSpec struct {
	Name                   string               `json:"name"`
	ImageURL               string               `json:"image_url"`
	Command                []string             `json:"command"`
	Args                   []string             `json:"args"`
	Env                    []EnvVarSpec         `json:"env"`
	Ports                  []PortSpec           `json:"ports"`
	Resources              ResourceSpec         `json:"resources"`
	ReadinessProbe         ProbeSpec            `json:"readiness_probe"`
	LivenessProbe          ProbeSpec            `json:"liveness_probe"`
	Workdir                string               `json:"workdir"`
	ReadOnlyRootFilesystem *bool                `json:"read_only_root_filesystem"`
	Labels                 map[string]string    `json:"labels"`
	MountWorkspace         *bool                `json:"mount_workspace"`
	EphemeralMounts        []EphemeralMountSpec `json:"ephemeral_mounts"`
}

// PodSpec 描述一个工作负载 Pod 及其组件组。
type PodSpec struct {
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	Containers []ComponentSpec   `json:"containers"`
}

// NetworkPortRef 描述网络策略放行的目标端口,优先使用端口名称。
type NetworkPortRef struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// NetworkRuleSpec 描述同一工作负载内组件或 Pod 之间显式允许的网络访问。
type NetworkRuleSpec struct {
	Name  string           `json:"name"`
	From  string           `json:"from"`
	To    string           `json:"to"`
	Ports []NetworkPortRef `json:"ports"`
}

// ServicePortSpec 描述组件 ClusterIP Service 端口。
type ServicePortSpec struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
}

// ServiceSpec 描述工作负载内的稳定服务发现入口。
type ServiceSpec struct {
	Name      string            `json:"name"`
	Component string            `json:"component"`
	Ports     []ServicePortSpec `json:"ports"`
}

// RouteSpec 描述平台代理可访问的 Web 工具路由入口。
type RouteSpec struct {
	PathPrefix string `json:"path_prefix"`
	Service    string `json:"service"`
	Port       string `json:"port"`
}
