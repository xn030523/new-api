import { createFileRoute, redirect } from '@tanstack/react-router'

import { InterceptorLogs } from '@/features/interceptor-logs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/interceptor-logs/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: InterceptorLogs,
})
