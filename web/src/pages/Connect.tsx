import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Loader2, LogOut, Plug, ServerCog } from 'lucide-react'
import { api, ApiError } from '@/lib/api'
import { useSession } from '@/lib/session'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ThemeToggle } from '@/components/ThemeToggle'

const RECENT_KEY = 'sftpweb.lastTarget'

interface Recent {
  host: string
  port: number
  username: string
}

function loadRecent(): Recent {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    if (raw) return { port: 22, ...JSON.parse(raw) }
  } catch {
    /* ignore malformed storage */
  }
  return { host: '', port: 22, username: '' }
}

export default function Connect() {
  const navigate = useNavigate()
  const { data: session } = useSession()
  const [form, setForm] = useState(() => ({ ...loadRecent(), password: '' }))
  const [error, setError] = useState<string | null>(null)

  const status = useQuery({ queryKey: ['status'], queryFn: api.status })

  useEffect(() => {
    if (status.data?.connected && status.data.home) {
      navigate(`/files${status.data.home}`, { replace: true })
    }
  }, [status.data, navigate])

  const connect = useMutation({
    mutationFn: () =>
      api.connect({
        host: form.host.trim(),
        port: Number(form.port) || 22,
        username: form.username.trim(),
        password: form.password,
      }),
    onSuccess: (info) => {
      // Host and username are convenience only; the password is never persisted.
      localStorage.setItem(
        RECENT_KEY,
        JSON.stringify({ host: form.host.trim(), port: Number(form.port) || 22, username: form.username.trim() }),
      )
      navigate(`/files${info.home ?? '/'}`, { replace: true })
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Connection failed'),
  })

  const logout = async () => {
    await api.logout()
    window.location.href = '/'
  }

  return (
    <div className="relative flex min-h-screen flex-col">
      <div className="pointer-events-none absolute -top-40 left-1/2 h-[30rem] w-[30rem] -translate-x-1/2 rounded-full bg-primary/10 blur-3xl" />

      <header className="relative flex items-center justify-between border-b px-6 py-3">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <ServerCog className="h-4 w-4 text-primary" />
          Fetchr
        </div>
        <div className="flex items-center gap-1">
          <span className="mr-2 text-sm text-muted-foreground">{session?.user?.name ?? session?.user?.username}</span>
          <ThemeToggle />
          {!session?.skipLogin && (
            <Button variant="ghost" size="icon" onClick={logout} aria-label="Sign out">
              <LogOut />
            </Button>
          )}
        </div>
      </header>

      <main className="relative flex flex-1 items-center justify-center px-6 py-12">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle>Connect to a server</CardTitle>
            <CardDescription>
              Credentials are used to open a session on the server and are never written to disk.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              className="grid gap-4"
              onSubmit={(e) => {
                e.preventDefault()
                setError(null)
                connect.mutate()
              }}
            >
              <div className="grid grid-cols-[1fr_6rem] gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="host">Host</Label>
                  <Input
                    id="host"
                    placeholder="sftp.example.com"
                    autoComplete="off"
                    required
                    value={form.host}
                    onChange={(e) => setForm({ ...form, host: e.target.value })}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="port">Port</Label>
                  <Input
                    id="port"
                    type="number"
                    min={1}
                    max={65535}
                    required
                    value={form.port}
                    onChange={(e) => setForm({ ...form, port: Number(e.target.value) })}
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="username">Username</Label>
                <Input
                  id="username"
                  autoComplete="username"
                  required
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                />
              </div>

              {error && (
                <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </p>
              )}

              <Button type="submit" disabled={connect.isPending}>
                {connect.isPending ? <Loader2 className="animate-spin" /> : <Plug />}
                {connect.isPending ? 'Connecting…' : 'Connect'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
    </div>
  )
}
