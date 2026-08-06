// 仿真推演分享弹层。
//
// 分享出去的是「场景 + 随机条件 + 操作序列」,任何人打开都能复现同一过程;
// 它不携带账号与学校信息,也不是取运行包的凭据 —— 公开回放只运行平台内置场景
// (见 docs/04-仿真可视化引擎/05-接口设计.md §3.4)。
//
// 两种执行位置共用它:分享的是服务端记录的过程,与这次推演跑在浏览器还是容器里无关。

import { useCallback, useState } from 'react'
import { Link2, Share2 } from 'lucide-react'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  toast,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { userFacingErrorMessage } from '../../utils/userFacingError'

export interface SimShareModalProps {
  sessionId: string
  onClose: () => void
}

/**
 * SimShareModal 为这次推演生成公开分享码。
 */
export function SimShareModal({ sessionId, onClose }: SimShareModalProps) {
  const [expireAt, setExpireAt] = useState('')
  const [code, setCode] = useState<string>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.sim.shareSession(sessionId, {
        expire_at: expireAt ? new Date(expireAt).toISOString() : undefined,
      })
      setCode(result.code)
      toast.success('分享码已生成')
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '分享码没有生成成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [expireAt, sessionId])

  const link = code ? `${window.location.origin}/sim/shared/${code}` : ''

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>分享这次推演</ModalTitle>
          <ModalDescription>
            分享的是这次推演的过程本身:场景、随机条件与你的操作序列。任何人打开都能看到同一过程,
            链接里不含你的账号与学校信息。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {code ? (
            <div className="flex flex-col gap-2 rounded-md border border-line bg-surface-sunken p-3">
              <span className="text-sm text-ink-sub">公开回放链接</span>
              <span className="break-all font-mono text-sm text-ink">{link}</span>
              <Button
                variant="outline"
                size="sm"
                leftIcon={Link2}
                onClick={() => {
                  void navigator.clipboard
                    .writeText(link)
                    .then(() => toast.success('链接已复制'))
                    .catch(() => toast.error('复制没有成功,请手动选中链接。'))
                }}
              >
                复制链接
              </Button>
            </div>
          ) : (
            <label className="flex flex-col gap-1">
              <span className="text-sm text-ink">有效期至</span>
              <Input
                type="datetime-local"
                value={expireAt}
                onChange={(event) => setExpireAt(event.target.value)}
              />
              <span className="text-xs text-ink-sub">留空表示按平台默认有效期。</span>
            </label>
          )}

          {formError ? <p className="text-sm text-danger">{formError}</p> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            {code ? '关闭' : '取消'}
          </Button>
          {code ? null : (
            <Button variant="seal" leftIcon={Share2} loading={working} onClick={() => void submit()}>
              生成分享码
            </Button>
          )}
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
