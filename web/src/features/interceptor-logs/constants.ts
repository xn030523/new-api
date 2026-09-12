export const INTERCEPTOR_LOGS_QUERY_KEY = ['interceptor-logs'] as const

export const INTERCEPTOR_LOG_STATS_QUERY_KEY = [
  'interceptor-log-stats',
] as const

/** 统计视图的时间窗口选项，值为秒。 */
export const STATS_RANGES = [
  { value: '3600', labelKey: 'Last hour' },
  { value: '86400', labelKey: 'Last 24 hours' },
  { value: '604800', labelKey: 'Last 7 days' },
  { value: '0', labelKey: 'All time' },
] as const

export const ACTION_OPTIONS = [
  { value: 'all', labelKey: 'All actions' },
  { value: 'modified', labelKey: 'Modified' },
  { value: 'rejected', labelKey: 'Rejected' },
] as const
