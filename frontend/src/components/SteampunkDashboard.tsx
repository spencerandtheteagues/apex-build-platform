import React, { useState, useEffect, useRef } from 'react';

interface SteampunkDashboardProps {
  onNavigate: (view: 'ide' | 'builder' | 'projects' | 'settings') => void;
}

// Network node for topology visualization
interface NetworkNode {
  id: number;
  x: number;
  y: number;
  vx: number;
  vy: number;
  type: 'primary' | 'secondary' | 'threat';
}

// Status Card Component
const StatusCard: React.FC<{
  title: string;
  value: string | number;
  subtitle?: string;
  icon?: React.ReactNode;
  trend?: string;
}> = ({ title, value, subtitle, icon, trend }) => (
  <div
    style={{
      background: 'rgba(20, 20, 20, 0.8)',
      border: '1px solid rgba(255, 50, 50, 0.2)',
      borderRadius: '8px',
      padding: '20px 24px',
      minWidth: '180px',
    }}
  >
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
      <span style={{ color: '#666', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '1px' }}>
        {title}
      </span>
      {icon && <span style={{ color: '#ff3333', fontSize: '14px' }}>{icon}</span>}
    </div>
    <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px' }}>
      <span style={{ color: '#fff', fontSize: '32px', fontWeight: '300', fontFamily: 'monospace' }}>
        {value}
      </span>
      {trend && (
        <span style={{ color: '#4ade80', fontSize: '12px' }}>{trend}</span>
      )}
    </div>
    {subtitle && (
      <span style={{ color: subtitle === 'CRITICAL' ? '#ff3333' : '#4ade80', fontSize: '12px', fontWeight: 'bold' }}>
        {subtitle}
      </span>
    )}
  </div>
);

// Navigation Item Component
const NavItem: React.FC<{
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  onClick?: () => void;
}> = ({ icon, label, active, onClick }) => (
  <div
    onClick={onClick}
    style={{
      display: 'flex',
      alignItems: 'center',
      gap: '12px',
      padding: '14px 20px',
      cursor: 'pointer',
      background: active ? 'rgba(255, 50, 50, 0.15)' : 'transparent',
      borderLeft: active ? '3px solid #ff3333' : '3px solid transparent',
      borderRadius: '0 8px 8px 0',
      transition: 'all 0.2s ease',
      marginBottom: '4px',
    }}
  >
    <span style={{ color: active ? '#ff3333' : '#888', fontSize: '18px' }}>{icon}</span>
    <span style={{ color: active ? '#ff3333' : '#888', fontSize: '14px', fontWeight: active ? '500' : '400' }}>
      {label}
    </span>
  </div>
);

// Alert Item Component
const AlertItem: React.FC<{
  title: string;
  description: string;
  time: string;
}> = ({ title, description, time }) => (
  <div
    style={{
      padding: '16px',
      borderBottom: '1px solid rgba(255, 50, 50, 0.1)',
    }}
  >
    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '6px' }}>
      <div style={{
        width: '24px',
        height: '24px',
        borderRadius: '6px',
        background: 'rgba(255, 50, 50, 0.2)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: '12px',
      }}>
        ⚠️
      </div>
      <span style={{ color: '#fff', fontSize: '14px', fontWeight: '500' }}>{title}</span>
    </div>
    <p style={{ color: '#888', fontSize: '12px', margin: '0 0 6px 34px', lineHeight: '1.4' }}>
      {description}
    </p>
    <span style={{ color: '#555', fontSize: '11px', marginLeft: '34px' }}>{time}</span>
  </div>
);

export const SteampunkDashboard: React.FC<SteampunkDashboardProps> = ({ onNavigate }) => {
  const [activeNav, setActiveNav] = useState('intelligence');
  const [backendStatus, setBackendStatus] = useState<'online' | 'offline'>('offline');
  const [terminalLines, setTerminalLines] = useState<string[]>([]);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const nodesRef = useRef<NetworkNode[]>([]);

  // Check backend status
  useEffect(() => {
    const checkBackend = async () => {
      try {
        const response = await fetch('http://localhost:8080/health');
        setBackendStatus(response.ok ? 'online' : 'offline');
      } catch {
        setBackendStatus('offline');
      }
    };
    checkBackend();
    const interval = setInterval(checkBackend, 10000);
    return () => clearInterval(interval);
  }, []);

  // Terminal simulation
  useEffect(() => {
    const lines = [
      '[SUCCESS] Node 84.12.99.1 connected via Tor circuit #442',
      '[INFO] Handshake complete. Encryption: AES-256-GCM',
      '[WARNING] Anomalous traffic detected on port 9050',
      '[INFO] Rerouting traffic through secondary relay...',
      '[SUCCESS] Route established. Latency: 142ms',
      '[INFO] Decrypting payload stream...',
    ];

    let index = 0;
    const interval = setInterval(() => {
      if (index < lines.length) {
        setTerminalLines(prev => [...prev, lines[index]]);
        index++;
      }
    }, 800);

    return () => clearInterval(interval);
  }, []);

  // Network topology canvas
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const resize = () => {
      canvas.width = canvas.offsetWidth;
      canvas.height = canvas.offsetHeight;
    };
    resize();
    window.addEventListener('resize', resize);

    // Initialize nodes
    if (nodesRef.current.length === 0) {
      for (let i = 0; i < 25; i++) {
        nodesRef.current.push({
          id: i,
          x: Math.random() * canvas.width,
          y: Math.random() * canvas.height,
          vx: (Math.random() - 0.5) * 0.5,
          vy: (Math.random() - 0.5) * 0.5,
          type: Math.random() > 0.85 ? 'threat' : Math.random() > 0.5 ? 'primary' : 'secondary',
        });
      }
    }

    const animate = () => {
      ctx.fillStyle = 'rgba(10, 10, 10, 0.15)';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      const nodes = nodesRef.current;

      // Draw connections
      nodes.forEach((node, i) => {
        nodes.slice(i + 1).forEach(other => {
          const dx = node.x - other.x;
          const dy = node.y - other.y;
          const dist = Math.sqrt(dx * dx + dy * dy);

          if (dist < 120) {
            ctx.beginPath();
            ctx.moveTo(node.x, node.y);
            ctx.lineTo(other.x, other.y);
            const alpha = (1 - dist / 120) * 0.4;
            ctx.strokeStyle = node.type === 'threat' || other.type === 'threat'
              ? `rgba(255, 50, 50, ${alpha})`
              : `rgba(100, 100, 100, ${alpha * 0.5})`;
            ctx.lineWidth = node.type === 'threat' || other.type === 'threat' ? 1.5 : 0.5;
            ctx.stroke();
          }
        });
      });

      // Draw and update nodes
      nodes.forEach(node => {
        node.x += node.vx;
        node.y += node.vy;

        if (node.x < 0 || node.x > canvas.width) node.vx *= -1;
        if (node.y < 0 || node.y > canvas.height) node.vy *= -1;

        ctx.beginPath();
        ctx.arc(node.x, node.y, node.type === 'threat' ? 5 : 3, 0, Math.PI * 2);
        ctx.fillStyle = node.type === 'threat' ? '#ff3333' : node.type === 'primary' ? '#666' : '#444';
        ctx.fill();

        if (node.type === 'threat') {
          ctx.beginPath();
          ctx.arc(node.x, node.y, 8, 0, Math.PI * 2);
          ctx.strokeStyle = 'rgba(255, 50, 50, 0.3)';
          ctx.lineWidth = 2;
          ctx.stroke();
        }
      });

      requestAnimationFrame(animate);
    };

    animate();
    return () => window.removeEventListener('resize', resize);
  }, []);

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: '#0a0a0a', color: '#fff', fontFamily: "'Inter', -apple-system, sans-serif" }}>
      {/* Sidebar */}
      <aside style={{ width: '240px', background: '#0f0f0f', borderRight: '1px solid rgba(255, 50, 50, 0.1)', padding: '20px 0' }}>
        {/* Logo */}
        <div style={{ padding: '10px 20px 30px', display: 'flex', alignItems: 'center', gap: '10px' }}>
          <span style={{ color: '#ff3333', fontSize: '24px' }}>⚡</span>
          <span style={{ color: '#ff3333', fontSize: '20px', fontWeight: 'bold', letterSpacing: '2px' }}>
            APEX.BUILD
          </span>
        </div>

        {/* Navigation */}
        <nav>
          <NavItem
            icon="🌐"
            label="Intelligence"
            active={activeNav === 'intelligence'}
            onClick={() => setActiveNav('intelligence')}
          />
          <NavItem
            icon="⚠️"
            label="App Builder"
            active={activeNav === 'builder'}
            onClick={() => { setActiveNav('builder'); onNavigate('builder'); }}
          />
          <NavItem
            icon="📊"
            label="IDE"
            active={activeNav === 'ide'}
            onClick={() => { setActiveNav('ide'); onNavigate('ide'); }}
          />
          <NavItem
            icon="⚙️"
            label="System"
            active={activeNav === 'system'}
            onClick={() => setActiveNav('system')}
          />
        </nav>

        {/* Status footer */}
        <div style={{ position: 'absolute', bottom: '20px', left: '0', width: '240px', padding: '0 20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: '#666', marginBottom: '8px' }}>
            <span>STATUS</span>
            <span style={{ color: backendStatus === 'online' ? '#4ade80' : '#ff3333' }}>
              {backendStatus.toUpperCase()}
            </span>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: '#666' }}>
            <span>BACKEND</span>
            <span style={{ color: backendStatus === 'online' ? '#4ade80' : '#ff3333' }}>
              {backendStatus === 'online' ? 'CONNECTED' : 'DISCONNECTED'}
            </span>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main style={{ flex: 1, padding: '30px 40px', overflow: 'auto' }}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '30px' }}>
          <div>
            <h1 style={{
              fontSize: '36px',
              fontWeight: '700',
              margin: '0 0 8px 0',
              fontFamily: "'Orbitron', monospace",
              letterSpacing: '2px',
            }}>
              DASHBOARD <span style={{ color: '#ff3333' }}>::</span>
            </h1>
            <h2 style={{ fontSize: '28px', fontWeight: '300', margin: 0, color: '#ccc' }}>
              OVERVIEW
            </h2>
            <p style={{ color: '#666', fontSize: '14px', marginTop: '8px', fontFamily: 'monospace' }}>
              System Integrity: <span style={{ color: '#4ade80' }}>98.4%</span> | Active Threads: <span style={{ color: '#ff3333' }}>4</span>
            </p>
          </div>

          <div style={{ display: 'flex', gap: '12px' }}>
            <button
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: '8px',
                padding: '12px 24px',
                color: '#fff',
                fontSize: '13px',
                fontWeight: '500',
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
            >
              EXPORT LOGS
            </button>
            <button
              onClick={() => onNavigate('builder')}
              style={{
                background: '#ff3333',
                border: 'none',
                borderRadius: '8px',
                padding: '12px 24px',
                color: '#fff',
                fontSize: '13px',
                fontWeight: '600',
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
            >
              BUILD APP
            </button>
          </div>
        </div>

        {/* Stats Row */}
        <div style={{ display: 'flex', gap: '16px', marginBottom: '30px', flexWrap: 'wrap' }}>
          <StatusCard
            title="ACTIVE NODES"
            value="1,024"
            trend="+12.5%"
            icon="📡"
          />
          <StatusCard
            title="THREAT LEVEL"
            value="HIGH"
            subtitle="CRITICAL"
            icon="🛡️"
          />
          <StatusCard
            title="AI REQUESTS"
            value="8.4M"
            trend="+2.1%"
            icon="🔄"
          />
          <StatusCard
            title="BUILDS TODAY"
            value="42"
            subtitle="STABLE"
            icon="🔧"
          />
        </div>

        {/* Main Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: '24px' }}>
          {/* Left Column - Network & Terminal */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
            {/* Network Topology */}
            <div style={{
              background: 'rgba(20, 20, 20, 0.6)',
              border: '1px solid rgba(255, 50, 50, 0.15)',
              borderRadius: '12px',
              padding: '20px',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                <h3 style={{ margin: 0, fontSize: '14px', fontWeight: '600', letterSpacing: '1px' }}>
                  LIVE NETWORK TOPOLOGY
                </h3>
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                  <select
                    style={{
                      background: 'rgba(0,0,0,0.3)',
                      border: '1px solid rgba(255,255,255,0.1)',
                      borderRadius: '6px',
                      padding: '6px 12px',
                      color: '#fff',
                      fontSize: '12px',
                    }}
                  >
                    <option>Global View</option>
                  </select>
                </div>
              </div>
              <div style={{ position: 'relative', height: '280px', borderRadius: '8px', overflow: 'hidden', background: '#0a0a0a' }}>
                <canvas
                  ref={canvasRef}
                  style={{ width: '100%', height: '100%' }}
                />
                <div style={{
                  position: 'absolute',
                  top: '12px',
                  right: '12px',
                  background: 'rgba(0,0,0,0.7)',
                  border: '1px solid rgba(255, 50, 50, 0.3)',
                  borderRadius: '6px',
                  padding: '8px 14px',
                  fontSize: '11px',
                  color: '#ff3333',
                  fontFamily: 'monospace',
                }}>
                  LIVE FEED :: SECURE
                </div>
              </div>
            </div>

            {/* Terminal */}
            <div style={{
              background: 'rgba(10, 10, 10, 0.9)',
              border: '1px solid rgba(255, 50, 50, 0.2)',
              borderLeft: '3px solid #ff3333',
              borderRadius: '8px',
              padding: '16px 20px',
              fontFamily: 'monospace',
              fontSize: '13px',
              maxHeight: '200px',
              overflow: 'auto',
            }}>
              {terminalLines.map((line, i) => (
                <div
                  key={i}
                  style={{
                    color: line.includes('[WARNING]') ? '#ff3333' :
                           line.includes('[SUCCESS]') ? '#4ade80' :
                           line.includes('[INFO]') ? '#888' : '#fff',
                    marginBottom: '6px',
                    lineHeight: '1.5',
                  }}
                >
                  {line}
                </div>
              ))}
              <span style={{ color: '#ff3333' }}>_</span>
            </div>
          </div>

          {/* Right Column - Alerts & AI Status */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
            {/* Recent Alerts */}
            <div style={{
              background: 'rgba(20, 20, 20, 0.6)',
              border: '1px solid rgba(255, 50, 50, 0.15)',
              borderRadius: '12px',
              overflow: 'hidden',
            }}>
              <div style={{ padding: '16px 20px', borderBottom: '1px solid rgba(255, 50, 50, 0.1)' }}>
                <h3 style={{ margin: 0, fontSize: '14px', fontWeight: '600', letterSpacing: '1px' }}>
                  RECENT ALERTS
                </h3>
              </div>
              <AlertItem
                title="Build Complete"
                description="Project 'todo-app' deployed successfully"
                time="2 mins ago"
              />
              <AlertItem
                title="AI Response"
                description="Claude generated 12 files for new feature"
                time="5 mins ago"
              />
              <AlertItem
                title="System Update"
                description="Backend API restarted with new config"
                time="12 mins ago"
              />
            </div>

            {/* AI Integration Status */}
            <div style={{
              background: 'rgba(20, 20, 20, 0.6)',
              border: '1px solid rgba(255, 50, 50, 0.15)',
              borderRadius: '12px',
              padding: '20px',
            }}>
              <h3 style={{ margin: '0 0 16px 0', fontSize: '14px', fontWeight: '600', letterSpacing: '1px' }}>
                AI INTEGRATION
              </h3>
              {[
                { name: 'Claude Opus', color: '#d97706', status: 'Ready' },
                { name: 'GPT-4 Turbo', color: '#4ade80', status: 'Ready' },
                { name: 'Gemini Pro', color: '#3b82f6', status: 'Ready' },
              ].map((ai) => (
                <div
                  key={ai.name}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '12px 14px',
                    background: 'rgba(0,0,0,0.3)',
                    borderRadius: '8px',
                    marginBottom: '8px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <div style={{
                      width: '10px',
                      height: '10px',
                      borderRadius: '50%',
                      background: ai.color,
                    }} />
                    <span style={{ color: ai.color, fontFamily: 'monospace', fontSize: '13px' }}>
                      {ai.name}
                    </span>
                  </div>
                  <span style={{
                    fontSize: '10px',
                    padding: '4px 10px',
                    background: 'rgba(74, 222, 128, 0.15)',
                    color: '#4ade80',
                    borderRadius: '12px',
                    fontWeight: '600',
                  }}>
                    {ai.status.toUpperCase()}
                  </span>
                </div>
              ))}
            </div>

            {/* Quick Actions */}
            <div style={{
              background: 'rgba(20, 20, 20, 0.6)',
              border: '1px solid rgba(255, 50, 50, 0.15)',
              borderRadius: '12px',
              padding: '20px',
            }}>
              <h3 style={{ margin: '0 0 16px 0', fontSize: '14px', fontWeight: '600', letterSpacing: '1px' }}>
                QUICK ACTIONS
              </h3>
              <button
                onClick={() => onNavigate('builder')}
                style={{
                  width: '100%',
                  background: 'linear-gradient(135deg, #ff3333, #cc0000)',
                  border: 'none',
                  borderRadius: '8px',
                  padding: '14px',
                  color: '#fff',
                  fontSize: '13px',
                  fontWeight: '600',
                  cursor: 'pointer',
                  marginBottom: '10px',
                  letterSpacing: '1px',
                }}
              >
                🚀 NEW BUILD
              </button>
              <button
                onClick={() => onNavigate('ide')}
                style={{
                  width: '100%',
                  background: 'rgba(255, 255, 255, 0.05)',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                  borderRadius: '8px',
                  padding: '14px',
                  color: '#fff',
                  fontSize: '13px',
                  fontWeight: '500',
                  cursor: 'pointer',
                  letterSpacing: '1px',
                }}
              >
                💻 OPEN IDE
              </button>
            </div>
          </div>
        </div>
      </main>

      {/* Google Fonts */}
      <style>{`
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Orbitron:wght@400;700&display=swap');
      `}</style>
    </div>
  );
};

export default SteampunkDashboard;
