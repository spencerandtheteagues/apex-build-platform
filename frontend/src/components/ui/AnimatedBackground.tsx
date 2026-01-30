// APEX.BUILD Animated Background
// Cyberpunk world - Mesmerizing particle system with grid, orbs, and data streams

import React, { useEffect, useRef, useCallback, useMemo } from 'react'
import { cn } from '@/lib/utils'

export interface AnimatedBackgroundProps {
  variant?: 'particles' | 'grid' | 'orbs' | 'matrix' | 'full'
  intensity?: 'low' | 'medium' | 'high'
  interactive?: boolean
  className?: string
}

// Particle interface for canvas animation
interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  opacity: number
  color: string
  pulseOffset: number
}

// Data stream interface
interface DataStream {
  x: number
  y: number
  speed: number
  length: number
  opacity: number
  chars: string[]
}

// Gradient orb interface
interface GradientOrb {
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  color1: string
  color2: string
}

// Intensity configurations
const intensityConfig = {
  low: {
    particleCount: 30,
    streamCount: 3,
    orbCount: 2,
    speed: 0.3,
    connectionDistance: 100,
  },
  medium: {
    particleCount: 60,
    streamCount: 6,
    orbCount: 3,
    speed: 0.5,
    connectionDistance: 130,
  },
  high: {
    particleCount: 100,
    streamCount: 10,
    orbCount: 5,
    speed: 0.7,
    connectionDistance: 160,
  },
}

// APEX color palette
const apexColors = {
  primary: ['#ef4444', '#dc2626', '#b91c1c', '#991b1b'],
  secondary: ['#f97316', '#ea580c', '#c2410c'],
  accent: ['#7c3aed', '#6d28d9', '#5b21b6'],
  glow: ['rgba(239, 68, 68, 0.6)', 'rgba(249, 115, 22, 0.4)', 'rgba(124, 58, 237, 0.3)'],
}

export const AnimatedBackground: React.FC<AnimatedBackgroundProps> = ({
  variant = 'full',
  intensity = 'medium',
  interactive = true,
  className,
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const particlesRef = useRef<Particle[]>([])
  const streamsRef = useRef<DataStream[]>([])
  const orbsRef = useRef<GradientOrb[]>([])
  const mouseRef = useRef({ x: -1000, y: -1000 })
  const animationRef = useRef<number>()
  const timeRef = useRef(0)
  const prefersReducedMotion = useRef(false)

  const config = useMemo(() => intensityConfig[intensity], [intensity])

  // Check for reduced motion preference
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    prefersReducedMotion.current = mediaQuery.matches

    const handler = (e: MediaQueryListEvent) => {
      prefersReducedMotion.current = e.matches
    }
    mediaQuery.addEventListener('change', handler)
    return () => mediaQuery.removeEventListener('change', handler)
  }, [])

  // Initialize particles
  const initParticles = useCallback((width: number, height: number) => {
    const particles: Particle[] = []
    for (let i = 0; i < config.particleCount; i++) {
      particles.push({
        x: Math.random() * width,
        y: Math.random() * height,
        vx: (Math.random() - 0.5) * config.speed,
        vy: (Math.random() - 0.5) * config.speed,
        size: Math.random() * 2.5 + 0.5,
        opacity: Math.random() * 0.6 + 0.2,
        color: apexColors.primary[Math.floor(Math.random() * apexColors.primary.length)],
        pulseOffset: Math.random() * Math.PI * 2,
      })
    }
    particlesRef.current = particles
  }, [config])

  // Initialize data streams
  const initStreams = useCallback((width: number, height: number) => {
    const streams: DataStream[] = []
    const chars = '01アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン'.split('')

    for (let i = 0; i < config.streamCount; i++) {
      const streamLength = Math.floor(Math.random() * 15) + 8
      streams.push({
        x: Math.random() * width,
        y: Math.random() * height - height,
        speed: Math.random() * 2 + 1,
        length: streamLength,
        opacity: Math.random() * 0.3 + 0.1,
        chars: Array.from({ length: streamLength }, () => chars[Math.floor(Math.random() * chars.length)]),
      })
    }
    streamsRef.current = streams
  }, [config])

  // Initialize gradient orbs
  const initOrbs = useCallback((width: number, height: number) => {
    const orbs: GradientOrb[] = []
    const colors = [
      { c1: 'rgba(139, 0, 0, 0.15)', c2: 'rgba(0, 0, 0, 0)' },
      { c1: 'rgba(88, 28, 135, 0.12)', c2: 'rgba(0, 0, 0, 0)' },
      { c1: 'rgba(154, 52, 18, 0.1)', c2: 'rgba(0, 0, 0, 0)' },
      { c1: 'rgba(127, 29, 29, 0.12)', c2: 'rgba(0, 0, 0, 0)' },
      { c1: 'rgba(30, 0, 50, 0.15)', c2: 'rgba(0, 0, 0, 0)' },
    ]

    for (let i = 0; i < config.orbCount; i++) {
      const colorSet = colors[i % colors.length]
      orbs.push({
        x: Math.random() * width,
        y: Math.random() * height,
        vx: (Math.random() - 0.5) * 0.2,
        vy: (Math.random() - 0.5) * 0.2,
        radius: Math.random() * 200 + 150,
        color1: colorSet.c1,
        color2: colorSet.c2,
      })
    }
    orbsRef.current = orbs
  }, [config])

  // Draw perspective grid
  const drawGrid = useCallback((ctx: CanvasRenderingContext2D, width: number, height: number, time: number) => {
    const gridSpacing = 60
    const horizonY = height * 0.35
    const perspectiveStrength = 0.85

    ctx.strokeStyle = 'rgba(239, 68, 68, 0.08)'
    ctx.lineWidth = 1

    // Vertical lines with perspective
    for (let x = -width; x <= width * 2; x += gridSpacing) {
      const startX = x
      const endX = width / 2 + (x - width / 2) * perspectiveStrength

      ctx.beginPath()
      ctx.moveTo(startX, height)
      ctx.lineTo(endX, horizonY)
      ctx.stroke()
    }

    // Horizontal lines
    const lineCount = 20
    for (let i = 0; i < lineCount; i++) {
      const progress = i / lineCount
      const y = horizonY + (height - horizonY) * progress
      const perspectiveFactor = progress
      const lineWidth = width * (0.3 + perspectiveFactor * 0.7)
      const startX = (width - lineWidth) / 2

      // Pulsing effect on grid lines
      const pulseIntensity = Math.sin(time * 0.001 + i * 0.3) * 0.02 + 0.08
      ctx.strokeStyle = `rgba(239, 68, 68, ${pulseIntensity})`

      ctx.beginPath()
      ctx.moveTo(startX, y)
      ctx.lineTo(startX + lineWidth, y)
      ctx.stroke()
    }

    // Intersection glow points
    ctx.fillStyle = 'rgba(239, 68, 68, 0.3)'
    for (let i = 0; i < 8; i++) {
      for (let j = 0; j < 5; j++) {
        const progress = (j + 1) / 6
        const y = horizonY + (height - horizonY) * progress
        const lineWidth = width * (0.3 + progress * 0.7)
        const startX = (width - lineWidth) / 2
        const x = startX + (lineWidth / 7) * (i + 1)

        const glowSize = Math.sin(time * 0.002 + i + j) * 1 + 2
        ctx.beginPath()
        ctx.arc(x, y, glowSize, 0, Math.PI * 2)
        ctx.fill()
      }
    }
  }, [])

  // Draw gradient orbs
  const drawOrbs = useCallback((ctx: CanvasRenderingContext2D, width: number, height: number) => {
    orbsRef.current.forEach((orb) => {
      // Update position
      orb.x += orb.vx
      orb.y += orb.vy

      // Bounce off edges with padding
      const padding = orb.radius * 0.5
      if (orb.x < -padding || orb.x > width + padding) orb.vx *= -1
      if (orb.y < -padding || orb.y > height + padding) orb.vy *= -1

      // Draw radial gradient
      const gradient = ctx.createRadialGradient(orb.x, orb.y, 0, orb.x, orb.y, orb.radius)
      gradient.addColorStop(0, orb.color1)
      gradient.addColorStop(1, orb.color2)

      ctx.fillStyle = gradient
      ctx.beginPath()
      ctx.arc(orb.x, orb.y, orb.radius, 0, Math.PI * 2)
      ctx.fill()
    })
  }, [])

  // Draw particles with constellation effect
  const drawParticles = useCallback((ctx: CanvasRenderingContext2D, width: number, height: number, time: number) => {
    const particles = particlesRef.current
    const mouse = mouseRef.current

    particles.forEach((particle, i) => {
      // Update position
      particle.x += particle.vx
      particle.y += particle.vy

      // Bounce off edges
      if (particle.x < 0 || particle.x > width) particle.vx *= -1
      if (particle.y < 0 || particle.y > height) particle.vy *= -1

      particle.x = Math.max(0, Math.min(width, particle.x))
      particle.y = Math.max(0, Math.min(height, particle.y))

      // Mouse interaction - particles flee from cursor
      if (interactive && mouse.x > 0 && mouse.y > 0) {
        const dx = particle.x - mouse.x
        const dy = particle.y - mouse.y
        const distance = Math.sqrt(dx * dx + dy * dy)
        const fleeRadius = 100

        if (distance < fleeRadius && distance > 0) {
          const force = (fleeRadius - distance) / fleeRadius * 0.5
          particle.x += (dx / distance) * force * 3
          particle.y += (dy / distance) * force * 3
        }
      }

      // Pulsing glow effect
      const pulse = Math.sin(time * 0.003 + particle.pulseOffset) * 0.3 + 0.7
      const glowSize = particle.size * 3 * pulse

      // Draw glow
      const gradient = ctx.createRadialGradient(
        particle.x, particle.y, 0,
        particle.x, particle.y, glowSize
      )
      gradient.addColorStop(0, particle.color)
      gradient.addColorStop(0.4, `${particle.color}66`)
      gradient.addColorStop(1, 'transparent')

      ctx.fillStyle = gradient
      ctx.globalAlpha = particle.opacity * pulse
      ctx.beginPath()
      ctx.arc(particle.x, particle.y, glowSize, 0, Math.PI * 2)
      ctx.fill()

      // Draw core
      ctx.fillStyle = '#ffffff'
      ctx.globalAlpha = particle.opacity
      ctx.beginPath()
      ctx.arc(particle.x, particle.y, particle.size * 0.5, 0, Math.PI * 2)
      ctx.fill()

      // Draw connections (constellation effect)
      for (let j = i + 1; j < particles.length; j++) {
        const other = particles[j]
        const dx = particle.x - other.x
        const dy = particle.y - other.y
        const distance = Math.sqrt(dx * dx + dy * dy)

        if (distance < config.connectionDistance) {
          const opacity = (1 - distance / config.connectionDistance) * 0.25
          ctx.beginPath()
          ctx.moveTo(particle.x, particle.y)
          ctx.lineTo(other.x, other.y)
          ctx.strokeStyle = particle.color
          ctx.globalAlpha = opacity
          ctx.lineWidth = 0.5
          ctx.stroke()
        }
      }
    })

    ctx.globalAlpha = 1
  }, [config, interactive])

  // Draw data streams (Matrix-style)
  const drawStreams = useCallback((ctx: CanvasRenderingContext2D, width: number, height: number) => {
    const fontSize = 14
    ctx.font = `${fontSize}px monospace`

    streamsRef.current.forEach((stream) => {
      // Update position
      stream.y += stream.speed

      // Reset when off screen
      if (stream.y > height + stream.length * fontSize) {
        stream.y = -stream.length * fontSize
        stream.x = Math.random() * width
        stream.opacity = Math.random() * 0.3 + 0.1
      }

      // Randomize characters occasionally
      if (Math.random() < 0.05) {
        const charIndex = Math.floor(Math.random() * stream.chars.length)
        const chars = '01アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン'.split('')
        stream.chars[charIndex] = chars[Math.floor(Math.random() * chars.length)]
      }

      // Draw characters
      stream.chars.forEach((char, i) => {
        const y = stream.y + i * fontSize
        if (y < 0 || y > height) return

        // Gradient opacity - brighter at the head
        const headDistance = stream.chars.length - i
        const fadeOpacity = headDistance / stream.chars.length
        const finalOpacity = stream.opacity * fadeOpacity

        // Head character is brightest
        if (i === stream.chars.length - 1) {
          ctx.fillStyle = '#ffffff'
          ctx.globalAlpha = stream.opacity * 0.9
        } else {
          // Orange to red gradient for APEX theme
          const colorRatio = i / stream.chars.length
          const r = Math.floor(239 + (249 - 239) * colorRatio)
          const g = Math.floor(68 + (115 - 68) * colorRatio)
          const b = Math.floor(68 + (22 - 68) * colorRatio)
          ctx.fillStyle = `rgb(${r}, ${g}, ${b})`
          ctx.globalAlpha = finalOpacity
        }

        ctx.fillText(char, stream.x, y)
      })
    })

    ctx.globalAlpha = 1
  }, [])

  // Draw scanlines (CRT effect)
  const drawScanlines = useCallback((ctx: CanvasRenderingContext2D, width: number, height: number, time: number) => {
    const lineSpacing = 3
    const scanLineOffset = (time * 0.02) % lineSpacing

    ctx.fillStyle = 'rgba(0, 0, 0, 0.03)'
    for (let y = scanLineOffset; y < height; y += lineSpacing) {
      ctx.fillRect(0, y, width, 1)
    }

    // Moving scan bar
    const scanY = (time * 0.1) % (height + 100) - 50
    const scanGradient = ctx.createLinearGradient(0, scanY - 20, 0, scanY + 20)
    scanGradient.addColorStop(0, 'transparent')
    scanGradient.addColorStop(0.5, 'rgba(239, 68, 68, 0.03)')
    scanGradient.addColorStop(1, 'transparent')
    ctx.fillStyle = scanGradient
    ctx.fillRect(0, scanY - 20, width, 40)
  }, [])

  // Main animation loop
  const animate = useCallback(() => {
    if (prefersReducedMotion.current) {
      // For reduced motion, just draw static elements once
      const canvas = canvasRef.current
      if (!canvas) return
      const ctx = canvas.getContext('2d')
      if (!ctx) return

      ctx.clearRect(0, 0, canvas.width, canvas.height)

      if (variant === 'orbs' || variant === 'full') {
        drawOrbs(ctx, canvas.width, canvas.height)
      }
      return
    }

    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const { width, height } = canvas
    timeRef.current += 16 // Approximate 60fps

    // Clear canvas
    ctx.clearRect(0, 0, width, height)

    // Draw layers based on variant
    if (variant === 'orbs' || variant === 'full') {
      drawOrbs(ctx, width, height)
    }

    if (variant === 'grid' || variant === 'full') {
      drawGrid(ctx, width, height, timeRef.current)
    }

    if (variant === 'matrix' || variant === 'full') {
      drawStreams(ctx, width, height)
    }

    if (variant === 'particles' || variant === 'full') {
      drawParticles(ctx, width, height, timeRef.current)
    }

    if (variant === 'full') {
      drawScanlines(ctx, width, height, timeRef.current)
    }

    animationRef.current = requestAnimationFrame(animate)
  }, [variant, drawOrbs, drawGrid, drawStreams, drawParticles, drawScanlines])

  // Handle resize
  const handleResize = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const parent = canvas.parentElement
    if (!parent) return

    // Use devicePixelRatio for crisp rendering
    const dpr = window.devicePixelRatio || 1
    const rect = parent.getBoundingClientRect()

    canvas.width = rect.width * dpr
    canvas.height = rect.height * dpr
    canvas.style.width = `${rect.width}px`
    canvas.style.height = `${rect.height}px`

    const ctx = canvas.getContext('2d')
    if (ctx) {
      ctx.scale(dpr, dpr)
    }

    initParticles(rect.width, rect.height)
    initStreams(rect.width, rect.height)
    initOrbs(rect.width, rect.height)
  }, [initParticles, initStreams, initOrbs])

  // Handle mouse move
  const handleMouseMove = useCallback((e: MouseEvent) => {
    const canvas = canvasRef.current
    if (!canvas) return

    const rect = canvas.getBoundingClientRect()
    mouseRef.current = {
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
    }
  }, [])

  // Handle mouse leave
  const handleMouseLeave = useCallback(() => {
    mouseRef.current = { x: -1000, y: -1000 }
  }, [])

  // Setup and cleanup
  useEffect(() => {
    handleResize()

    // Start animation after a brief delay to ensure canvas is ready
    const startTimer = setTimeout(() => {
      animate()
    }, 50)

    window.addEventListener('resize', handleResize)

    if (interactive) {
      window.addEventListener('mousemove', handleMouseMove)
      window.addEventListener('mouseleave', handleMouseLeave)
    }

    return () => {
      clearTimeout(startTimer)
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current)
      }
      window.removeEventListener('resize', handleResize)
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseleave', handleMouseLeave)
    }
  }, [handleResize, animate, handleMouseMove, handleMouseLeave, interactive])

  return (
    <div className={cn('absolute inset-0 overflow-hidden', className)}>
      {/* Base gradient layer */}
      <div className="absolute inset-0 bg-gradient-to-br from-black via-gray-950 to-red-950/20" />

      {/* Vignette effect */}
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          background: 'radial-gradient(ellipse at center, transparent 0%, rgba(0,0,0,0.4) 100%)',
        }}
      />

      {/* Canvas layer */}
      <canvas
        ref={canvasRef}
        className="absolute inset-0 pointer-events-none"
        style={{ mixBlendMode: 'screen' }}
      />

      {/* Noise texture overlay (CSS-based for performance) */}
      <div
        className="absolute inset-0 pointer-events-none opacity-[0.015]"
        style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E")`,
        }}
      />
    </div>
  )
}

export default AnimatedBackground
