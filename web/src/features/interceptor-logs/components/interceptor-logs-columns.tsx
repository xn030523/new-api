import type { ColumnDef } from '@tanstack/react-table'
import { Check, Copy, Eye } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type { InterceptorLog } from '../types'
import { InterceptorLogDetailDialog } from './interceptor-log-detail-dialog'

export function useInterceptorLogsColumns(): ColumnDef<InterceptorLog>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const ts = row.original.created_at
        if (!ts) return '-'
        return new Date(ts * 1000).toLocaleString()
      },
    },
    {
      accessorKey: 'username',
      header: t('Username'),
      cell: ({ row }) => row.original.username || '-',
    },
    {
      accessorKey: 'channel_id',
      header: t('Channel'),
      cell: ({ row }) => {
        const log = row.original
        if (log.channel_name) return `${log.channel_name} (${log.channel_id})`
        return log.channel_id || '-'
      },
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: ({ row }) => row.original.model_name || '-',
    },
    {
      accessorKey: 'rule_type',
      header: t('Rule'),
      cell: ({ row }) => (
        <Badge variant='secondary' className='font-mono text-xs'>
          {row.original.rule_type}
        </Badge>
      ),
    },
    {
      accessorKey: 'action',
      header: t('Action'),
      cell: ({ row }) => {
        const action = row.original.action
        if (action === 'rejected') {
          return (
            <Badge variant='destructive' className='text-xs'>
              {t('Rejected')}
            </Badge>
          )
        }
        return (
          <Badge variant='default' className='text-xs'>
            {t('Modified')}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'reject_reason',
      header: t('Reason'),
      cell: ({ row }) => {
        const reason = row.original.reject_reason
        if (!reason) return '-'
        return (
          <span
            className='text-muted-foreground block max-w-[200px] truncate text-xs'
            title={reason}
          >
            {reason}
          </span>
        )
      },
    },
    {
      accessorKey: 'request_body',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const [dialogOpen, setDialogOpen] = useState(false)
        const { t } = useTranslation()
        const { copiedText, copyToClipboard } = useCopyToClipboard({
          notify: false,
        })

        const preview =
          log.request_body.length > 80
            ? `${log.request_body.slice(0, 80)}...`
            : log.request_body

        return (
          <div className='flex items-center gap-1'>
            <button
              type='button'
              className='group text-muted-foreground hover:text-foreground flex max-w-[300px] items-center gap-1 text-left text-xs transition-colors'
              onClick={() => setDialogOpen(true)}
            >
              <span className='truncate font-mono'>{preview}</span>
              <Eye className='size-3 shrink-0 opacity-0 transition-opacity group-hover:opacity-100' />
            </button>
            <Button
              variant='ghost'
              size='sm'
              className='h-5 w-5 shrink-0 p-0'
              onClick={(e) => {
                e.stopPropagation()
                copyToClipboard(log.request_body)
              }}
              title={t('Copy request body')}
            >
              {copiedText === log.request_body ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <InterceptorLogDetailDialog
              log={log}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </div>
        )
      },
    },
  ]
}
