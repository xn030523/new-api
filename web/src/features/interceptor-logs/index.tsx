import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { InterceptorLogDetail } from './components/interceptor-log-detail'
import { InterceptorLogStats } from './components/interceptor-log-stats'
import { STATS_RANGES } from './constants'

export function InterceptorLogs() {
  const { t } = useTranslation()
  const [view, setView] = useState('stats')
  const [range, setRange] = useState('86400')

  const rangeSeconds = Number(range)

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Interceptor Logs')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Select
          value={range}
          onValueChange={(value) => setRange(value ?? '86400')}
        >
          <SelectTrigger className='h-8 w-36'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {STATS_RANGES.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {t(option.labelKey)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Tabs value={view} onValueChange={setView}>
          <TabsList>
            <TabsTrigger value='stats'>{t('Statistics')}</TabsTrigger>
            <TabsTrigger value='detail'>{t('Details')}</TabsTrigger>
          </TabsList>
        </Tabs>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col'>
          {view === 'stats' ? (
            <div className='min-h-0 flex-1 overflow-auto'>
              <InterceptorLogStats rangeSeconds={rangeSeconds} />
            </div>
          ) : (
            <div className='min-h-0 flex-1'>
              <InterceptorLogDetail rangeSeconds={rangeSeconds} />
            </div>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
