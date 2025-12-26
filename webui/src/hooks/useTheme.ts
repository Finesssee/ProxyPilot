import { useEffect, useState } from 'react'

type Theme = 'light' | 'dark' | 'system'

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(() => {
    if (typeof window === 'undefined') return 'dark'
    const stored = localStorage.getItem('pp-theme') as Theme | null
    return stored || 'dark'
  })

  useEffect(() => {
    const root = document.documentElement
    const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches

    const effectiveTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

    root.classList.remove('light', 'dark')
    root.classList.add(effectiveTheme)
    localStorage.setItem('pp-theme', theme)
  }, [theme])

  // Listen for system theme changes
  useEffect(() => {
    if (theme !== 'system') return

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => {
      const root = document.documentElement
      root.classList.remove('light', 'dark')
      root.classList.add(e.matches ? 'dark' : 'light')
    }

    mediaQuery.addEventListener('change', handler)
    return () => mediaQuery.removeEventListener('change', handler)
  }, [theme])

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
  }

  return { theme, setTheme }
}
