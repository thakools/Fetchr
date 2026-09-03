import { Link } from 'react-router-dom'
import { ChevronRight, HardDrive } from 'lucide-react'

export function Breadcrumbs({ path }: { path: string }) {
  const segments = path.split('/').filter(Boolean)

  return (
    <nav aria-label="Breadcrumb" className="flex min-w-0 flex-wrap items-center gap-1 text-sm">
      <Link
        to="/files/"
        className="flex items-center gap-1.5 rounded px-1.5 py-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
      >
        <HardDrive className="h-3.5 w-3.5" />
        root
      </Link>
      {segments.map((seg, i) => {
        const to = '/files/' + segments.slice(0, i + 1).join('/')
        const isLast = i === segments.length - 1
        return (
          <span key={to} className="flex min-w-0 items-center gap-1">
            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
            {isLast ? (
              <span className="truncate px-1.5 py-0.5 font-medium">{seg}</span>
            ) : (
              <Link to={to} className="truncate rounded px-1.5 py-0.5 text-muted-foreground hover:bg-accent hover:text-foreground">
                {seg}
              </Link>
            )}
          </span>
        )
      })}
    </nav>
  )
}
