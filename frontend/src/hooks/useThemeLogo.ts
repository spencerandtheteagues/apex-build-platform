import { useSyncExternalStore } from 'react'

function getLogoSrc(): string {
  if (typeof document === 'undefined') return '/logo.png'
  const theme = document.documentElement.getAttribute('data-ui-theme')
  return theme === 'blue' ? '/logo-blue.png' : '/logo.png'
}

function subscribe(callback: () => void): () => void {
  const observer = new MutationObserver(callback)
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-ui-theme'],
  })
  return () => observer.disconnect()
}

/** Returns the correct logo path for the current UI color scheme. */
export function useThemeLogo(): string {
  return useSyncExternalStore(subscribe, getLogoSrc, () => '/logo.png')
}
