// AttackDefenseTopology 业务组件：统一展示攻防拓扑，图形和文字清单并存。

import React from 'react'
import { AlertTriangle, CheckCircle2, Shield, Swords, WifiOff } from 'lucide-react'
import { clsx } from 'clsx'
import './AttackDefenseTopology.css'

export type TopologyNodeRole = 'defender' | 'attacker' | 'observer' | 'service'
export type TopologyNodeStatus = 'healthy' | 'risk' | 'blocked' | 'idle'

export interface TopologyNode {
  id: string
  label: string
  role: TopologyNodeRole
  status: TopologyNodeStatus
  detail?: string
}

export interface TopologyEdge {
  id: string
  source: string
  target: string
  label: string
  status?: 'normal' | 'attack' | 'blocked'
}

export interface AttackDefenseTopologyProps extends React.HTMLAttributes<HTMLElement> {
  title: string
  summary?: string
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

const roleMeta: Record<TopologyNodeRole, { label: string; icon: React.ReactElement }> = {
  defender: { label: '防守节点', icon: <Shield size={15} /> },
  attacker: { label: '攻击节点', icon: <Swords size={15} /> },
  observer: { label: '观察节点', icon: <CheckCircle2 size={15} /> },
  service: { label: '服务节点', icon: <CheckCircle2 size={15} /> },
}

const statusMeta: Record<TopologyNodeStatus, { label: string; icon: React.ReactElement }> = {
  healthy: { label: '状态正常', icon: <CheckCircle2 size={15} /> },
  risk: { label: '存在风险', icon: <AlertTriangle size={15} /> },
  blocked: { label: '已阻断', icon: <WifiOff size={15} /> },
  idle: { label: '待观察', icon: <CheckCircle2 size={15} /> },
}

/**
 * AttackDefenseTopology 将节点图和文字清单并列输出，保证颜色之外仍有角色、状态和链路说明。
 */
export function AttackDefenseTopology({ title, summary, nodes, edges, className, ...props }: AttackDefenseTopologyProps): React.ReactElement {
  const layout = layoutNodes(nodes)
  return (
    <section className={clsx('chaimir-topology', className)} aria-label={title} {...props}>
      <header className="chaimir-topology__header">
        <h3>{title}</h3>
        {summary && <p>{summary}</p>}
      </header>
      <div className="chaimir-topology__body">
        <svg viewBox="0 0 360 220" role="img" aria-label={summary ?? title} preserveAspectRatio="xMidYMid meet">
          {edges.map((edge) => {
            const source = layout.get(edge.source)
            const target = layout.get(edge.target)
            if (!source || !target) return null
            return (
              <g key={edge.id} className={`chaimir-topology__edge is-${edge.status ?? 'normal'}`}>
                <line x1={source.x} y1={source.y} x2={target.x} y2={target.y} />
                <text x={(source.x + target.x) / 2} y={(source.y + target.y) / 2 - 6}>{edge.label}</text>
              </g>
            )
          })}
          {nodes.map((node) => {
            const position = layout.get(node.id)
            if (!position) return null
            return (
              <g key={node.id} className={`chaimir-topology__node is-${node.role} is-${node.status}`} tabIndex={0} role="listitem" aria-label={`${node.label}，${roleMeta[node.role].label}，${statusMeta[node.status].label}${node.detail ? `，${node.detail}` : ''}`}>
                <circle cx={position.x} cy={position.y} r="25" />
                <text x={position.x} y={position.y}>{node.label}</text>
              </g>
            )
          })}
        </svg>
        <div className="chaimir-topology__list">
          <h4>节点状态</h4>
          {nodes.map((node) => (
            <article key={node.id} className={`is-${node.status}`}>
              <span aria-hidden="true">{roleMeta[node.role].icon}</span>
              <div>
                <strong>{node.label}</strong>
                <p>{roleMeta[node.role].label}，{statusMeta[node.status].label}{node.detail ? `，${node.detail}` : ''}</p>
              </div>
              <span aria-hidden="true">{statusMeta[node.status].icon}</span>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}

/**
 * layoutNodes 按椭圆轨道分配节点位置，保持拓扑在任意节点数下可读。
 */
function layoutNodes(nodes: TopologyNode[]): Map<string, { x: number; y: number }> {
  const centerX = 180
  const centerY = 110
  const radiusX = 130
  const radiusY = 72
  return nodes.reduce((positions, node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(1, nodes.length) - Math.PI / 2
    positions.set(node.id, {
      x: centerX + Math.cos(angle) * radiusX,
      y: centerY + Math.sin(angle) * radiusY,
    })
    return positions
  }, new Map<string, { x: number; y: number }>())
}
