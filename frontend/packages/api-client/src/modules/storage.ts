// Storage API 文件定义统一文件服务的前端调用入口。
// 后端 /api/v1/storage/download 是全平台唯一的文件投放出口(见 docs/总-API接口总览.md
// §统一文件服务):各业务模块只签发授权,消费一律经这里,避免每个模块各写一遍取件逻辑。

import { ApiClient } from '../client'
import type { AttachmentResponse } from '../client'

/**
 * StorageApi 封装统一文件服务的两种投放模式。
 * 授权由业务模块签发并在签名内携带 mode:download 一次性取件、stream 流式续播。
 */
export class StorageApi {
  /**
   * constructor 注入统一 API 客户端,复用鉴权与错误信封处理。
   */
  constructor(private client: ApiClient) {}

  /**
   * consumeGrant 消费 mode=download 授权,返回文件内容与后端声明的保存文件名。
   * 授权是一次性的:同一 token 第二次请求会被后端拒绝,调用方需要重新签发。
   */
  async consumeGrant(token: string): Promise<AttachmentResponse> {
    return this.client.getAttachment('/storage/download', { token })
  }

  /**
   * streamUrl 构造 mode=stream 授权的投放地址,供 <video>/<audio> 的 src 使用。
   * 这类请求由浏览器自身发起、拿不到请求拦截器,故需要绝对地址;
   * 鉴权由授权令牌与路径受限 Cookie 承载,页面不拼接对象存储地址。
   */
  streamUrl(token: string): string {
    return this.client.absoluteURL('/storage/download', { token })
  }
}
