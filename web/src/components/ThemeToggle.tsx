import { useEffect, useState } from 'react'
import { Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'

const KEY = 'fetchr.theme'
const LEGACY_KEY = 'sftpweb.theme'

export function ThemeToggle() {
  const [dark, setDark] = useState(() => (localStorage.getItem(KEY) || localStorage.getItem(LEGACY_KEY)) !== 'light')

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    localStorage.setItem(KEY, dark ? 'dark' : 'light')
  }, [dark])

  return (
    <Button variant="ghost" size="icon" onClick={() => setDark((d) => !d)} aria-label="Toggle theme">
      {dark ? <Sun /> : <Moon />}
    </Button>
  )
}
