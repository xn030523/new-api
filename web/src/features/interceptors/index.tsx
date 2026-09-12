import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Shield } from 'lucide-react'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { handleServerError } from '@/lib/handle-server-error'

import {
  deleteInterceptor,
  getInterceptors,
  getRuleTypes,
  updateInterceptor,
} from './api'
import { InterceptorEditDialog } from './components/interceptor-edit-dialog'
import { INTERCEPTOR_QUERY_KEY, RULE_TYPES_QUERY_KEY } from './constants'
import type { Interceptor, InterceptorRule } from './types'

export function Interceptors() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [editDialogOpen, setEditDialogOpen] = useState(false)
  const [editingInterceptor, setEditingInterceptor] =
    useState<Interceptor | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)

  const interceptorsQuery = useQuery({
    queryKey: [...INTERCEPTOR_QUERY_KEY],
    queryFn: getInterceptors,
  })

  const ruleTypesQuery = useQuery({
    queryKey: [...RULE_TYPES_QUERY_KEY],
    queryFn: getRuleTypes,
  })

  const ruleTypes = ruleTypesQuery.data ?? {}

  const deleteMutation = useMutation({
    mutationFn: deleteInterceptor,
    onSuccess: () => {
      toast.success(t('Deleted successfully'))
      queryClient.invalidateQueries({ queryKey: INTERCEPTOR_QUERY_KEY })
      setDeleteId(null)
    },
    onError: (error) => {
      handleServerError(error)
    },
  })

  const toggleMutation = useMutation({
    mutationFn: async (item: Interceptor) => {
      await updateInterceptor({
        id: item.id,
        name: item.name,
        channel_id: item.channel_id,
        enabled: !item.enabled,
        priority: item.priority,
        rules: item.rules,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: INTERCEPTOR_QUERY_KEY })
    },
    onError: (error) => {
      handleServerError(error)
    },
  })

  function handleCreate() {
    setEditingInterceptor(null)
    setEditDialogOpen(true)
  }

  function handleEdit(item: Interceptor) {
    setEditingInterceptor(item)
    setEditDialogOpen(true)
  }

  function parseRules(rulesJson: string): InterceptorRule[] {
    try {
      return JSON.parse(rulesJson) as InterceptorRule[]
    } catch {
      return []
    }
  }

  const interceptors = interceptorsQuery.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Shield className="h-5 w-5" />
          <h2 className="text-lg font-semibold">{t('Request Interceptors')}</h2>
        </div>
        <Button onClick={handleCreate} size="sm">
          <Plus className="mr-1 h-4 w-4" />
          {t('Create Interceptor')}
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        {t(
          'Interceptors process and fix request JSON before sending to upstream APIs. Use built-in rules for common fixes or custom rules for new error patterns.'
        )}
      </p>

      {interceptorsQuery.isLoading && (
        <p className="text-sm text-muted-foreground">{t('Loading...')}</p>
      )}

      {interceptors.length === 0 && !interceptorsQuery.isLoading && (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            {t('No interceptors configured yet. Create one to get started.')}
          </CardContent>
        </Card>
      )}

      <div className="grid gap-4">
        {interceptors.map((item) => {
          const rules = parseRules(item.rules)
          return (
            <Card key={item.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Switch
                      checked={item.enabled}
                      onCheckedChange={() => toggleMutation.mutate(item)}
                    />
                    <CardTitle className="text-base">{item.name}</CardTitle>
                    <Badge variant={item.channel_id === 0 ? 'default' : 'outline'}>
                      {item.channel_id === 0
                        ? t('Global')
                        : `Channel #${item.channel_id}`}
                    </Badge>
                    {item.priority > 0 && (
                      <Badge variant="secondary">
                        Priority: {item.priority}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleEdit(item)}
                    >
                      {t('Edit')}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive"
                      onClick={() => setDeleteId(item.id)}
                    >
                      {t('Delete')}
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                {item.description && (
                  <p className="text-sm text-muted-foreground mb-2">
                    {item.description}
                  </p>
                )}
                <div className="flex flex-wrap gap-1.5">
                  {rules.map((rule) => {
                    const ruleKey = `${rule.type}-${JSON.stringify(rule.config ?? {})}`
                    return (
                      <Badge
                        key={ruleKey}
                        variant="secondary"
                        className="text-xs"
                        title={rule.type}
                      >
                        {ruleTypes[rule.type] ?? rule.type}
                        {rule.config &&
                          Object.keys(rule.config).length > 0 &&
                          ` (${Object.entries(rule.config)
                            .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
                            .join(', ')})`}
                      </Badge>
                    )
                  })}
                  {rules.length === 0 && (
                    <span className="text-xs text-muted-foreground">
                      {t('No rules')}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <InterceptorEditDialog
        open={editDialogOpen}
        onOpenChange={setEditDialogOpen}
        interceptor={editingInterceptor}
      />

      <ConfirmDialog
        open={deleteId !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null)
        }}
        title={t('Delete Interceptor')}
        desc={t(
          'Are you sure you want to delete this interceptor? This action cannot be undone.'
        )}
        handleConfirm={() => {
          if (deleteId !== null) {
            deleteMutation.mutate(deleteId)
          }
        }}
        destructive
        isLoading={deleteMutation.isPending}
      />
    </div>
  )
}
