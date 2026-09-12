import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { createServerError } from '@/lib/server-error-message'

import { getInterceptorLogStats } from '../api'
import { INTERCEPTOR_LOG_STATS_QUERY_KEY } from '../constants'
import type { InterceptorLogStatRow } from '../types'

/** rangeSeconds 为 0 表示不限时间。 */
export function InterceptorLogStats(props: { rangeSeconds: number }) {
  const { t } = useTranslation()

  const { data, isLoading } = useQuery({
    queryKey: [...INTERCEPTOR_LOG_STATS_QUERY_KEY, props.rangeSeconds],
    queryFn: async () => {
      const start =
        props.rangeSeconds > 0
          ? Math.floor(Date.now() / 1000) - props.rangeSeconds
          : undefined
      const result = await getInterceptorLogStats({ start, limit: 20 })
      if (!result?.success) {
        throw createServerError(result, t('Failed to load interceptor stats'))
      }
      return result.data
    },
  })

  if (isLoading) {
    return <LoadingState />
  }
  if (!data || data.total === 0) {
    return (
      <EmptyState
        title={t('No interceptor logs')}
        description={t(
          'Interceptor logs will appear here when rules modify or reject requests.'
        )}
      />
    )
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 sm:grid-cols-3'>
        <SummaryCard label={t('Total intercepts')} value={data.total} />
        <SummaryCard label={t('Modified')} value={data.modified} />
        <SummaryCard
          label={t('Rejected')}
          value={data.rejected}
          highlight={data.rejected > 0}
        />
      </div>

      <div className='grid gap-4 lg:grid-cols-2'>
        <StatTable title={t('By user')} rows={data.by_user} />
        <StatTable title={t('By rule')} rows={data.by_rule} />
        <StatTable title={t('By model')} rows={data.by_model} />
        <StatTable title={t('By channel')} rows={data.by_channel} />
      </div>
    </div>
  )
}

function SummaryCard(props: {
  label: string
  value: number
  highlight?: boolean
}) {
  return (
    <Card>
      <CardContent className='pt-6'>
        <p className='text-muted-foreground text-sm'>{props.label}</p>
        <p
          className={
            props.highlight
              ? 'text-2xl font-semibold text-rose-600 dark:text-rose-400'
              : 'text-2xl font-semibold'
          }
        >
          {props.value.toLocaleString()}
        </p>
      </CardContent>
    </Card>
  )
}

function StatTable(props: { title: string; rows: InterceptorLogStatRow[] }) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader className='pb-3'>
        <CardTitle className='text-base'>{props.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {props.rows.length === 0 ? (
          <p className='text-muted-foreground text-sm'>{t('No data')}</p>
        ) : (
          <StaticDataTable
            data={props.rows}
            columns={[
              {
                id: 'key',
                header: t('Name'),
                cell: (row) => (
                  <span className='font-mono text-xs'>
                    {row.key || t('(empty)')}
                  </span>
                ),
              },
              {
                id: 'total',
                header: t('Total'),
                className: 'text-right',
                cellClassName: 'text-right tabular-nums',
                cell: (row) => row.total.toLocaleString(),
              },
              {
                id: 'rejected',
                header: t('Rejected'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (row) =>
                  row.rejected > 0 ? (
                    <Badge variant='destructive'>
                      {row.rejected.toLocaleString()}
                    </Badge>
                  ) : (
                    <span className='text-muted-foreground tabular-nums'>
                      0
                    </span>
                  ),
              },
            ]}
          />
        )}
      </CardContent>
    </Card>
  )
}
