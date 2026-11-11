// hooks/index.ts 汇总应用级通用 Hook。业务专属 Hook 留在对应 features 模块内。
// useMediaQuery 住在 @chaimir/ui:导航壳与沉浸态工作台壳都要按断点切形态,同一个订阅不做两份。
export * from './useActiveAnchor'
export * from './useAsyncResource'
export * from './useOnlineStatus'
export * from './usePagedResource'
export * from './usePageLocation'
export { locationLabel, type PageLocationItem } from './pageLocation'
export * from './useResourceTotal'
export * from './useReviewQueue'
export * from './useTicketedWebSocket'
export * from './useUrlState'
export * from './useVisiblePolling'
