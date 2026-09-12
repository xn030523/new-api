import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createInterceptor,
  getRuleTypes,
  updateInterceptor,
} from '../api'
import {
  CUSTOM_RULE_CONFIG_FIELDS,
  INTERCEPTOR_QUERY_KEY,
  RULE_TYPES_QUERY_KEY,
} from '../constants'
import type { Interceptor, InterceptorRule } from '../types'

interface InterceptorEditDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  interceptor: Interceptor | null
}

/** Rule with a UI-only stable id for React keys; stripped before saving */
type EditableRule = InterceptorRule & { _id: string }

// crypto.randomUUID 在非 HTTPS 页面不可用，用计数器生成稳定 id
let ruleIdCounter = 0
function newRuleId(): string {
  ruleIdCounter += 1
  return `rule-${Date.now()}-${ruleIdCounter}`
}

function toEditableRules(parsed: InterceptorRule[]): EditableRule[] {
  return parsed.map((rule) => ({ ...rule, _id: newRuleId() }))
}

function toSerializableRules(editable: EditableRule[]): InterceptorRule[] {
  return editable.map((rule) => {
    const { _id: _ignored, ...rest } = rule
    return rest
  })
}

export function InterceptorEditDialog(props: InterceptorEditDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [channelId, setChannelId] = useState(0)
  const [enabled, setEnabled] = useState(true)
  const [priority, setPriority] = useState(0)
  const [rules, setRules] = useState<EditableRule[]>([])
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerSelected, setPickerSelected] = useState<Set<string>>(new Set())

  const ruleTypesQuery = useQuery({
    queryKey: [...RULE_TYPES_QUERY_KEY],
    queryFn: getRuleTypes,
  })

  useEffect(() => {
    if (props.interceptor) {
      setName(props.interceptor.name)
      setDescription(props.interceptor.description)
      setChannelId(props.interceptor.channel_id)
      setEnabled(props.interceptor.enabled)
      setPriority(props.interceptor.priority)
      try {
        setRules(
          toEditableRules(JSON.parse(props.interceptor.rules) as InterceptorRule[])
        )
      } catch {
        setRules([])
      }
    } else {
      setName('')
      setDescription('')
      setChannelId(0)
      setEnabled(true)
      setPriority(0)
      setRules([])
    }
  }, [props.interceptor, props.open])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const data = {
        ...(props.interceptor ? { id: props.interceptor.id } : {}),
        name,
        description,
        channel_id: channelId,
        enabled,
        priority,
        rules: JSON.stringify(toSerializableRules(rules)),
      }
      if (props.interceptor) {
        await updateInterceptor(data)
      } else {
        await createInterceptor(data)
      }
    },
    onSuccess: () => {
      toast.success(
        props.interceptor ? t('Updated successfully') : t('Created successfully')
      )
      queryClient.invalidateQueries({ queryKey: INTERCEPTOR_QUERY_KEY })
      props.onOpenChange(false)
    },
    onError: (error) => {
      handleServerError(error)
    },
  })

  function addRules(types: string[]) {
    const existingTypes = new Set(rules.map((r) => r.type))
    const toAdd = types.filter((tp) => !existingTypes.has(tp))
    if (toAdd.length === 0) {
      toast.info(t('All selected rules are already added'))
      return
    }
    setRules([
      ...rules,
      ...toAdd.map((type) => ({ type, _id: newRuleId() })),
    ])
    if (toAdd.length < types.length) {
      toast.info(
        t('Added {{count}} new rules (duplicates skipped)', {
          count: toAdd.length,
        })
      )
    } else {
      toast.success(t('Added {{count}} rules', { count: toAdd.length }))
    }
  }

  function openPicker() {
    setPickerSelected(new Set())
    setPickerOpen(true)
  }

  function togglePickerSelection(type: string) {
    setPickerSelected((prev) => {
      const next = new Set(prev)
      if (next.has(type)) {
        next.delete(type)
      } else {
        next.add(type)
      }
      return next
    })
  }

  function confirmPicker() {
    addRules([...pickerSelected])
    setPickerOpen(false)
  }

  function removeRule(index: number) {
    setRules(rules.filter((_, i) => i !== index))
  }

  function updateRuleConfig(index: number, key: string, value: string) {
    const updated = [...rules]
    const rule = { ...updated[index] }
    if (!rule.config) {
      rule.config = {}
    }
    // Try parsing JSON values
    try {
      rule.config[key] = JSON.parse(value) as unknown
    } catch {
      rule.config[key] = value
    }
    updated[index] = rule
    setRules(updated)
  }

  const ruleTypes = ruleTypesQuery.data ?? {}
  const allRuleTypeKeys = Object.keys(ruleTypes)

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {props.interceptor
              ? t('Edit Interceptor')
              : t('Create Interceptor')}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">{t('Name')}</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('Interceptor name')}
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">
                {t('Channel ID')}
              </label>
              <Input
                type="number"
                value={channelId}
                onChange={(e) => setChannelId(Number(e.target.value))}
                placeholder="0 = global"
              />
              <p className="text-xs text-muted-foreground">
                0 = {t('Apply to all channels')}
              </p>
            </div>
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t('Description')}</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('Optional description')}
            />
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <Switch checked={enabled} onCheckedChange={setEnabled} />
              <label className="text-sm">{t('Enabled')}</label>
            </div>
            <div className="flex items-center gap-2">
              <label className="text-sm">{t('Priority')}</label>
              <Input
                type="number"
                value={priority}
                onChange={(e) => setPriority(Number(e.target.value))}
                className="w-20"
              />
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">
                {t('Rules')} ({rules.length})
              </label>
            </div>

            {rules.map((rule, index) => (
              <div
                key={rule._id}
                className="rounded-md border p-3 space-y-2"
              >
                <div className="flex items-center justify-between">
                  <div className="flex flex-col gap-0.5">
                    <span className="text-sm font-medium">
                      {ruleTypes[rule.type] ?? rule.type}
                    </span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {rule.type}
                    </span>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => removeRule(index)}
                    className="text-destructive h-7 px-2"
                  >
                    {t('Remove')}
                  </Button>
                </div>
                {CUSTOM_RULE_CONFIG_FIELDS[rule.type]?.map((field) => (
                  <div key={field.key} className="space-y-1">
                    <label className="text-xs text-muted-foreground">
                      {field.label}
                      {field.required && (
                        <span className="text-destructive"> *</span>
                      )}
                    </label>
                    <Input
                      value={
                        rule.config?.[field.key] != null
                          ? String(rule.config[field.key])
                          : ''
                      }
                      onChange={(e) =>
                        updateRuleConfig(index, field.key, e.target.value)
                      }
                      placeholder={field.placeholder}
                      className="h-8 text-sm"
                    />
                  </div>
                ))}
              </div>
            ))}

            <div className="space-y-1.5">
              <label className="text-xs text-muted-foreground">
                {t('Add rule')}
              </label>
              <Button
                variant="outline"
                className="w-full"
                onClick={openPicker}
              >
                {t('Add rules...')}
              </Button>
            </div>
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">
              {t('Rules JSON')} ({t('advanced')})
            </label>
            <Textarea
              value={JSON.stringify(toSerializableRules(rules), null, 2)}
              onChange={(e) => {
                try {
                  setRules(
                    toEditableRules(
                      JSON.parse(e.target.value) as InterceptorRule[]
                    )
                  )
                } catch {
                  // Invalid JSON, ignore
                }
              }}
              rows={6}
              className="font-mono text-xs"
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending || !name}
          >
            {saveMutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>

      {/* Rule picker dialog */}
      <Dialog open={pickerOpen} onOpenChange={setPickerOpen}>
        <DialogContent className="max-w-lg max-h-[70vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle>{t('Add Rules')}</DialogTitle>
          </DialogHeader>
          <Command className="flex-1 overflow-hidden">
            <CommandInput placeholder={t('Search rules...')} />
            <CommandList>
              <CommandEmpty>{t('No rules found.')}</CommandEmpty>
              <CommandGroup heading={t('Built-in rules')}>
                {allRuleTypeKeys
                  .filter((tp) => !tp.startsWith('custom_'))
                  .map((tp) => (
                    <RulePickerItem
                      key={tp}
                      type={tp}
                      description={ruleTypes[tp] ?? tp}
                      checked={pickerSelected.has(tp)}
                      onToggle={togglePickerSelection}
                    />
                  ))}
              </CommandGroup>
              <CommandSeparator />
              <CommandGroup heading={t('Custom rules')}>
                {allRuleTypeKeys
                  .filter((tp) => tp.startsWith('custom_'))
                  .map((tp) => (
                    <RulePickerItem
                      key={tp}
                      type={tp}
                      description={ruleTypes[tp] ?? tp}
                      checked={pickerSelected.has(tp)}
                      onToggle={togglePickerSelection}
                    />
                  ))}
              </CommandGroup>
            </CommandList>
          </Command>
          <DialogFooter className="pt-2">
            <Button
              variant="outline"
              onClick={() => setPickerOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={confirmPicker}
              disabled={pickerSelected.size === 0}
            >
              {t('Add {{count}} selected', { count: pickerSelected.size })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Dialog>
  )
}

interface RulePickerItemProps {
  type: string
  description: string
  checked: boolean
  onToggle: (type: string) => void
}

function RulePickerItem(props: RulePickerItemProps) {
  const { type, description, checked, onToggle } = props
  return (
    <CommandItem
      value={type}
      onSelect={() => onToggle(type)}
      className="flex items-center gap-2 cursor-pointer"
    >
      <Checkbox
        checked={checked}
        onCheckedChange={() => onToggle(type)}
        aria-label={description}
        className="pointer-events-none"
      />
      <div className="flex flex-col gap-0.5 min-w-0">
        <span className="text-sm">{description}</span>
        <span className="font-mono text-xs text-muted-foreground">{type}</span>
      </div>
    </CommandItem>
  )
}
