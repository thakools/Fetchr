import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import { formatBytes } from '@/lib/utils'

export interface Upload {
  id: number
  name: string
  size: number
  percent: number
  state: 'uploading' | 'done' | 'error'
  error?: string
}

export function UploadPanel({ uploads }: { uploads: Upload[] }) {
  if (uploads.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-40 w-80 space-y-2 rounded-xl border bg-card p-3 shadow-lg">
      <p className="text-xs font-medium text-muted-foreground">Transfers</p>
      {uploads.map((u) => (
        <div key={u.id} className="space-y-1">
          <div className="flex items-center gap-2 text-sm">
            {u.state === 'uploading' && <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-primary" />}
            {u.state === 'done' && <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-primary" />}
            {u.state === 'error' && <XCircle className="h-3.5 w-3.5 shrink-0 text-destructive" />}
            <span className="truncate">{u.name}</span>
            <span className="ml-auto shrink-0 text-xs text-muted-foreground">{formatBytes(u.size)}</span>
          </div>
          {u.state === 'uploading' && <Progress value={u.percent} />}
          {u.state === 'error' && <p className="text-xs text-destructive">{u.error}</p>}
        </div>
      ))}
    </div>
  )
}
