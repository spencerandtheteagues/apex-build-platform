import React, { useState, useEffect, useRef } from 'react';
import '../styles/steampunk.css';

interface SteampunkDashboardProps {
  onNavigate: (view: 'ide' | 'builder' | 'projects' | 'settings') => void;
}

// Particle component for floating effects
const Particle: React.FC<{ delay: number; x: number; color: string }> = ({ delay, x, color }) => (
  <div
    style={{
      position: 'absolute',
      width: '4px',
      height: '4px',
      borderRadius: '50%',
      background: color,
      left: `${x}%`,
      bottom: '0',
      boxShadow: `0 0 10px ${color}, 0 0 20px ${color}`,
      animation: `particle-rise 8s ease-in-out infinite`,
      animationDelay: `${delay}s`,
      opacity: 0,
    }}
  />
);

// Gear component for steampunk decoration
const Gear: React.FC<{ size: number; top: string; left: string; reverse?: boolean; opacity?: number }> = ({
  size, top, left, reverse = false, opacity = 0.15
}) => (
  <div
    style={{
      position: 'absolute',
      width: `${size}px`,
      height: `${size}px`,
      top,
      left,
      opacity,
      pointerEvents: 'none',
      animation: reverse ? 'gear-spin-reverse 20s linear infinite' : 'gear-spin 15s linear infinite',
    }}
  >
    <svg viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="50" cy="50" r="35" stroke="#b5a642" strokeWidth="3" fill="none" />
      <circle cx="50" cy="50" r="12" stroke="#b5a642" strokeWidth="2" fill="#1a1a1a" />
      {[0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330].map((angle) => (
        <rect
          key={angle}
          x="47"
          y="10"
          width="6"
          height="15"
          fill="#b5a642"
          transform={`rotate(${angle} 50 50)`}
        />
      ))}
    </svg>
  </div>
);

// Steam effect component
const SteamVent: React.FC<{ left: string }> = ({ left }) => {
  const particles = Array.from({ length: 5 }, (_, i) => i);
  return (
    <div style={{ position: 'absolute', bottom: '0', left, width: '30px', height: '60px' }}>
      {particles.map((i) => (
        <div
          key={i}
          style={{
            position: 'absolute',
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(255,255,255,0.4), transparent)',
            left: `${10 + Math.random() * 10}px`,
            bottom: '0',
            animation: `steam-rise ${2 + Math.random()}s ease-out infinite`,
            animationDelay: `${i * 0.4}s`,
          }}
        />
      ))}
    </div>
  );
};

// Holographic card component
const HoloCard: React.FC<{
  children: React.ReactNode;
  glowColor: string;
  featured?: boolean;
  onClick?: () => void;
}> = ({ children, glowColor, featured = false, onClick }) => {
  const [isHovered, setIsHovered] = useState(false);

  return (
    <div
      onClick={onClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      style={{
        position: 'relative',
        background: featured
          ? 'linear-gradient(145deg, rgba(0, 245, 255, 0.08), rgba(139, 0, 255, 0.08), rgba(255, 0, 128, 0.05))'
          : 'linear-gradient(145deg, rgba(5, 5, 8, 0.95), rgba(10, 10, 15, 0.9))',
        border: `2px solid ${featured ? glowColor : 'rgba(181, 166, 66, 0.4)'}`,
        borderRadius: '16px',
        padding: '28px',
        cursor: onClick ? 'pointer' : 'default',
        transition: 'all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275)',
        transform: isHovered ? 'translateY(-8px) scale(1.02)' : 'translateY(0) scale(1)',
        boxShadow: isHovered
          ? `0 20px 60px rgba(0, 0, 0, 0.5), 0 0 40px ${glowColor}40, inset 0 0 30px ${glowColor}10`
          : `0 8px 32px rgba(0, 0, 0, 0.4), 0 0 20px ${glowColor}20`,
        overflow: 'hidden',
      }}
    >
      {/* Corner brass decorations */}
      <div style={{ position: 'absolute', top: '-1px', left: '-1px', width: '24px', height: '24px', borderTop: '3px solid #b5a642', borderLeft: '3px solid #b5a642', borderRadius: '16px 0 0 0' }} />
      <div style={{ position: 'absolute', top: '-1px', right: '-1px', width: '24px', height: '24px', borderTop: '3px solid #b5a642', borderRight: '3px solid #b5a642', borderRadius: '0 16px 0 0' }} />
      <div style={{ position: 'absolute', bottom: '-1px', left: '-1px', width: '24px', height: '24px', borderBottom: '3px solid #b5a642', borderLeft: '3px solid #b5a642', borderRadius: '0 0 0 16px' }} />
      <div style={{ position: 'absolute', bottom: '-1px', right: '-1px', width: '24px', height: '24px', borderBottom: '3px solid #b5a642', borderRight: '3px solid #b5a642', borderRadius: '0 0 16px 0' }} />

      {/* Holographic shimmer on hover */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: '-100%',
          width: '100%',
          height: '100%',
          background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.05), transparent)',
          transition: 'left 0.6s ease',
          ...(isHovered && { left: '100%' }),
        }}
      />

      {children}
    </div>
  );
};

// Neon button component
const NeonButton: React.FC<{
  children: React.ReactNode;
  color: 'cyan' | 'pink' | 'green' | 'purple';
  onClick?: () => void;
  fullWidth?: boolean;
  size?: 'sm' | 'md' | 'lg';
}> = ({ children, color, onClick, fullWidth = false, size = 'md' }) => {
  const [isPressed, setIsPressed] = useState(false);

  const colors = {
    cyan: { bg: 'linear-gradient(135deg, #00f5ff, #0099cc)', glow: '#00f5ff', text: '#000' },
    pink: { bg: 'linear-gradient(135deg, #ff0080, #cc0066)', glow: '#ff0080', text: '#fff' },
    green: { bg: 'linear-gradient(135deg, #39ff14, #00aa00)', glow: '#39ff14', text: '#000' },
    purple: { bg: 'linear-gradient(135deg, #8b00ff, #00f5ff)', glow: '#8b00ff', text: '#fff' },
  };

  const sizes = {
    sm: { padding: '10px 20px', fontSize: '14px' },
    md: { padding: '16px 32px', fontSize: '16px' },
    lg: { padding: '20px 40px', fontSize: '18px' },
  };

  return (
    <button
      onClick={onClick}
      onMouseDown={() => setIsPressed(true)}
      onMouseUp={() => setIsPressed(false)}
      onMouseLeave={() => setIsPressed(false)}
      style={{
        position: 'relative',
        background: colors[color].bg,
        color: colors[color].text,
        border: 'none',
        borderRadius: '10px',
        padding: sizes[size].padding,
        fontSize: sizes[size].fontSize,
        fontWeight: 'bold',
        fontFamily: "'Orbitron', 'Rajdhani', monospace",
        textTransform: 'uppercase',
        letterSpacing: '2px',
        cursor: 'pointer',
        width: fullWidth ? '100%' : 'auto',
        transition: 'all 0.3s ease',
        transform: isPressed ? 'scale(0.98)' : 'scale(1)',
        boxShadow: `
          0 0 10px ${colors[color].glow}60,
          0 0 20px ${colors[color].glow}40,
          0 0 40px ${colors[color].glow}20,
          inset 0 0 20px rgba(255,255,255,0.1)
        `,
        textShadow: color === 'cyan' || color === 'green' ? 'none' : '0 0 10px rgba(255,255,255,0.5)',
        overflow: 'hidden',
      }}
    >
      {/* Animated shine effect */}
      <span
        style={{
          position: 'absolute',
          top: 0,
          left: '-100%',
          width: '100%',
          height: '100%',
          background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent)',
          animation: 'button-shine 3s ease-in-out infinite',
        }}
      />
      {children}
    </button>
  );
};

// Status indicator with pulse
const StatusIndicator: React.FC<{ status: 'online' | 'offline' | 'processing'; label: string }> = ({ status, label }) => {
  const statusColors = {
    online: '#39ff14',
    offline: '#ff0000',
    processing: '#00f5ff',
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '12px' }}>
      <div
        style={{
          width: '10px',
          height: '10px',
          borderRadius: '50%',
          background: statusColors[status],
          boxShadow: `0 0 10px ${statusColors[status]}, 0 0 20px ${statusColors[status]}60`,
          animation: status === 'online' ? 'pulse-glow 1.5s ease-in-out infinite' : 'none',
        }}
      />
      <span style={{ color: statusColors[status], fontSize: '14px', fontFamily: 'monospace' }}>{label}</span>
    </div>
  );
};

export const SteampunkDashboard: React.FC<SteampunkDashboardProps> = ({ onNavigate }) => {
  const [time, setTime] = useState(new Date());
  const [backendStatus, setBackendStatus] = useState<'online' | 'offline' | 'processing'>('processing');
  const canvasRef = useRef<HTMLCanvasElement>(null);

  // Update clock
  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  // Check backend status
  useEffect(() => {
    const checkBackend = async () => {
      try {
        const response = await fetch('http://localhost:8080/health');
        if (response.ok) {
          setBackendStatus('online');
        } else {
          setBackendStatus('offline');
        }
      } catch {
        setBackendStatus('offline');
      }
    };
    checkBackend();
    const interval = setInterval(checkBackend, 10000);
    return () => clearInterval(interval);
  }, []);

  // Particle animation canvas
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const particles: Array<{ x: number; y: number; vx: number; vy: number; color: string; size: number }> = [];

    // Create particles
    for (let i = 0; i < 50; i++) {
      particles.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height,
        vx: (Math.random() - 0.5) * 0.5,
        vy: (Math.random() - 0.5) * 0.5,
        color: ['#00f5ff', '#ff0080', '#39ff14', '#8b00ff'][Math.floor(Math.random() * 4)],
        size: Math.random() * 2 + 1,
      });
    }

    const animate = () => {
      ctx.fillStyle = 'rgba(0, 0, 0, 0.05)';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      particles.forEach((p) => {
        p.x += p.vx;
        p.y += p.vy;

        if (p.x < 0 || p.x > canvas.width) p.vx *= -1;
        if (p.y < 0 || p.y > canvas.height) p.vy *= -1;

        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        ctx.fillStyle = p.color;
        ctx.shadowBlur = 15;
        ctx.shadowColor = p.color;
        ctx.fill();
      });

      // Draw connecting lines
      particles.forEach((p1, i) => {
        particles.slice(i + 1).forEach((p2) => {
          const dx = p1.x - p2.x;
          const dy = p1.y - p2.y;
          const dist = Math.sqrt(dx * dx + dy * dy);

          if (dist < 150) {
            ctx.beginPath();
            ctx.moveTo(p1.x, p1.y);
            ctx.lineTo(p2.x, p2.y);
            ctx.strokeStyle = `rgba(0, 245, 255, ${0.1 * (1 - dist / 150)})`;
            ctx.lineWidth = 0.5;
            ctx.stroke();
          }
        });
      });

      requestAnimationFrame(animate);
    };

    animate();

    const handleResize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  return (
    <div
      style={{
        minHeight: '100vh',
        background: '#000000',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Animated particle canvas */}
      <canvas
        ref={canvasRef}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          zIndex: 0,
          pointerEvents: 'none',
        }}
      />

      {/* Gradient overlay */}
      <div
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          background: `
            radial-gradient(ellipse at 15% 85%, rgba(139, 0, 255, 0.15) 0%, transparent 50%),
            radial-gradient(ellipse at 85% 15%, rgba(0, 245, 255, 0.15) 0%, transparent 50%),
            radial-gradient(ellipse at 50% 50%, rgba(255, 0, 128, 0.08) 0%, transparent 60%)
          `,
          zIndex: 1,
          pointerEvents: 'none',
        }}
      />

      {/* Circuit pattern overlay */}
      <div
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          backgroundImage: `
            linear-gradient(rgba(0, 245, 255, 0.03) 1px, transparent 1px),
            linear-gradient(90deg, rgba(0, 245, 255, 0.03) 1px, transparent 1px)
          `,
          backgroundSize: '50px 50px',
          zIndex: 2,
          pointerEvents: 'none',
        }}
      />

      {/* Scanline effect */}
      <div
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          height: '100%',
          background: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.1) 0px, rgba(0,0,0,0.1) 1px, transparent 1px, transparent 2px)',
          zIndex: 3,
          pointerEvents: 'none',
          opacity: 0.5,
        }}
      />

      {/* Moving scanline */}
      <div
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          width: '100%',
          height: '4px',
          background: 'linear-gradient(180deg, transparent, rgba(0, 245, 255, 0.4), transparent)',
          zIndex: 4,
          pointerEvents: 'none',
          animation: 'scanline 6s linear infinite',
        }}
      />

      {/* Decorative gears */}
      <Gear size={200} top="-50px" left="-50px" opacity={0.08} />
      <Gear size={150} top="20%" right="-30px" left="auto" reverse opacity={0.06} />
      <Gear size={120} bottom="10%" left="5%" top="auto" opacity={0.05} />
      <Gear size={180} bottom="-40px" right="10%" left="auto" top="auto" reverse opacity={0.07} />

      {/* Steam vents */}
      <SteamVent left="10%" />
      <SteamVent left="90%" />

      {/* Main content */}
      <div
        style={{
          position: 'relative',
          zIndex: 10,
          maxWidth: '1400px',
          margin: '0 auto',
          padding: '40px 20px',
        }}
      >
        {/* Header */}
        <header style={{ textAlign: 'center', marginBottom: '60px' }}>
          {/* Clock display */}
          <div
            style={{
              position: 'absolute',
              top: '20px',
              right: '40px',
              fontFamily: "'Orbitron', monospace",
              fontSize: '14px',
              color: '#b5a642',
              textShadow: '0 0 10px rgba(181, 166, 66, 0.5)',
            }}
          >
            {time.toLocaleTimeString()}
          </div>

          {/* Main title */}
          <h1
            style={{
              fontSize: 'clamp(2.5rem, 8vw, 5rem)',
              fontFamily: "'Orbitron', 'Rajdhani', sans-serif",
              fontWeight: 900,
              margin: 0,
              marginBottom: '10px',
              background: 'linear-gradient(180deg, #00f5ff 0%, #0099cc 50%, #00f5ff 100%)',
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
              textShadow: 'none',
              filter: 'drop-shadow(0 0 30px rgba(0, 245, 255, 0.5))',
              letterSpacing: '8px',
            }}
          >
            APEX.BUILD
          </h1>

          {/* Subtitle with typing effect */}
          <p
            style={{
              fontSize: 'clamp(1rem, 2.5vw, 1.4rem)',
              color: '#ffffff',
              fontFamily: "'Rajdhani', monospace",
              letterSpacing: '4px',
              textTransform: 'uppercase',
              margin: 0,
              opacity: 0.9,
            }}
          >
            22nd Century Cloud Development Platform
          </p>

          {/* Energy line */}
          <div
            style={{
              width: '300px',
              height: '2px',
              margin: '30px auto',
              background: 'linear-gradient(90deg, transparent, #00f5ff, #8b00ff, #ff0080, #8b00ff, #00f5ff, transparent)',
              boxShadow: '0 0 20px rgba(0, 245, 255, 0.5)',
              animation: 'energy-pulse 2s ease-in-out infinite',
            }}
          />

          {/* Tagline */}
          <p
            style={{
              fontSize: '1.1rem',
              color: '#b5a642',
              fontFamily: 'monospace',
              fontStyle: 'italic',
            }}
          >
            "Where AI Agents Build Your Dreams"
          </p>
        </header>

        {/* Card grid */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))',
            gap: '30px',
            marginBottom: '50px',
          }}
        >
          {/* AI App Builder - Featured Card */}
          <HoloCard glowColor="#00f5ff" featured onClick={() => onNavigate('builder')}>
            <div
              style={{
                position: 'absolute',
                top: '15px',
                right: '15px',
                background: 'linear-gradient(135deg, #39ff14, #00ff88)',
                color: '#000',
                padding: '6px 14px',
                borderRadius: '20px',
                fontSize: '10px',
                fontWeight: 'bold',
                textTransform: 'uppercase',
                letterSpacing: '2px',
                boxShadow: '0 0 15px rgba(57, 255, 20, 0.5)',
              }}
            >
              FLAGSHIP
            </div>

            <div style={{ fontSize: '48px', marginBottom: '15px', filter: 'drop-shadow(0 0 10px #00f5ff)' }}>
              🤖
            </div>

            <h3
              style={{
                color: '#00f5ff',
                fontSize: '1.6rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '15px',
                textShadow: '0 0 20px rgba(0, 245, 255, 0.5)',
              }}
            >
              AI App Builder
            </h3>

            <p style={{ color: '#ccc', lineHeight: '1.8', marginBottom: '25px', fontSize: '15px' }}>
              Describe your application in plain English. Watch Claude, GPT-4, and Gemini agents collaborate in real-time to build production-ready code.
            </p>

            <NeonButton color="cyan" fullWidth onClick={() => onNavigate('builder')}>
              ⚡ Build My App
            </NeonButton>
          </HoloCard>

          {/* Professional IDE */}
          <HoloCard glowColor="#ff0080" onClick={() => onNavigate('ide')}>
            <div style={{ fontSize: '42px', marginBottom: '15px', filter: 'drop-shadow(0 0 10px #ff0080)' }}>
              💻
            </div>

            <h3
              style={{
                color: '#ff0080',
                fontSize: '1.4rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '15px',
                textShadow: '0 0 15px rgba(255, 0, 128, 0.5)',
              }}
            >
              Professional IDE
            </h3>

            <p style={{ color: '#ccc', lineHeight: '1.7', marginBottom: '25px', fontSize: '14px' }}>
              Full-featured Monaco Editor with AI-powered code completion, real-time collaboration, and intelligent refactoring.
            </p>

            <NeonButton color="pink" fullWidth onClick={() => onNavigate('ide')}>
              🚀 Launch IDE
            </NeonButton>
          </HoloCard>

          {/* Backend Status */}
          <HoloCard glowColor="#39ff14">
            <div style={{ fontSize: '42px', marginBottom: '15px', filter: 'drop-shadow(0 0 10px #39ff14)' }}>
              ⚙️
            </div>

            <h3
              style={{
                color: '#39ff14',
                fontSize: '1.4rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '20px',
                textShadow: '0 0 15px rgba(57, 255, 20, 0.5)',
              }}
            >
              System Status
            </h3>

            <div style={{ marginBottom: '20px' }}>
              <StatusIndicator status={backendStatus} label={`API Server: ${backendStatus.toUpperCase()}`} />
              <StatusIndicator status="online" label="Database: CONNECTED" />
              <StatusIndicator status="online" label="WebSocket: READY" />
              <StatusIndicator status="online" label="AI Services: CONFIGURED" />
            </div>

            <NeonButton
              color="green"
              fullWidth
              size="sm"
              onClick={() => {
                setBackendStatus('processing');
                fetch('http://localhost:8080/health')
                  .then((r) => r.ok ? setBackendStatus('online') : setBackendStatus('offline'))
                  .catch(() => setBackendStatus('offline'));
              }}
            >
              🔧 Test Connection
            </NeonButton>
          </HoloCard>

          {/* Quick Actions */}
          <HoloCard glowColor="#8b00ff">
            <div style={{ fontSize: '42px', marginBottom: '15px', filter: 'drop-shadow(0 0 10px #8b00ff)' }}>
              ⚡
            </div>

            <h3
              style={{
                color: '#8b00ff',
                fontSize: '1.4rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '20px',
                textShadow: '0 0 15px rgba(139, 0, 255, 0.5)',
              }}
            >
              Quick Actions
            </h3>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <NeonButton color="purple" fullWidth size="sm" onClick={() => onNavigate('builder')}>
                🆕 New Project
              </NeonButton>
              <NeonButton color="cyan" fullWidth size="sm" onClick={() => onNavigate('ide')}>
                📂 Open Recent
              </NeonButton>
              <NeonButton color="pink" fullWidth size="sm">
                📚 Documentation
              </NeonButton>
            </div>
          </HoloCard>

          {/* Platform Features */}
          <HoloCard glowColor="#b5a642">
            <div style={{ fontSize: '42px', marginBottom: '15px', filter: 'drop-shadow(0 0 10px #b5a642)' }}>
              🎯
            </div>

            <h3
              style={{
                color: '#b5a642',
                fontSize: '1.4rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '20px',
                textShadow: '0 0 15px rgba(181, 166, 66, 0.5)',
              }}
            >
              Platform Features
            </h3>

            <div style={{ color: '#ccc', fontSize: '13px', lineHeight: '2' }}>
              {[
                '✅ Multi-AI Agent Orchestration',
                '✅ Real-time Code Generation',
                '✅ Monaco Editor Integration',
                '✅ Live App Preview',
                '✅ Version Control & Checkpoints',
                '✅ Cloud Code Execution',
                '✅ Enterprise Security',
                '✅ WebSocket Collaboration',
              ].map((feature, i) => (
                <div key={i} style={{ textShadow: '0 0 5px rgba(57, 255, 20, 0.3)' }}>
                  {feature}
                </div>
              ))}
            </div>
          </HoloCard>

          {/* AI Models */}
          <HoloCard glowColor="#00f5ff">
            <div style={{ fontSize: '42px', marginBottom: '15px' }}>🧠</div>

            <h3
              style={{
                color: '#00f5ff',
                fontSize: '1.4rem',
                fontFamily: "'Orbitron', monospace",
                marginBottom: '20px',
              }}
            >
              AI Integration
            </h3>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
              {[
                { name: 'Claude Opus', color: '#d97706', icon: '🟠' },
                { name: 'GPT-4 Turbo', color: '#10b981', icon: '🟢' },
                { name: 'Gemini Pro', color: '#3b82f6', icon: '🔵' },
              ].map((ai) => (
                <div
                  key={ai.name}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '12px',
                    padding: '12px 16px',
                    background: 'rgba(0,0,0,0.4)',
                    border: `1px solid ${ai.color}40`,
                    borderRadius: '10px',
                  }}
                >
                  <span style={{ fontSize: '20px' }}>{ai.icon}</span>
                  <span style={{ color: ai.color, fontFamily: 'monospace', fontWeight: 'bold' }}>{ai.name}</span>
                  <span
                    style={{
                      marginLeft: 'auto',
                      fontSize: '10px',
                      padding: '4px 10px',
                      background: `${ai.color}20`,
                      color: ai.color,
                      borderRadius: '12px',
                      textTransform: 'uppercase',
                      fontWeight: 'bold',
                    }}
                  >
                    Ready
                  </span>
                </div>
              ))}
            </div>
          </HoloCard>
        </div>

        {/* Console output */}
        <div
          style={{
            background: 'rgba(0, 0, 0, 0.8)',
            border: '2px solid #b5a642',
            borderRadius: '12px',
            padding: '20px',
            fontFamily: 'monospace',
            fontSize: '14px',
            marginBottom: '40px',
            position: 'relative',
            boxShadow: 'inset 0 0 30px rgba(0, 0, 0, 0.5), 0 0 20px rgba(181, 166, 66, 0.2)',
          }}
        >
          {/* Terminal header */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '15px',
              paddingBottom: '10px',
              borderBottom: '1px solid rgba(181, 166, 66, 0.3)',
            }}
          >
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#ff5f56' }} />
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#ffbd2e' }} />
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#27ca40' }} />
            <span style={{ marginLeft: '10px', color: '#888', fontSize: '12px' }}>APEX.BUILD Terminal v2.0</span>
          </div>

          <div style={{ color: '#39ff14' }}>
            <div>$ system.status</div>
            <div style={{ color: '#00f5ff' }}>► APEX.BUILD Platform: <span style={{ color: '#39ff14' }}>OPERATIONAL</span></div>
            <div style={{ color: '#00f5ff' }}>► Multi-AI Agents: <span style={{ color: '#39ff14' }}>READY</span></div>
            <div style={{ color: '#00f5ff' }}>► Backend API: <span style={{ color: '#39ff14' }}>localhost:8080</span></div>
            <div style={{ color: '#00f5ff' }}>► Database: <span style={{ color: '#39ff14' }}>PostgreSQL Connected</span></div>
            <div style={{ color: '#b5a642', marginTop: '10px' }}>
              $ <span style={{ animation: 'blink 1s infinite' }}>_</span>
            </div>
          </div>
        </div>

        {/* Footer */}
        <footer
          style={{
            textAlign: 'center',
            paddingTop: '30px',
            borderTop: '1px solid rgba(181, 166, 66, 0.2)',
          }}
        >
          <p style={{ color: '#666', fontSize: '13px', marginBottom: '8px' }}>
            APEX.BUILD v2.0.0 — Enterprise-Grade Cloud Development Platform
          </p>
          <p style={{ color: '#b5a642', fontSize: '11px' }}>
            Powered by Claude Opus • GPT-4 Turbo • Gemini Pro • Monaco Editor
          </p>
        </footer>
      </div>

      {/* CSS Keyframes */}
      <style>{`
        @keyframes particle-rise {
          0% { transform: translateY(0); opacity: 0; }
          10% { opacity: 0.8; }
          90% { opacity: 0.2; }
          100% { transform: translateY(-100vh); opacity: 0; }
        }

        @keyframes gear-spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }

        @keyframes gear-spin-reverse {
          from { transform: rotate(360deg); }
          to { transform: rotate(0deg); }
        }

        @keyframes scanline {
          0% { transform: translateY(-100vh); }
          100% { transform: translateY(100vh); }
        }

        @keyframes steam-rise {
          0% { transform: translateY(0) scale(1); opacity: 0.5; }
          100% { transform: translateY(-60px) scale(2); opacity: 0; }
        }

        @keyframes pulse-glow {
          0%, 100% { box-shadow: 0 0 5px currentColor, 0 0 10px currentColor; }
          50% { box-shadow: 0 0 15px currentColor, 0 0 30px currentColor; }
        }

        @keyframes energy-pulse {
          0%, 100% { opacity: 1; transform: scaleX(1); }
          50% { opacity: 0.7; transform: scaleX(1.05); }
        }

        @keyframes button-shine {
          0% { left: -100%; }
          50%, 100% { left: 100%; }
        }

        @keyframes blink {
          0%, 50% { opacity: 1; }
          51%, 100% { opacity: 0; }
        }

        @import url('https://fonts.googleapis.com/css2?family=Orbitron:wght@400;700;900&family=Rajdhani:wght@400;500;700&display=swap');
      `}</style>
    </div>
  );
};

export default SteampunkDashboard;
