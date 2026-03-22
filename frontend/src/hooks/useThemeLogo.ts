import { useSyncExternalStore } from 'react'

function getLogoSrc(): string {
  if (typeof document === 'undefined') return '/apex-build-logo-transparent.png'
  const theme = document.documentElement.getAttribute('data-ui-theme')
  return theme === 'blue' ? '/logo-blue.png' : '/apex-build-logo-transparent.png'
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
  return useSyncExternalStore(subscribe, getLogoSrc, () => '/apex-build-logo-transparent.png')
}
