import React, { useEffect, useRef } from 'react'

interface BuildPieProgressProps {
  progress: number   // 0–100
  status: string
  phase?: string
  size?: number
}

// Interpolate color and glow intensity based on progress + status
function getTheme(progress: number, status: string) {
  if (status === 'completed') {
    return {
      stroke: '#22c55e',
      strokeDim: '#15803d',
      glow: 'rgba(34,197,94,0.9)',
      textColor: '#4ade80',
      trackColor: '#14532d',
      ringColor: 'rgba(34,197,94,0.4)',
    }
  }
  if (status === 'failed') {
    return {
      stroke: '#ef4444',
      strokeDim: '#7f1d1d',
      glow: 'rgba(239,68,68,0.7)',
      textColor: '#f87171',
      trackColor: '#1c0a0a',
      ringColor: 'rgba(239,68,68,0.3)',
    }
  }
  // In-progress: dim red → vivid red → orange → amber as progress climbs
  if (progress < 15) {
    return {
      stroke: '#7f1d1d',
      strokeDim: '#450a0a',
      glow: 'rgba(127,29,29,0.3)',
      textColor: '#b91c1c',
      trackColor: '#0a0a0a',
      ringColor: 'rgba(127,29,29,0.15)',
    }
  }
  if (progress < 30) {
    return {
      stroke: '#991b1b',
      strokeDim: '#7f1d1d',
      glow: 'rgba(153,27,27,0.45)',
      textColor: '#dc2626',
      trackColor: '#0f0404',
      ringColor: 'rgba(153,27,27,0.2)',
    }
  }
  if (progress < 50) {
    return {
      stroke: '#dc2626',
      strokeDim: '#991b1b',
      glow: 'rgba(220,38,38,0.55)',
      textColor: '#f87171',
      trackColor: '#150505',
      ringColor: 'rgba(220,38,38,0.25)',
    }
  }
  if (progress < 65) {
    return {
      stroke: '#ef4444',
      strokeDim: '#dc2626',
      glow: 'rgba(239,68,68,0.65)',
      textColor: '#f87171',
      trackColor: '#180606',
      ringColor: 'rgba(239,68,68,0.3)',
    }
  }
  if (progress < 80) {
    return {
      stroke: '#f97316',
      strokeDim: '#ea580c',
      glow: 'rgba(249,115,22,0.75)',
      textColor: '#fb923c',
      trackColor: '#1a0800',
      ringColor: 'rgba(249,115,22,0.35)',
    }
  }
  // 80–99%: bright amber, blazing
  return {
    stroke: '#fbbf24',
    strokeDim: '#f59e0b',
    glow: 'rgba(251,191,36,0.9)',
    textColor: '#fde68a',
    trackColor: '#1a1000',
    ringColor: 'rgba(251,191,36,0.45)',
  }
}

const BuildPieProgress: React.FC<BuildPieProgressProps> = ({
  progress,
  status,
  phase,
  size = 172,
}) => {
  const isActive = status !== 'completed' && status !== 'failed' && status !== 'idle'
  const clampedProgress = Math.min(100, Math.max(0, progress))
  const theme = getTheme(clampedProgress, status)

  // SVG arc math
  const cx = size / 2
  const cy = size / 2
  const outerR = size / 2 - 6     // outer spinning ring
  const arcR   = size / 2 - 16    // main progress arc
  const trackR = size / 2 - 16

  const arcCirc  = 2 * Math.PI * arcR
  const arcOffset = arcCirc - (clampedProgress / 100) * arcCirc

  // Outer ring: dashes that spin (always animated while active)
  const outerCirc = 2 * Math.PI * outerR
  const dashLen   = outerCirc * 0.12
  const gapLen    = outerCirc * 0.08

  // Brightness: 0% → 0.25 opacity, 100% → 1.0
  const brightness = 0.25 + (clampedProgress / 100) * 0.75
  // Glow radius grows with progress
  const glowPx = 4 + (clampedProgress / 100) * 18

  // Pulsing glow animation ref
  const pulseRef = useRef<SVGCircleElement>(null)

  useEffect(() => {
    const el = pulseRef.current
    if (!el || !isActive) return
    // keyframe-based pulse using Web Animations API
    const anim = el.animate(
      [
        { opacity: 0.5, filter: `drop-shadow(0 0 ${glowPx}px ${theme.glow})` },
        { opacity: 1.0, filter: `drop-shadow(0 0 ${glowPx * 2}px ${theme.glow})` },
        { opacity: 0.5, filter: `drop-shadow(0 0 ${glowPx}px ${theme.glow})` },
      ],
      { duration: 1800, iterations: Infinity, easing: 'ease-in-out' }
    )
    return () => anim.cancel()
  }, [isActive, theme.glow, glowPx])

  const phaseLabel = phase
    ? phase.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
    : status.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())

  return (
    <div
      className="relative flex flex-col items-center select-none"
      style={{ width: size, height: size + 28 }}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        style={{ overflow: 'visible' }}
      >
        <defs>
          {/* Radial glow behind the whole chart */}
          <radialGradient id="bpp-bg-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%"  stopColor={theme.glow} stopOpacity={brightness * 0.18} />
            <stop offset="100%" stopColor={theme.glow} stopOpacity={0} />
          </radialGradient>
          {/* Arc gradient */}
          <linearGradient id="bpp-arc-grad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%"   stopColor={theme.strokeDim} />
            <stop offset="100%" stopColor={theme.stroke} />
          </linearGradient>
        </defs>

        {/* Background glow blob */}
        <circle
          cx={cx} cy={cy}
          r={arcR + 14}
          fill="url(#bpp-bg-glow)"
          style={{ opacity: brightness }}
        />

        {/* Track circle */}
        <circle
          cx={cx} cy={cy} r={trackR}
          fill="none"
          stroke={theme.trackColor}
          strokeWidth={10}
        />

        {/* Outer spinning dashed ring — only when active */}
        {isActive && (
          <circle
            cx={cx} cy={cy} r={outerR}
            fill="none"
            stroke={theme.stroke}
            strokeWidth={2}
            strokeDasharray={`${dashLen} ${gapLen}`}
            strokeOpacity={brightness * 0.55}
            style={{
              transformOrigin: `${cx}px ${cy}px`,
              animation: 'bpp-spin 4s linear infinite',
            }}
          />
        )}

        {/* Static outer ring for completed/failed */}
        {!isActive && (
          <circle
            cx={cx} cy={cy} r={outerR}
            fill="none"
            stroke={theme.stroke}
            strokeWidth={2}
            strokeOpacity={0.4}
          />
        )}

        {/* Progress arc — rotated so it starts at top */}
        <circle
          ref={pulseRef}
          cx={cx} cy={cy} r={arcR}
          fill="none"
          stroke="url(#bpp-arc-grad)"
          strokeWidth={10}
          strokeLinecap="round"
          strokeDasharray={arcCirc}
          strokeDashoffset={arcOffset}
          style={{
            transformOrigin: `${cx}px ${cy}px`,
            transform: 'rotate(-90deg)',
            transition: 'stroke-dashoffset 0.6s cubic-bezier(0.4,0,0.2,1)',
            filter: `drop-shadow(0 0 ${glowPx}px ${theme.glow})`,
            opacity: brightness,
          }}
        />

        {/* Inner center glow dot */}
        <circle
          cx={cx} cy={cy} r={arcR - 14}
          fill="none"
          stroke={theme.stroke}
          strokeWidth={1}
          strokeOpacity={brightness * 0.15}
        />

        {/* Percent text */}
        <text
          x={cx} y={cy - 6}
          textAnchor="middle"
          dominantBaseline="middle"
          fill={theme.textColor}
          fontSize={size * 0.21}
          fontWeight={900}
          fontFamily="ui-monospace, monospace"
          style={{
            filter: `drop-shadow(0 0 ${glowPx * 0.8}px ${theme.glow})`,
            opacity: Math.max(0.5, brightness),
            transition: 'fill 0.5s ease, opacity 0.5s ease',
          }}
        >
          {clampedProgress}%
        </text>

        {/* Phase label inside circle */}
        <text
          x={cx} y={cy + size * 0.145}
          textAnchor="middle"
          dominantBaseline="middle"
          fill={theme.textColor}
          fontSize={size * 0.078}
          fontFamily="ui-sans-serif, sans-serif"
          letterSpacing={0.5}
          style={{
            opacity: Math.max(0.35, brightness * 0.8),
            textTransform: 'uppercase',
            transition: 'fill 0.5s ease',
          }}
        >
          {phaseLabel.length > 14 ? phaseLabel.slice(0, 13) + '…' : phaseLabel}
        </text>
      </svg>

      {/* Stall-proof pulse ring — always visible and animating while active */}
      {isActive && (
        <div
          className="absolute rounded-full pointer-events-none"
          style={{
            width: size,
            height: size,
            top: 0,
            left: 0,
            background: 'transparent',
            boxShadow: `0 0 ${glowPx * 1.5}px ${theme.glow}, inset 0 0 ${glowPx}px ${theme.ringColor}`,
            opacity: brightness * 0.6,
            borderRadius: '50%',
            animation: 'bpp-pulse 2.2s ease-in-out infinite',
          }}
        />
      )}

      <style>{`
        @keyframes bpp-spin {
          from { transform: rotate(0deg); }
          to   { transform: rotate(360deg); }
        }
        @keyframes bpp-pulse {
          0%, 100% { opacity: ${brightness * 0.3}; }
          50%       { opacity: ${brightness * 0.7}; }
        }
      `}</style>
    </div>
  )
}

export default BuildPieProgress
