export interface Identity {
  subject: string
  username: string
  email?: string
  name?: string
}

export interface MeResponse {
  authenticated: boolean
  skipLogin: boolean
  user?: Identity
}

export interface ConnectionInfo {
  connected: boolean
  host?: string
  port?: number
  username?: string
  home?: string
}

export interface Entry {
  name: string
  path: string
  size: number
  isDir: boolean
  isLink: boolean
  mode: string
  modTime: string
}

export interface ListResponse {
  path: string
  parent: string
  entries: Entry[]
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message)
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, { credentials: 'same-origin', ...init })
  if (!res.ok) {
    let code = 'error'
    let message = res.statusText
    try {
      const body = await res.json()
      code = body.code ?? code
      message = body.message ?? message
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

function json<T>(path: string, body: unknown, method = 'POST'): Promise<T> {
  return request<T>(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export const api = {
  me: () => request<MeResponse>('/auth/me'),
  logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),

  status: () => request<ConnectionInfo>('/sftp/status'),
  connect: (creds: { host: string; port: number; username: string; password: string }) =>
    json<ConnectionInfo>('/sftp/connect', creds),
  disconnect: () => json<ConnectionInfo>('/sftp/disconnect', {}),

  list: (path: string) => request<ListResponse>(`/sftp/list?path=${encodeURIComponent(path)}`),
  mkdir: (path: string) => json<{ path: string }>('/sftp/mkdir', { path }),
  rename: (from: string, to: string) => json<{ path: string }>('/sftp/rename', { from, to }),
  remove: (path: string) =>
    request<{ removed: string }>(`/sftp/remove?path=${encodeURIComponent(path)}`, { method: 'DELETE' }),

  downloadUrl: (path: string) => `/api/sftp/download?path=${encodeURIComponent(path)}`,
}

/** Uploads one file with progress reporting; XHR is used because fetch cannot report upload progress. */
export function uploadFile(
  dir: string,
  file: File,
  onProgress: (percent: number) => void,
  signal?: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file, file.name)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', `/api/sftp/upload?path=${encodeURIComponent(dir)}`)
    xhr.withCredentials = true

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve()
        return
      }
      let message = `Upload failed (${xhr.status})`
      try {
        message = JSON.parse(xhr.responseText).message ?? message
      } catch {
        /* non-JSON error body */
      }
      reject(new ApiError(xhr.status, 'upload_failed', message))
    }
    xhr.onerror = () => reject(new ApiError(0, 'network_error', 'Network error during upload'))
    xhr.onabort = () => reject(new ApiError(0, 'aborted', 'Upload cancelled'))
    signal?.addEventListener('abort', () => xhr.abort())
    xhr.send(form)
  })
}
