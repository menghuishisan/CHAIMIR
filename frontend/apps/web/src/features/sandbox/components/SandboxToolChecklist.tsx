// SandboxToolChecklist 统一编排页里的可用工具多选清单。
// 工具定义来自沙箱编排目录,页面只负责提供当前值与接收更新,不重复维护勾选逻辑。

import type { SandboxCatalogTool } from '@chaimir/api-client'
import { Checkbox } from '@chaimir/ui'

export interface SandboxToolChecklistProps {
  tools: SandboxCatalogTool[]
  selectedCodes: readonly string[]
  onChange: (codes: string[]) => void
}

/** SandboxToolChecklist 渲染工具清单并保持选择顺序。 */
export function SandboxToolChecklist({
  tools,
  selectedCodes,
  onChange,
}: SandboxToolChecklistProps) {
  return (
    <div className="flex flex-col gap-2">
      {tools.map((tool) => (
        <Checkbox
          key={tool.code}
          checked={selectedCodes.includes(tool.code)}
          label={tool.name}
          onCheckedChange={(checked) =>
            onChange(
              checked === true
                ? [...selectedCodes, tool.code]
                : selectedCodes.filter((code) => code !== tool.code),
            )
          }
        />
      ))}
    </div>
  )
}
