import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ColumnDef } from '@tanstack/react-table'

import {
  DataTablePage,
  DataTableRow,
  useDataTable,
} from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { createServerError } from '@/lib/server-error-message'

import { getInterceptorLogs } from './api'
import { useInterceptorLogsColumns } from './components/interceptor-logs-columns'
import { INTERCEPTOR_LOGS_QUERY_KEY } from './constants'
import type { InterceptorLog } from './types'

export function InterceptorLogs() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [page, setPage] = useState(1)
  const pageSize = isMobile ? 20 : 50

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [...INTERCEPTOR_LOGS_QUERY_KEY, page, pageSize],
    queryFn: async () => {
      const result = await getInterceptorLogs({ page, page_size: pageSize })
      if (!result?.success) {
        throw createServerError(result, t('Failed to load interceptor logs'))
      }
      return result.data ?? { items: [], total: 0, page: 1 }
    },
  })

  const logs = data?.items ?? []
  const columns = useInterceptorLogsColumns()

  const { table } = useDataTable({
    data: logs as unknown as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    enableRowSelection: false,
    manualPagination: true,
    totalCount: data?.total ?? 0,
    pagination: { pageIndex: page - 1, pageSize },
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function'
          ? updater({ pageIndex: page - 1, pageSize })
          : updater
      setPage(next.pageIndex + 1)
    },
  })

  return (
    <DataTablePage
      table={table}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No interceptor logs')}
      emptyDescription={t('Interceptor logs will appear here when rules modify or reject requests.')}
      skeletonKeyPrefix="interceptor-log-skeleton"
      renderRow={(row) => {
        const action = (row.original as unknown as InterceptorLog).action
        return (
          <DataTableRow
            key={row.id}
            row={row}
            className={
              action === 'rejected'
                ? 'bg-rose-50/40 dark:bg-rose-950/20 transition-colors'
                : 'transition-colors'
            }
          />
        )
      }}
    />
  )
}
