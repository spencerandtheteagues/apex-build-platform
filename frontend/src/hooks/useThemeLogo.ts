import { useSyncExternalStore } from 'react'

function getLogoSrc(): string {
  return '/apex-build-mark-metal.png'
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
  return useSyncExternalStore(subscribe, getLogoSrc, () => '/apex-build-mark-metal.png')
}
