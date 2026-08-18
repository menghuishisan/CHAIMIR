// ImportActionFooter 统一身份域批量导入在上传、预览和提交阶段的底部操作。

import { CircleCheck, FileSpreadsheet, Upload } from 'lucide-react'
import type { ImportPreviewResponse } from '@chaimir/api-client'
import { Button, ModalFooter } from '@chaimir/ui'

interface ImportActionFooterProps {
  preview?: ImportPreviewResponse
  fileSelected: boolean
  working: boolean
  onClose: () => void
  onReset: () => void
  onPreview: () => void
  onCommit: () => void
}

/** ImportActionFooter 根据是否已有服务端预览切换上传与提交命令。 */
export function ImportActionFooter({
  preview,
  fileSelected,
  working,
  onClose,
  onReset,
  onPreview,
  onCommit,
}: ImportActionFooterProps) {
  return (
    <ModalFooter>
      <Button variant="outline" onClick={onClose}>
        取消
      </Button>
      {preview ? (
        <>
          <Button variant="outline" leftIcon={Upload} onClick={onReset}>
            换个文件
          </Button>
          <Button
            variant="primary"
            leftIcon={CircleCheck}
            loading={working}
            disabled={preview.valid === 0}
            onClick={onCommit}
          >
            提交导入 {preview.valid} 行
          </Button>
        </>
      ) : (
        <Button
          variant="primary"
          leftIcon={FileSpreadsheet}
          loading={working}
          disabled={!fileSelected}
          onClick={onPreview}
        >
          上传并校验
        </Button>
      )}
    </ModalFooter>
  )
}

