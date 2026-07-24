// AdminStatisticsPage 配置学校级运营统计页面。

import React from 'react'
import { api } from '../../../../../app/api'
import { StatisticsPage } from '../../../components/StatisticsPage'

/** loadSchoolStatistics 保持回调引用稳定，并保留 AdminApi 实例上下文。 */
const loadSchoolStatistics = (range: { from: string; to: string }) => api.admin.getSchoolStatistics(range)

/** AdminStatisticsPage 使用统一统计展示能力。 */
export default function AdminStatisticsPage(): React.ReactElement {
  return <StatisticsPage title="深度统计报表" loadingTitle="正在获取统计报表" emptyDescription="当前区间没有可展示的统计快照。" ariaLabel="学校统计快照列表" load={loadSchoolStatistics} />
}
