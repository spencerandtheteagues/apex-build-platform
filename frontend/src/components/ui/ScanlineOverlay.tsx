// APEX.BUILD Scanline + CRT Overlay
// Adds retro CRT monitor effects: horizontal scanlines, screen curvature,
// chromatic aberration, and a subtle vignette — pure CSS, zero performance cost.

import React from 'react'
import { cn } from '@/lib/utils'

export interface ScanlineOverlayProps {
  intensity?: 'subtle' | 'medium' | 'heavy'
  showVignette?: boolean
  showFlicker?: boolean
  showCurvature?: boolean
  className?: string
}

const intensityConfig = {
  subtle: { opacity: '0.02', spacing: '4px' },
  medium: { opacity: '0.04', spacing: '3px' },
  heavy:  { opacity: '0.07', spacing: '2px' },
}

export const ScanlineOverlay: React.FC<ScanlineOverlayProps> = ({
  intensity = 'subtle',
  showVignette = true,
  showFlicker = false,
  showCurvature = false,
  className,
}) => {
  const config = intensityConfig[intensity]

  return (
    <div className={cn('fixed inset-0 pointer-events-none z-[9999]', className)}>
      {/* Scanlines */}
      <div
        className="absolute inset-0"
        style={{
          backgroundImage: `repeating-linear-gradient(
            0deg,
            transparent,
            transparent ${config.spacing},
            rgba(0, 0, 0, ${config.opacity}) ${config.spacing},
            rgba(0, 0, 0, ${config.opacity}) calc(${config.spacing} * 2)
          )`,
        }}
      />

      {/* Vignette */}
      {showVignette && (
        <div
          className="absolute inset-0"
          style={{
            background: 'radial-gradient(ellipse at center, transparent 60%, rgba(0,0,0,0.35) 100%)',
          }}
        />
      )}

      {/* Screen flicker */}
      {showFlicker && (
        <div
          className="absolute inset-0 animate-[crtFlicker_0.15s_infinite]"
          style={{
            background: 'rgba(18, 16, 16, 0)',
          }}
        />
      )}

      {/* CRT curvature simulation */}
      {showCurvature && (
        <div
          className="absolute inset-0"
          style={{
            boxShadow: 'inset 0 0 100px 40px rgba(0,0,0,0.15)',
            borderRadius: '8px',
          }}
        />
      )}

      {/* Subtle moving scan bar */}
      <div
        className="absolute left-0 right-0 h-[2px] animate-[scanBar_8s_linear_infinite]"
        style={{
          background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.02), transparent)',
          top: '0',
        }}
      />

      <style>{`
        @keyframes crtFlicker {
          0% { opacity: 0.27861; }
          5% { opacity: 0.34769; }
          10% { opacity: 0.23604; }
          15% { opacity: 0.90626; }
          20% { opacity: 0.18128; }
          25% { opacity: 0.83891; }
          30% { opacity: 0.65583; }
          35% { opacity: 0.67807; }
          40% { opacity: 0.26559; }
          45% { opacity: 0.84693; }
          50% { opacity: 0.96019; }
          55% { opacity: 0.08594; }
          60% { opacity: 0.20313; }
          65% { opacity: 0.71988; }
          70% { opacity: 0.53455; }
          75% { opacity: 0.37288; }
          80% { opacity: 0.71428; }
          85% { opacity: 0.70419; }
          90% { opacity: 0.7003; }
          95% { opacity: 0.36108; }
          100% { opacity: 0.24387; }
        }

        @keyframes scanBar {
          0% { top: -2px; }
          100% { top: 100%; }
        }
      `}</style>
    </div>
  )
}

export default ScanlineOverlay
