import { useTranslation } from 'react-i18next'
import { Check, Copy } from 'lucide-react'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type { InterceptorLog } from '../types'

interface InterceptorLogDetailDialogProps {
  log: InterceptorLog
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function InterceptorLogDetailDialog(props: InterceptorLogDetailDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const log = props.log

  const isRejected = log.action === 'rejected'

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <div className="flex items-center gap-2">
          <span>{t('Interceptor Log Detail')}</span>
          <Badge variant={isRejected ? 'destructive' : 'default'} className="text-xs">
            {isRejected ? t('Rejected') : t('Modified')}
          </Badge>
        </div>
      }
      contentClassName="max-w-2xl max-h-[80vh] overflow-y-auto"
    >
      <div className="space-y-4 py-2">
        {/* Meta info */}
        <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
          <DetailField label={t('Time')} value={log.created_at ? new Date(log.created_at * 1000).toLocaleString() : '-'} />
          <DetailField label={t('Username')} value={log.username || '-'} />
          <DetailField label={t('User ID')} value={String(log.user_id)} />
          <DetailField label={t('Token')} value={log.token_name || '-'} />
          <DetailField label={t('Channel')} value={log.channel_name ? `${log.channel_name} (${log.channel_id})` : String(log.channel_id)} />
          <DetailField label={t('Model')} value={log.model_name || '-'} />
          <DetailField label={t('Rule')} value={log.rule_type} mono />
          <DetailField label={t('Action')} value={isRejected ? t('Rejected') : t('Modified')} />
        </div>

        {/* Reject reason */}
        {isRejected && log.reject_reason && (
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-destructive">{t('Reject Reason')}</label>
            <div className="bg-destructive/5 border-destructive/20 relative overflow-hidden rounded-md border p-2.5">
              <p className="pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap text-destructive">
                {log.reject_reason}
              </p>
            </div>
          </div>
        )}

        {/* Original body */}
        <div className="space-y-1.5">
          <label className="text-xs font-semibold">{t('Original Request Body')}</label>
          <div className="bg-muted/30 relative overflow-hidden rounded-md border p-2.5">
            <CopyButton value={log.request_body} copiedText={copiedText} onCopy={copyToClipboard} />
            <p className="min-w-0 pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap font-mono">
              {log.request_body}
            </p>
          </div>
        </div>

        {/* Modified body (only if different) */}
        {!isRejected && log.modified_body && log.modified_body !== log.request_body && (
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-amber-600">{t('Modified Request Body')}</label>
            <div className="bg-amber-50/30 dark:bg-amber-950/10 border-amber-200 dark:border-amber-800 relative overflow-hidden rounded-md border p-2.5">
              <CopyButton value={log.modified_body} copiedText={copiedText} onCopy={copyToClipboard} />
              <p className="min-w-0 pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap font-mono">
                {log.modified_body}
              </p>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  )
}

function DetailField(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-baseline gap-2 min-w-0">
      <span className="text-xs text-muted-foreground shrink-0 w-16">{props.label}</span>
      <span className={`text-xs truncate ${props.mono ? 'font-mono' : ''}`}>{props.value}</span>
    </div>
  )
}

function CopyButton(props: {
  value: string
  copiedText: string | null
  onCopy: (text: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Button
      variant="ghost"
      size="sm"
      className="absolute top-1.5 right-1.5 h-5 w-5 p-0"
      onClick={() => props.onCopy(props.value)}
      title={t('Copy to clipboard')}
    >
      {props.copiedText === props.value ? (
        <Check className="size-3 text-green-600" />
      ) : (
        <Copy className="size-3" />
      )}
    </Button>
  )
}
