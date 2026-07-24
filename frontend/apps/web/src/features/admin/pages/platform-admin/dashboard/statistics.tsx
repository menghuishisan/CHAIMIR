// PlatformStatisticsPage 配置平台级运营统计页面。

import React from 'react'
import { api } from '../../../../../app/api'
import { StatisticsPage } from '../../../components/StatisticsPage'

/** loadPlatformStatistics 保持回调引用稳定，并保留 AdminApi 实例上下文。 */
const loadPlatformStatistics = (range: { from: string; to: string }) => api.admin.getPlatformStatistics(range)

/** PlatformStatisticsPage 使用统一统计展示能力。 */
export default function PlatformStatisticsPage(): React.ReactElement {
  return <StatisticsPage title="平台运营统计" loadingTitle="正在获取平台统计" emptyDescription="当前区间没有可展示的平台统计快照。" ariaLabel="平台统计快照列表" load={loadPlatformStatistics} backPath="/platform-admin/dashboard" />
}
