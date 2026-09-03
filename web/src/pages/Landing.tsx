import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowRight, FolderTree, Loader2, Lock, ShieldCheck, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useSession } from '@/lib/session'
import { ThemeToggle } from '@/components/ThemeToggle'

const features = [
  { icon: FolderTree, title: 'Browse anywhere', body: 'Navigate remote directories with breadcrumbs, sorting and search.' },
  { icon: Zap, title: 'Streamed transfers', body: 'Uploads and downloads stream straight through; nothing is buffered to disk.' },
  { icon: Lock, title: 'Credentials stay put', body: 'SFTP credentials live only in server memory and are dropped when your session ends.' },
]

export default function Landing() {
  const { data, isPending } = useSession()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const returnTo = params.get('return') ?? '/connect'

  useEffect(() => {
    if (data?.authenticated || data?.skipLogin) navigate(returnTo, { replace: true })
  }, [data, navigate, returnTo])

  if (isPending || data?.authenticated || data?.skipLogin) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="relative min-h-screen overflow-hidden">
      <div className="pointer-events-none absolute -top-40 left-1/2 h-[36rem] w-[36rem] -translate-x-1/2 rounded-full bg-primary/20 blur-3xl" />

      <header className="relative flex items-center justify-between px-6 py-5">
        <div className="flex items-center gap-2 font-semibold">
          <FolderTree className="h-5 w-5 text-primary" />
          SFTP Web
        </div>
        <ThemeToggle />
      </header>

      <main className="relative mx-auto flex max-w-3xl flex-col items-center px-6 pt-20 text-center">
        <div className="mb-6 inline-flex items-center gap-2 rounded-full border bg-card px-3 py-1 text-xs text-muted-foreground">
          <ShieldCheck className="h-3.5 w-3.5 text-primary" />
          Secured with Keycloak
        </div>
        <h1 className="text-balance text-5xl font-bold tracking-tight">A browser for your SFTP servers</h1>
        <p className="mt-5 max-w-xl text-balance text-lg text-muted-foreground">
          Sign in, point it at any SFTP host, and manage remote files from a clean interface. No client install, no
          stored passwords.
        </p>
        <Button size="lg" className="mt-8" asChild>
          <a href={`/api/auth/login?return=${encodeURIComponent(returnTo)}`}>
            Sign in with Keycloak
            <ArrowRight />
          </a>
        </Button>

        <div className="mt-24 grid w-full gap-4 pb-16 text-left sm:grid-cols-3">
          {features.map(({ icon: Icon, title, body }) => (
            <div key={title} className="rounded-xl border bg-card p-5">
              <Icon className="h-5 w-5 text-primary" />
              <h2 className="mt-3 font-medium">{title}</h2>
              <p className="mt-1 text-sm text-muted-foreground">{body}</p>
            </div>
          ))}
        </div>
      </main>
    </div>
  )
}
