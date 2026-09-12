import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { Search, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePage,
  DataTableRow,
  useDataTable,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useMediaQuery } from '@/hooks'
import { createServerError } from '@/lib/server-error-message'

import { getInterceptorLogs } from '../api'
import { ACTION_OPTIONS, INTERCEPTOR_LOGS_QUERY_KEY } from '../constants'
import type { InterceptorLog } from '../types'
import { useInterceptorLogsColumns } from './interceptor-logs-columns'

interface Filters {
  username: string
  model_name: string
  rule_type: string
  action: string
}

const EMPTY_FILTERS: Filters = {
  username: '',
  model_name: '',
  rule_type: '',
  action: 'all',
}

export function InterceptorLogDetail(props: { rangeSeconds: number }) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const pageSize = isMobile ? 20 : 50

  const [page, setPage] = useState(1)
  // 输入中的条件与已提交的条件分开，否则每敲一个字都会发请求。
  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS)
  const [applied, setApplied] = useState<Filters>(EMPTY_FILTERS)

  const start =
    props.rangeSeconds > 0
      ? Math.floor(Date.now() / 1000) - props.rangeSeconds
      : undefined

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      ...INTERCEPTOR_LOGS_QUERY_KEY,
      page,
      pageSize,
      props.rangeSeconds,
      applied,
    ],
    queryFn: async () => {
      const result = await getInterceptorLogs({
        page,
        page_size: pageSize,
        start,
        username: applied.username || undefined,
        model_name: applied.model_name || undefined,
        rule_type: applied.rule_type || undefined,
        action: applied.action === 'all' ? undefined : applied.action,
      })
      if (!result?.success) {
        throw createServerError(result, t('Failed to load interceptor logs'))
      }
      return result.data ?? { items: [], total: 0, page: 1 }
    },
  })

  function applyFilters() {
    setPage(1)
    setApplied(draft)
  }

  function resetFilters() {
    setPage(1)
    setDraft(EMPTY_FILTERS)
    setApplied(EMPTY_FILTERS)
  }

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

  const hasFilters =
    applied.username !== '' ||
    applied.model_name !== '' ||
    applied.rule_type !== '' ||
    applied.action !== 'all'

  const toolbar = (
    <div className='flex flex-wrap items-center gap-2'>
      <Input
        className='h-8 w-36'
        placeholder={t('Username')}
        value={draft.username}
        onChange={(e) => setDraft({ ...draft, username: e.target.value })}
        onKeyDown={(e) => {
          if (e.key === 'Enter') applyFilters()
        }}
      />
      <Input
        className='h-8 w-40'
        placeholder={t('Model')}
        value={draft.model_name}
        onChange={(e) => setDraft({ ...draft, model_name: e.target.value })}
        onKeyDown={(e) => {
          if (e.key === 'Enter') applyFilters()
        }}
      />
      <Input
        className='h-8 w-44'
        placeholder={t('Rule type')}
        value={draft.rule_type}
        onChange={(e) => setDraft({ ...draft, rule_type: e.target.value })}
        onKeyDown={(e) => {
          if (e.key === 'Enter') applyFilters()
        }}
      />
      <Select
        value={draft.action}
        onValueChange={(value) =>
          setDraft({ ...draft, action: value ?? 'all' })
        }
      >
        <SelectTrigger className='h-8 w-32'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {ACTION_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {t(option.labelKey)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button size='sm' className='h-8' onClick={applyFilters}>
        <Search className='mr-1 h-4 w-4' aria-hidden='true' />
        {t('Search')}
      </Button>
      {hasFilters && (
        <Button
          size='sm'
          variant='ghost'
          className='h-8'
          onClick={resetFilters}
        >
          <X className='mr-1 h-4 w-4' aria-hidden='true' />
          {t('Reset')}
        </Button>
      )}
    </div>
  )

  return (
    <DataTablePage
      table={table}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoading}
      isFetching={isFetching}
      toolbar={toolbar}
      fixedHeight
      emptyTitle={t('No interceptor logs')}
      emptyDescription={t(
        'Interceptor logs will appear here when rules modify or reject requests.'
      )}
      skeletonKeyPrefix='interceptor-log-skeleton'
      renderRow={(row) => {
        const action = (row.original as unknown as InterceptorLog).action
        return (
          <DataTableRow
            key={row.id}
            row={row}
            className={
              action === 'rejected'
                ? 'bg-rose-50/40 transition-colors dark:bg-rose-950/20'
                : 'transition-colors'
            }
          />
        )
      }}
    />
  )
}
