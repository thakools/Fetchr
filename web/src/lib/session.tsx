import { createContext, useContext } from 'react'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { api, type MeResponse } from '@/lib/api'

const SessionContext = createContext<UseQueryResult<MeResponse> | null>(null)

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const query = useQuery({ queryKey: ['me'], queryFn: api.me })
  return <SessionContext.Provider value={query}>{children}</SessionContext.Provider>
}

export function useSession() {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession must be used inside SessionProvider')
  return ctx
}
