import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { SessionProvider, useSession } from '@/lib/session'
import Landing from '@/pages/Landing'
import Connect from '@/pages/Connect'
import Files from '@/pages/Files'
import { Loader2 } from 'lucide-react'

function Splash() {
  return (
    <div className="flex h-screen items-center justify-center">
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
    </div>
  )
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { data, isPending } = useSession()
  const location = useLocation()
  if (isPending) return <Splash />
  if (!data?.authenticated && !data?.skipLogin) {
    return <Navigate to={`/?return=${encodeURIComponent(location.pathname)}`} replace />
  }
  return <>{children}</>
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route
        path="/connect"
        element={
          <RequireAuth>
            <Connect />
          </RequireAuth>
        }
      />
      <Route
        path="/files/*"
        element={
          <RequireAuth>
            <Files />
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  return (
    <SessionProvider>
      <AppRoutes />
    </SessionProvider>
  )
}
