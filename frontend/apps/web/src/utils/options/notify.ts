// notify 定义公告与告警页面使用的选择项。

import { AlertStatus, AnnouncementScope } from '@chaimir/api-client'
import { alertStatusLabel, announcementScopeLabel } from '../labels'
import { option, withAllOption } from './shared'

export const announcementScopeOptions = [option(AnnouncementScope.TENANT, announcementScopeLabel(AnnouncementScope.TENANT)), option(AnnouncementScope.ROLES, announcementScopeLabel(AnnouncementScope.ROLES))]
export const alertStatusFilterOptions = withAllOption('全部状态', [option(AlertStatus.PENDING, alertStatusLabel(AlertStatus.PENDING)), option(AlertStatus.HANDLED, alertStatusLabel(AlertStatus.HANDLED)), option(AlertStatus.IGNORED, alertStatusLabel(AlertStatus.IGNORED))])
export const alertLevelOptions = [option(1, '一般提醒'), option(2, '重要告警'), option(3, '严重告警')]
