import { Moon, Sun, Monitor } from 'lucide-react'
import { Button } from './button'
import { useTheme } from '@/hooks/useTheme'

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  const cycleTheme = () => {
    if (theme === 'dark') setTheme('light')
    else if (theme === 'light') setTheme('system')
    else setTheme('dark')
  }

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      onClick={cycleTheme}
      className="text-muted-foreground hover:text-foreground"
      title={`Theme: ${theme}`}
    >
      {theme === 'dark' && <Moon className="h-4 w-4" />}
      {theme === 'light' && <Sun className="h-4 w-4" />}
      {theme === 'system' && <Monitor className="h-4 w-4" />}
    </Button>
  )
}
