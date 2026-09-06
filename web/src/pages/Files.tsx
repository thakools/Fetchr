import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  ArrowUpDown,
  Download,
  File as FileIcon,
  Folder,
  FolderPlus,
  Link2,
  Loader2,
  LogOut,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Search,
  ServerCog,
  Trash2,
  Unplug,
  Upload as UploadIcon,
} from 'lucide-react'
import { api, ApiError, uploadFile, type Entry } from '@/lib/api'
import { cn, formatBytes, formatDate } from '@/lib/utils'
import { useSession } from '@/lib/session'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useToast } from '@/components/ui/toast'
import { Breadcrumbs } from '@/components/Breadcrumbs'
import { UploadPanel, type Upload } from '@/components/UploadPanel'
import { ThemeToggle } from '@/components/ThemeToggle'

type SortKey = 'name' | 'size' | 'modTime'

export default function Files() {
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()
  const { data: session } = useSession()

  const dir = decodeURIComponent(location.pathname.replace(/^\/files/, '')) || '/'

  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; asc: boolean }>({ key: 'name', asc: true })
  const [uploads, setUploads] = useState<Upload[]>([])
  const [dragging, setDragging] = useState(false)
  const [mkdirOpen, setMkdirOpen] = useState(false)
  const [newFolder, setNewFolder] = useState('')
  const [renaming, setRenaming] = useState<Entry | null>(null)
  const [renameTo, setRenameTo] = useState('')
  const [deleting, setDeleting] = useState<Entry | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  const uploadId = useRef(0)

  const status = useQuery({ queryKey: ['status'], queryFn: api.status })
  const listing = useQuery({
    queryKey: ['list', dir],
    queryFn: () => api.list(dir),
    enabled: status.data?.connected === true,
  })

  useEffect(() => {
    if (status.isSuccess && !status.data.connected) navigate('/connect', { replace: true })
  }, [status.isSuccess, status.data, navigate])


  const refresh = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['list', dir] })
  }, [queryClient, dir])

  const rows = useMemo(() => {
    const entries = listing.data?.entries ?? []
    const needle = filter.trim().toLowerCase()
    const filtered = needle ? entries.filter((e) => e.name.toLowerCase().includes(needle)) : entries
    const dirFirst = (a: Entry, b: Entry) => Number(b.isDir) - Number(a.isDir)
    return [...filtered].sort((a, b) => {
      const byType = dirFirst(a, b)
      if (byType !== 0) return byType
      let cmp = 0
      if (sort.key === 'name') cmp = a.name.localeCompare(b.name, undefined, { numeric: true })
      else if (sort.key === 'size') cmp = a.size - b.size
      else cmp = a.modTime.localeCompare(b.modTime)
      return sort.asc ? cmp : -cmp
    })
  }, [listing.data, filter, sort])

  const startUploads = useCallback(
    async (files: FileList | File[]) => {
      for (const file of Array.from(files)) {
        const id = uploadId.current++
        setUploads((prev) => [...prev, { id, name: file.name, size: file.size, percent: 0, state: 'uploading' }])
        const patch = (u: Partial<Upload>) =>
          setUploads((prev) => prev.map((item) => (item.id === id ? { ...item, ...u } : item)))
        try {
          await uploadFile(dir, file, (percent) => patch({ percent }))
          patch({ percent: 100, state: 'done' })
          refresh()
          setTimeout(() => setUploads((prev) => prev.filter((item) => item.id !== id)), 4000)
        } catch (e) {
          patch({ state: 'error', error: e instanceof ApiError ? e.message : 'Upload failed' })
        }
      }
    },
    [dir, refresh],
  )

  const mkdir = useMutation({
    mutationFn: (name: string) => api.mkdir(`${dir.replace(/\/$/, '')}/${name}`),
    onSuccess: () => {
      setMkdirOpen(false)
      setNewFolder('')
      refresh()
      toast({ title: 'Folder created' })
    },
    onError: (e) => toast({ title: 'Could not create folder', description: e.message, variant: 'destructive' }),
  })

  const rename = useMutation({
    mutationFn: ({ from, to }: { from: string; to: string }) => api.rename(from, to),
    onSuccess: () => {
      setRenaming(null)
      refresh()
      toast({ title: 'Renamed' })
    },
    onError: (e) => toast({ title: 'Could not rename', description: e.message, variant: 'destructive' }),
  })

  const remove = useMutation({
    mutationFn: (path: string) => api.remove(path),
    onSuccess: () => {
      setDeleting(null)
      refresh()
      toast({ title: 'Deleted' })
    },
    onError: (e) => toast({ title: 'Could not delete', description: e.message, variant: 'destructive' }),
  })

  const disconnect = async () => {
    await api.disconnect()
    queryClient.clear()
    navigate('/connect', { replace: true })
  }

  const logout = async () => {
    await api.logout()
    window.location.href = '/'
  }

  const toggleSort = (key: SortKey) => setSort((s) => ({ key, asc: s.key === key ? !s.asc : true }))

  return (
    <div
      className="flex min-h-screen flex-col"
      onDragOver={(e) => {
        e.preventDefault()
        setDragging(true)
      }}
      onDragLeave={(e) => {
        if (e.currentTarget === e.target) setDragging(false)
      }}
      onDrop={(e) => {
        e.preventDefault()
        setDragging(false)
        if (e.dataTransfer.files.length) void startUploads(e.dataTransfer.files)
      }}
    >
      <header className="flex flex-wrap items-center gap-3 border-b px-4 py-3">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <ServerCog className="h-4 w-4 text-primary" />
          FetchR
        </div>
        {status.data?.connected && (
          <span className="rounded-full border px-2.5 py-0.5 text-xs text-muted-foreground">
            {status.data.username}@{status.data.host}:{status.data.port}
          </span>
        )}
        <div className="ml-auto flex items-center gap-1">
          <ThemeToggle />
          <Button variant="ghost" size="icon" onClick={disconnect} aria-label="Disconnect">
            <Unplug />
          </Button>
          {!session?.skipLogin && (
            <Button variant="ghost" size="icon" onClick={logout} aria-label="Sign out">
              <LogOut />
            </Button>
          )}
        </div>
      </header>

      <div className="flex flex-wrap items-center gap-3 border-b px-4 py-2.5">
        <Breadcrumbs path={dir} />
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter"
              className="h-9 w-40 pl-8"
            />
          </div>
          <Button variant="outline" size="sm" onClick={refresh}>
            <RefreshCw className={cn(listing.isFetching && 'animate-spin')} />
          </Button>
          <Button variant="outline" size="sm" onClick={() => setMkdirOpen(true)}>
            <FolderPlus />
            New folder
          </Button>
          <Button size="sm" onClick={() => fileInput.current?.click()}>
            <UploadIcon />
            Upload
          </Button>
          <input
            ref={fileInput}
            type="file"
            multiple
            hidden
            onChange={(e) => {
              if (e.target.files?.length) void startUploads(e.target.files)
              e.target.value = ''
            }}
          />
        </div>
      </div>

      <main className="relative flex-1 px-4 py-4">
        {listing.isPending && (
          <div className="flex items-center justify-center py-24">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        )}

        {listing.isError && (
          <div className="mx-auto mt-16 flex max-w-md flex-col items-center gap-3 rounded-xl border border-destructive/40 bg-destructive/5 p-6 text-center">
            <AlertCircle className="h-6 w-6 text-destructive" />
            <p className="text-sm">{(listing.error as Error).message}</p>
            <Button variant="outline" size="sm" onClick={refresh}>
              Try again
            </Button>
          </div>
        )}

        {listing.isSuccess && (
          <div className="overflow-hidden rounded-xl border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-4 py-2.5 font-medium">
                    <button className="inline-flex items-center gap-1" onClick={() => toggleSort('name')}>
                      Name <ArrowUpDown className="h-3 w-3" />
                    </button>
                  </th>
                  <th className="w-28 px-4 py-2.5 text-right font-medium">
                    <button className="inline-flex items-center gap-1" onClick={() => toggleSort('size')}>
                      Size <ArrowUpDown className="h-3 w-3" />
                    </button>
                  </th>
                  <th className="hidden w-52 px-4 py-2.5 font-medium md:table-cell">
                    <button className="inline-flex items-center gap-1" onClick={() => toggleSort('modTime')}>
                      Modified <ArrowUpDown className="h-3 w-3" />
                    </button>
                  </th>
                  <th className="hidden w-32 px-4 py-2.5 font-medium lg:table-cell">Mode</th>
                  <th className="w-12 px-2 py-2.5" />
                </tr>
              </thead>
              <tbody>
                {dir !== '/' && (
                  <tr className="border-t hover:bg-accent/50">
                    <td colSpan={5} className="px-4 py-2">
                      <Link
                        to={`/files${listing.data.parent}`}
                        className="inline-flex items-center gap-2 text-muted-foreground hover:text-foreground"
                      >
                        <Folder className="h-4 w-4" />
                        ..
                      </Link>
                    </td>
                  </tr>
                )}

                {rows.map((e) => (
                  <tr key={e.path} className="border-t hover:bg-accent/50">
                    <td className="max-w-0 px-4 py-2">
                      {e.isDir ? (
                        <Link to={`/files${e.path}`} className="flex items-center gap-2 truncate font-medium">
                          <Folder className="h-4 w-4 shrink-0 text-primary" />
                          <span className="truncate">{e.name}</span>
                        </Link>
                      ) : (
                        <a
                          href={api.downloadUrl(e.path)}
                          className="flex items-center gap-2 truncate hover:underline"
                          download={e.name}
                        >
                          {e.isLink ? (
                            <Link2 className="h-4 w-4 shrink-0 text-muted-foreground" />
                          ) : (
                            <FileIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                          )}
                          <span className="truncate">{e.name}</span>
                        </a>
                      )}
                    </td>
                    <td className="px-4 py-2 text-right tabular-nums text-muted-foreground">
                      {e.isDir ? '—' : formatBytes(e.size)}
                    </td>
                    <td className="hidden px-4 py-2 text-muted-foreground md:table-cell">{formatDate(e.modTime)}</td>
                    <td className="hidden px-4 py-2 font-mono text-xs text-muted-foreground lg:table-cell">{e.mode}</td>
                    <td className="px-2 py-2 text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Actions for ${e.name}`}>
                            <MoreHorizontal />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          {!e.isDir && (
                            <DropdownMenuItem asChild>
                              <a href={api.downloadUrl(e.path)} download={e.name}>
                                <Download />
                                Download
                              </a>
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem
                            onSelect={() => {
                              setRenaming(e)
                              setRenameTo(e.name)
                            }}
                          >
                            <Pencil />
                            Rename
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem destructive onSelect={() => setDeleting(e)}>
                            <Trash2 />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                ))}

                {rows.length === 0 && (
                  <tr className="border-t">
                    <td colSpan={5} className="px-4 py-16 text-center text-muted-foreground">
                      {filter ? 'Nothing matches that filter.' : 'This folder is empty. Drop files here to upload.'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        {dragging && (
          <div className="pointer-events-none fixed inset-0 z-50 flex items-center justify-center border-4 border-dashed border-primary bg-background/80">
            <p className="flex items-center gap-2 text-lg font-medium">
              <UploadIcon className="h-5 w-5" />
              Drop to upload into {dir}
            </p>
          </div>
        )}
      </main>

      <UploadPanel uploads={uploads} />

      <Dialog open={mkdirOpen} onOpenChange={setMkdirOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New folder</DialogTitle>
            <DialogDescription>Created inside {dir}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="folder">Name</Label>
            <Input id="folder" value={newFolder} autoFocus onChange={(e) => setNewFolder(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setMkdirOpen(false)}>
              Cancel
            </Button>
            <Button disabled={!newFolder.trim() || mkdir.isPending} onClick={() => mkdir.mutate(newFolder.trim())}>
              Create
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={renaming !== null} onOpenChange={(open) => !open && setRenaming(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename</DialogTitle>
            <DialogDescription className="truncate">{renaming?.path}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="rename">New name</Label>
            <Input id="rename" value={renameTo} autoFocus onChange={(e) => setRenameTo(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenaming(null)}>
              Cancel
            </Button>
            <Button
              disabled={!renameTo.trim() || rename.isPending}
              onClick={() =>
                renaming &&
                rename.mutate({ from: renaming.path, to: `${dir.replace(/\/$/, '')}/${renameTo.trim()}` })
              }
            >
              Rename
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete {deleting?.isDir ? 'folder' : 'file'}?</DialogTitle>
            <DialogDescription className="truncate">
              {deleting?.path} will be removed from the server. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={remove.isPending}
              onClick={() => deleting && remove.mutate(deleting.path)}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
