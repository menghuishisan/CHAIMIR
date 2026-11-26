// 文件授权经统一投放接口消费，对象存储地址不进入页面。

import { ApiClient } from '../client'
import type { AttachmentResponse } from '../client'

export class StorageApi {
  constructor(private client: ApiClient) {}

  /** 一次性取件；同一授权再次使用前须重新签发。 */
  async consumeGrant(token: string): Promise<AttachmentResponse> {
    return this.client.getAttachment('/storage/download', { token })
  }

  /** 供图片、音频和视频元素直接读取；凭证由授权令牌和路径受限 Cookie 承载。 */
  streamUrl(token: string): string {
    return this.client.absoluteURL('/storage/download', { token })
  }
}
