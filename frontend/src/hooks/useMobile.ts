// APEX.BUILD Mobile Detection and Responsiveness Hooks
// Touch-optimized utilities for mobile devices

import { useState, useEffect, useCallback, useRef } from 'react'

// Breakpoints matching Tailwind defaults
export const BREAKPOINTS = {
  xs: 0,
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  '2xl': 1536,
} as const

export type Breakpoint = keyof typeof BREAKPOINTS

// Hook to detect current breakpoint
export function useBreakpoint(): Breakpoint {
  const [breakpoint, setBreakpoint] = useState<Breakpoint>('lg')

  useEffect(() => {
    const updateBreakpoint = () => {
      const width = window.innerWidth
      if (width < BREAKPOINTS.sm) setBreakpoint('xs')
      else if (width < BREAKPOINTS.md) setBreakpoint('sm')
      else if (width < BREAKPOINTS.lg) setBreakpoint('md')
      else if (width < BREAKPOINTS.xl) setBreakpoint('lg')
      else if (width < BREAKPOINTS['2xl']) setBreakpoint('xl')
      else setBreakpoint('2xl')
    }

    updateBreakpoint()
    window.addEventListener('resize', updateBreakpoint)
    return () => window.removeEventListener('resize', updateBreakpoint)
  }, [])

  return breakpoint
}

// Hook to check if device is mobile
export function useIsMobile(): boolean {
  const breakpoint = useBreakpoint()
  return breakpoint === 'xs' || breakpoint === 'sm'
}

// Hook to check if device is tablet
export function useIsTablet(): boolean {
  const breakpoint = useBreakpoint()
  return breakpoint === 'md'
}

// Hook to check if device is desktop
export function useIsDesktop(): boolean {
  const breakpoint = useBreakpoint()
  return breakpoint === 'lg' || breakpoint === 'xl' || breakpoint === '2xl'
}

// Hook to detect touch device
export function useIsTouchDevice(): boolean {
  const [isTouch, setIsTouch] = useState(false)

  useEffect(() => {
    const checkTouch = () => {
      setIsTouch(
        'ontouchstart' in window ||
        navigator.maxTouchPoints > 0 ||
        // @ts-ignore
        navigator.msMaxTouchPoints > 0
      )
    }
    checkTouch()
  }, [])

  return isTouch
}

// Hook to detect device orientation
export function useOrientation(): 'portrait' | 'landscape' {
  const [orientation, setOrientation] = useState<'portrait' | 'landscape'>('portrait')

  useEffect(() => {
    const updateOrientation = () => {
      if (window.matchMedia('(orientation: portrait)').matches) {
        setOrientation('portrait')
      } else {
        setOrientation('landscape')
      }
    }

    updateOrientation()
    window.addEventListener('resize', updateOrientation)
    window.addEventListener('orientationchange', updateOrientation)

    return () => {
      window.removeEventListener('resize', updateOrientation)
      window.removeEventListener('orientationchange', updateOrientation)
    }
  }, [])

  return orientation
}

// Hook to detect reduced motion preference
export function useReducedMotion(): boolean {
  const [reducedMotion, setReducedMotion] = useState(false)

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    setReducedMotion(mediaQuery.matches)

    const handler = (e: MediaQueryListEvent) => setReducedMotion(e.matches)
    mediaQuery.addEventListener('change', handler)
    return () => mediaQuery.removeEventListener('change', handler)
  }, [])

  return reducedMotion
}

// Hook to detect low power mode (battery saver)
export function useLowPowerMode(): boolean {
  const [lowPower, setLowPower] = useState(false)

  useEffect(() => {
    // Check battery API
    if ('getBattery' in navigator) {
      // @ts-ignore
      navigator.getBattery().then((battery: any) => {
        const checkLowPower = () => {
          // Consider low power if battery < 20% and not charging
          setLowPower(battery.level < 0.2 && !battery.charging)
        }
        checkLowPower()
        battery.addEventListener('levelchange', checkLowPower)
        battery.addEventListener('chargingchange', checkLowPower)
      })
    }
  }, [])

  return lowPower
}

// Hook for swipe gestures
export interface SwipeHandlers {
  onSwipeLeft?: () => void
  onSwipeRight?: () => void
  onSwipeUp?: () => void
  onSwipeDown?: () => void
}

export function useSwipeGesture(
  elementRef: React.RefObject<HTMLElement>,
  handlers: SwipeHandlers,
  threshold: number = 50
) {
  const touchStart = useRef<{ x: number; y: number } | null>(null)

  useEffect(() => {
    const element = elementRef.current
    if (!element) return

    const handleTouchStart = (e: TouchEvent) => {
      touchStart.current = {
        x: e.touches[0].clientX,
        y: e.touches[0].clientY,
      }
    }

    const handleTouchEnd = (e: TouchEvent) => {
      if (!touchStart.current) return

      const deltaX = e.changedTouches[0].clientX - touchStart.current.x
      const deltaY = e.changedTouches[0].clientY - touchStart.current.y

      const absDeltaX = Math.abs(deltaX)
      const absDeltaY = Math.abs(deltaY)

      // Determine if horizontal or vertical swipe
      if (absDeltaX > absDeltaY && absDeltaX > threshold) {
        if (deltaX > 0) {
          handlers.onSwipeRight?.()
        } else {
          handlers.onSwipeLeft?.()
        }
      } else if (absDeltaY > absDeltaX && absDeltaY > threshold) {
        if (deltaY > 0) {
          handlers.onSwipeDown?.()
        } else {
          handlers.onSwipeUp?.()
        }
      }

      touchStart.current = null
    }

    element.addEventListener('touchstart', handleTouchStart, { passive: true })
    element.addEventListener('touchend', handleTouchEnd, { passive: true })

    return () => {
      element.removeEventListener('touchstart', handleTouchStart)
      element.removeEventListener('touchend', handleTouchEnd)
    }
  }, [elementRef, handlers, threshold])
}

// Hook for pull-to-refresh
export function usePullToRefresh(
  elementRef: React.RefObject<HTMLElement>,
  onRefresh: () => Promise<void>,
  threshold: number = 80
) {
  const [isPulling, setIsPulling] = useState(false)
  const [pullDistance, setPullDistance] = useState(0)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const touchStart = useRef<number | null>(null)

  useEffect(() => {
    const element = elementRef.current
    if (!element) return

    const handleTouchStart = (e: TouchEvent) => {
      if (element.scrollTop === 0) {
        touchStart.current = e.touches[0].clientY
        setIsPulling(true)
      }
    }

    const handleTouchMove = (e: TouchEvent) => {
      if (touchStart.current === null || isRefreshing) return

      const currentY = e.touches[0].clientY
      const distance = Math.max(0, currentY - touchStart.current)

      // Apply resistance
      const resistedDistance = Math.min(distance * 0.5, threshold * 1.5)
      setPullDistance(resistedDistance)

      if (resistedDistance > 0) {
        e.preventDefault()
      }
    }

    const handleTouchEnd = async () => {
      if (pullDistance >= threshold && !isRefreshing) {
        setIsRefreshing(true)
        await onRefresh()
        setIsRefreshing(false)
      }

      setIsPulling(false)
      setPullDistance(0)
      touchStart.current = null
    }

    element.addEventListener('touchstart', handleTouchStart, { passive: true })
    element.addEventListener('touchmove', handleTouchMove, { passive: false })
    element.addEventListener('touchend', handleTouchEnd, { passive: true })

    return () => {
      element.removeEventListener('touchstart', handleTouchStart)
      element.removeEventListener('touchmove', handleTouchMove)
      element.removeEventListener('touchend', handleTouchEnd)
    }
  }, [elementRef, onRefresh, threshold, isRefreshing, pullDistance])

  return { isPulling, pullDistance, isRefreshing }
}

// Hook for pinch-to-zoom
export function usePinchZoom(
  elementRef: React.RefObject<HTMLElement>,
  options: {
    minScale?: number
    maxScale?: number
    initialScale?: number
    onScaleChange?: (scale: number) => void
  } = {}
) {
  const { minScale = 0.5, maxScale = 3, initialScale = 1, onScaleChange } = options
  const [scale, setScale] = useState(initialScale)
  const initialDistance = useRef<number | null>(null)
  const initialScale2 = useRef<number>(initialScale)

  useEffect(() => {
    const element = elementRef.current
    if (!element) return

    const getDistance = (touches: TouchList): number => {
      const [t1, t2] = [touches[0], touches[1]]
      return Math.hypot(t2.clientX - t1.clientX, t2.clientY - t1.clientY)
    }

    const handleTouchStart = (e: TouchEvent) => {
      if (e.touches.length === 2) {
        initialDistance.current = getDistance(e.touches)
        initialScale2.current = scale
      }
    }

    const handleTouchMove = (e: TouchEvent) => {
      if (e.touches.length !== 2 || initialDistance.current === null) return

      const currentDistance = getDistance(e.touches)
      const scaleChange = currentDistance / initialDistance.current
      const newScale = Math.min(maxScale, Math.max(minScale, initialScale2.current * scaleChange))

      setScale(newScale)
      onScaleChange?.(newScale)
      e.preventDefault()
    }

    const handleTouchEnd = () => {
      initialDistance.current = null
    }

    element.addEventListener('touchstart', handleTouchStart, { passive: true })
    element.addEventListener('touchmove', handleTouchMove, { passive: false })
    element.addEventListener('touchend', handleTouchEnd, { passive: true })

    return () => {
      element.removeEventListener('touchstart', handleTouchStart)
      element.removeEventListener('touchmove', handleTouchMove)
      element.removeEventListener('touchend', handleTouchEnd)
    }
  }, [elementRef, minScale, maxScale, scale, onScaleChange])

  const resetScale = useCallback(() => {
    setScale(initialScale)
    onScaleChange?.(initialScale)
  }, [initialScale, onScaleChange])

  return { scale, resetScale }
}

// Hook for viewport height (handles mobile browser chrome)
export function useViewportHeight(): number {
  const [vh, setVh] = useState(window.innerHeight)

  useEffect(() => {
    const updateVh = () => {
      setVh(window.innerHeight)
      // Set CSS variable for use in styles
      document.documentElement.style.setProperty('--vh', `${window.innerHeight * 0.01}px`)
    }

    updateVh()
    window.addEventListener('resize', updateVh)
    window.addEventListener('orientationchange', updateVh)

    return () => {
      window.removeEventListener('resize', updateVh)
      window.removeEventListener('orientationchange', updateVh)
    }
  }, [])

  return vh
}

// Hook to detect safe area insets (for notched devices)
export function useSafeAreaInsets() {
  const [insets, setInsets] = useState({
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
  })

  useEffect(() => {
    const computedStyle = getComputedStyle(document.documentElement)

    setInsets({
      top: parseInt(computedStyle.getPropertyValue('--sat') || '0', 10),
      right: parseInt(computedStyle.getPropertyValue('--sar') || '0', 10),
      bottom: parseInt(computedStyle.getPropertyValue('--sab') || '0', 10),
      left: parseInt(computedStyle.getPropertyValue('--sal') || '0', 10),
    })

    // Also set CSS variables if env() is supported
    const style = document.documentElement.style
    style.setProperty('--sat', 'env(safe-area-inset-top, 0px)')
    style.setProperty('--sar', 'env(safe-area-inset-right, 0px)')
    style.setProperty('--sab', 'env(safe-area-inset-bottom, 0px)')
    style.setProperty('--sal', 'env(safe-area-inset-left, 0px)')
  }, [])

  return insets
}

// Hook to manage mobile keyboard visibility
export function useKeyboardVisibility() {
  const [isKeyboardVisible, setIsKeyboardVisible] = useState(false)
  const [keyboardHeight, setKeyboardHeight] = useState(0)

  useEffect(() => {
    const handleResize = () => {
      // On mobile, when keyboard opens, window height decreases
      if (window.visualViewport) {
        const heightDiff = window.innerHeight - window.visualViewport.height
        setIsKeyboardVisible(heightDiff > 150) // Threshold for keyboard
        setKeyboardHeight(heightDiff > 150 ? heightDiff : 0)
      }
    }

    if (window.visualViewport) {
      window.visualViewport.addEventListener('resize', handleResize)
      return () => window.visualViewport?.removeEventListener('resize', handleResize)
    }
  }, [])

  return { isKeyboardVisible, keyboardHeight }
}

export default {
  useBreakpoint,
  useIsMobile,
  useIsTablet,
  useIsDesktop,
  useIsTouchDevice,
  useOrientation,
  useReducedMotion,
  useLowPowerMode,
  useSwipeGesture,
  usePullToRefresh,
  usePinchZoom,
  useViewportHeight,
  useSafeAreaInsets,
  useKeyboardVisibility,
}
