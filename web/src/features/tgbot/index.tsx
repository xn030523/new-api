import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, Trash2, Bot, Save } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import { handleServerError } from '@/lib/handle-server-error'

import { getTgBotConfig, updateTgBotConfig, updateTgBotFeature } from './api'
import { TGBOT_QUERY_KEY } from './constants'
import type { TgBotFeature, TgBotTarget } from './types'

let targetKeyCounter = 0
function newTargetKey(): string {
  targetKeyCounter += 1
  return `t-${Date.now()}-${targetKeyCounter}`
}

interface EditableTarget extends TgBotTarget {
  _key: string
}

function toEditable(targets: TgBotTarget[]): EditableTarget[] {
  return targets.map((t) => ({ ...t, _key: newTargetKey() }))
}

function toSerializable(targets: EditableTarget[]): TgBotTarget[] {
  return targets.map(({ _key: _ignored, ...rest }) => rest)
}

export function TgBotSettings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: [...TGBOT_QUERY_KEY],
    queryFn: getTgBotConfig,
  })

  const config = data?.data?.config
  const features = data?.data?.features ?? []
  const running = data?.data?.running ?? false
  const descriptions = data?.data?.descriptions ?? {}

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: TGBOT_QUERY_KEY })

  if (isLoading) {
    return <div className="p-6 text-muted-foreground">{t('Loading...')}</div>
  }

  return (
    <div className="space-y-6 p-6">
      {/* Global config */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Bot className="size-5" />
              <div>
                <CardTitle>{t('Telegram Bot')}</CardTitle>
                <CardDescription>
                  {t('Global bot configuration')}
                </CardDescription>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant={running ? 'default' : 'secondary'}>
                {running ? t('Running') : t('Stopped')}
              </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {config && (
            <GlobalConfigForm
              config={config}
              onSaved={invalidate}
            />
          )}
        </CardContent>
      </Card>

      <Separator />

      {/* Feature cards */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold">{t('Features')}</h2>
        {features.map((feature) => (
          <FeatureCard
            key={feature.id}
            feature={feature}
            description={descriptions[feature.name] ?? feature.name}
            onSaved={invalidate}
          />
        ))}
      </div>
    </div>
  )
}

// --- Global config form ---

function GlobalConfigForm(props: {
  config: { id: number; bot_token: string; enabled: boolean }
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [botToken, setBotToken] = useState(props.config.bot_token)
  const [enabled, setEnabled] = useState(props.config.enabled)

  const saveMutation = useMutation({
    mutationFn: () =>
      updateTgBotConfig({ id: 1, bot_token: botToken, enabled }),
    onSuccess: () => {
      toast.success(t('Saved successfully'))
      props.onSaved()
    },
    onError: handleServerError,
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Switch checked={enabled} onCheckedChange={setEnabled} />
          <label className="text-sm font-medium">{t('Enable Bot')}</label>
        </div>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">{t('Bot Token')}</label>
        <Input
          value={botToken}
          onChange={(e) => setBotToken(e.target.value)}
          placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
          type="password"
        />
        <p className="text-xs text-muted-foreground">
          {t('Get from @BotFather on Telegram')}
        </p>
      </div>
      <Button
        onClick={() => saveMutation.mutate(undefined)}
        disabled={saveMutation.isPending}
        size="sm"
      >
        <Save className="size-3.5 mr-1" />
        {t('Save')}
      </Button>
    </div>
  )
}

// --- Feature card ---

function FeatureCard(props: {
  feature: TgBotFeature
  description: string
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const feature = props.feature

  let targets: TgBotTarget[] = []
  try {
    targets = JSON.parse(feature.targets || '[]')
  } catch {
    targets = []
  }

  const [enabled, setEnabled] = useState(feature.enabled)
  const [targetList, setTargetList] = useState<EditableTarget[]>(() => toEditable(targets))
  const [settingsJson, setSettingsJson] = useState(feature.settings || '{}')
  const [showSettings, setShowSettings] = useState(false)

  const saveMutation = useMutation({
    mutationFn: () =>
      updateTgBotFeature({
        name: feature.name,
        enabled,
        targets: JSON.stringify(toSerializable(targetList)),
        settings: settingsJson,
      }),
    onSuccess: () => {
      toast.success(t('Saved successfully'))
      props.onSaved()
    },
    onError: handleServerError,
  })

  function addTarget() {
    setTargetList([...targetList, { chat_id: 0, thread_id: 0, enabled: true, _key: newTargetKey() }])
  }

  function removeTarget(key: string) {
    setTargetList(targetList.filter((t) => t._key !== key))
  }

  function updateTarget(key: string, field: keyof TgBotTarget, value: unknown) {
    setTargetList(targetList.map((t) => t._key === key ? { ...t, [field]: value } : t))
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">
              {props.description}
            </CardTitle>
            <CardDescription className="font-mono text-xs">
              {feature.name}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} />
            <span className="text-sm">{enabled ? t('Enabled') : t('Disabled')}</span>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Targets */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">{t('Push Targets')}</label>
            <Button variant="outline" size="sm" onClick={addTarget}>
              <Plus className="size-3.5 mr-1" />
              {t('Add Target')}
            </Button>
          </div>
          {targetList.length === 0 && (
            <p className="text-xs text-muted-foreground">
              {t('No targets configured. Add a chat/topic to enable this feature.')}
            </p>
          )}
          {targetList.map((target) => (
            <div key={target._key} className="flex items-center gap-2 rounded-md border p-2">
              <Switch
                checked={target.enabled}
                onCheckedChange={(v) => updateTarget(target._key, 'enabled', v)}
              />
              <div className="flex-1 grid grid-cols-2 gap-2">
                <Input
                  type="number"
                  placeholder={t('Chat ID (e.g. -100123456)')}
                  value={target.chat_id || ''}
                  onChange={(e) => updateTarget(target._key, 'chat_id', Number(e.target.value))}
                  className="h-8 text-sm"
                />
                <Input
                  type="number"
                  placeholder={t('Thread ID (0 = no topic)')}
                  value={target.thread_id || ''}
                  onChange={(e) => updateTarget(target._key, 'thread_id', Number(e.target.value))}
                  className="h-8 text-sm"
                />
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => removeTarget(target._key)}
                className="text-destructive h-7 px-2"
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </div>

        {/* Settings */}
        <div className="space-y-1.5">
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            onClick={() => setShowSettings(!showSettings)}
          >
            {showSettings ? '▼' : '▶'} {t('Advanced Settings (JSON)')}
          </button>
          {showSettings && (
            <Textarea
              value={settingsJson}
              onChange={(e) => setSettingsJson(e.target.value)}
              rows={5}
              className="font-mono text-xs"
            />
          )}
        </div>

        <Button
          onClick={() => saveMutation.mutate(undefined)}
          disabled={saveMutation.isPending}
          size="sm"
        >
          <Save className="size-3.5 mr-1" />
          {t('Save')}
        </Button>
      </CardContent>
    </Card>
  )
}
