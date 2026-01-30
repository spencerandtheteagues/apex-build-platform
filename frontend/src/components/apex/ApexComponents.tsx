/**
 * APEX.BUILD Cyberpunk Component Library
 * Beautiful, futuristic components that make Replit look ancient
 * Never bland, always stunning with holographic effects
 */

import React, { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import * as THREE from 'three';

// ===== CORE DESIGN TOKENS =====
const APEX_TOKENS = {
  colors: {
    primary: '#00f5ff',      // Electric Cyan
    secondary: '#ff0080',    // Hot Pink
    accent: '#39ff14',       // Acid Green
    purple: '#8a2be2',       // Electric Purple
    surface: '#1a1a2e',      // Dark Steel
    background: '#0a0a0a',   // Deep Space
    text: '#ffffff',         // Neon White
  },

  effects: {
    glassMorphism: 'backdrop-filter: blur(10px)',
    neonGlow: '0 0 20px currentColor',
    smoothTransition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
  },

  animations: {
    fadeIn: { opacity: [0, 1], y: [10, 0] },
    slideUp: { y: [100, 0], opacity: [0, 1] },
    glow: { boxShadow: ['0 0 5px currentColor', '0 0 20px currentColor', '0 0 5px currentColor'] },
  }
};

// ===== UTILITY HOOKS =====
const useHolographicText = (text: string, speed: number = 3000) => {
  const [displayText, setDisplayText] = useState('');
  const [isComplete, setIsComplete] = useState(false);

  useEffect(() => {
    let currentIndex = 0;
    const interval = setInterval(() => {
      if (currentIndex <= text.length) {
        setDisplayText(text.slice(0, currentIndex));
        currentIndex++;
      } else {
        setIsComplete(true);
        clearInterval(interval);
      }
    }, speed / text.length);

    return () => clearInterval(interval);
  }, [text, speed]);

  return { displayText, isComplete };
};

// ===== CYBERPUNK BUTTON COMPONENT =====
interface APEXButtonProps {
  variant?: 'primary' | 'ghost' | 'holographic' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  children: React.ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  icon?: React.ReactNode;
  glowColor?: string;
}

const APEXButton: React.FC<APEXButtonProps> = ({
  variant = 'primary',
  size = 'md',
  children,
  onClick,
  disabled = false,
  icon,
  glowColor
}) => {
  const [isHovered, setIsHovered] = useState(false);

  const buttonVariants = {
    primary: {
      background: `linear-gradient(135deg, ${APEX_TOKENS.colors.primary}, ${APEX_TOKENS.colors.secondary})`,
      color: APEX_TOKENS.colors.background,
      border: 'none',
      boxShadow: `0 0 20px ${glowColor || APEX_TOKENS.colors.primary}33`,
    },
    ghost: {
      background: 'transparent',
      color: APEX_TOKENS.colors.primary,
      border: `1px solid ${APEX_TOKENS.colors.primary}`,
      boxShadow: 'none',
    },
    holographic: {
      background: `linear-gradient(45deg, transparent, ${APEX_TOKENS.colors.primary}1a, transparent)`,
      backgroundSize: '300% 300%',
      color: APEX_TOKENS.colors.text,
      border: `1px solid transparent`,
      borderImage: `linear-gradient(45deg, ${APEX_TOKENS.colors.primary}, ${APEX_TOKENS.colors.secondary}, ${APEX_TOKENS.colors.accent}) 1`,
    },
    danger: {
      background: `linear-gradient(135deg, #ff0040, #ff4080)`,
      color: APEX_TOKENS.colors.text,
      border: 'none',
      boxShadow: '0 0 20px #ff004033',
    }
  };

  const sizeStyles = {
    sm: { padding: '8px 16px', fontSize: '0.875rem' },
    md: { padding: '12px 24px', fontSize: '1rem' },
    lg: { padding: '16px 32px', fontSize: '1.125rem' }
  };

  return (
    <motion.button
      className="apex-button"
      style={{
        ...buttonVariants[variant],
        ...sizeStyles[size],
        borderRadius: '8px',
        fontWeight: 600,
        fontFamily: 'Space Grotesk, sans-serif',
        cursor: disabled ? 'not-allowed' : 'pointer',
        transition: APEX_TOKENS.effects.smoothTransition,
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        position: 'relative',
        overflow: 'hidden',
      }}
      whileHover={!disabled ? {
        scale: 1.02,
        y: -2,
        boxShadow: `0 10px 30px ${glowColor || APEX_TOKENS.colors.primary}66`,
      } : {}}
      whileTap={!disabled ? { scale: 0.98 } : {}}
      onClick={onClick}
      disabled={disabled}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Holographic animation effect */}
      {variant === 'holographic' && (
        <motion.div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background: `linear-gradient(45deg, ${APEX_TOKENS.colors.primary}, ${APEX_TOKENS.colors.secondary}, ${APEX_TOKENS.colors.accent})`,
            backgroundSize: '300% 300%',
            opacity: 0.1,
          }}
          animate={{
            backgroundPosition: ['0% 50%', '100% 50%', '0% 50%'],
          }}
          transition={{
            duration: 3,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        />
      )}

      {icon && <span>{icon}</span>}
      <span style={{ position: 'relative', zIndex: 1 }}>{children}</span>
    </motion.button>
  );
};

// ===== GLASS MORPHISM CARD COMPONENT =====
interface APEXCardProps {
  variant?: 'glass' | 'neon' | 'holographic';
  children: React.ReactNode;
  className?: string;
  glowColor?: string;
  onClick?: () => void;
}

const APEXCard: React.FC<APEXCardProps> = ({
  variant = 'glass',
  children,
  className = '',
  glowColor,
  onClick
}) => {
  const cardVariants = {
    glass: {
      background: 'rgba(26, 26, 46, 0.8)',
      backdropFilter: 'blur(10px)',
      border: `1px solid ${APEX_TOKENS.colors.primary}33`,
      boxShadow: '0 8px 32px rgba(0, 0, 0, 0.3)',
    },
    neon: {
      background: APEX_TOKENS.colors.surface,
      border: `2px solid ${APEX_TOKENS.colors.primary}`,
      boxShadow: `0 0 20px ${APEX_TOKENS.colors.primary}33, inset 0 0 20px ${APEX_TOKENS.colors.primary}1a`,
    },
    holographic: {
      background: `linear-gradient(135deg, ${APEX_TOKENS.colors.primary}0d, ${APEX_TOKENS.colors.secondary}0d)`,
      backdropFilter: 'blur(20px)',
      border: '1px solid transparent',
      borderImage: `linear-gradient(135deg, ${APEX_TOKENS.colors.primary}4d, ${APEX_TOKENS.colors.secondary}4d) 1`,
    }
  };

  return (
    <motion.div
      className={`apex-card ${className}`}
      style={{
        ...cardVariants[variant],
        borderRadius: '12px',
        padding: '24px',
        transition: APEX_TOKENS.effects.smoothTransition,
        cursor: onClick ? 'pointer' : 'default',
        position: 'relative',
        overflow: 'hidden',
      }}
      whileHover={onClick ? {
        scale: 1.02,
        boxShadow: `0 12px 40px ${glowColor || APEX_TOKENS.colors.primary}4d`,
      } : {}}
      onClick={onClick}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
    >
      {children}
    </motion.div>
  );
};

// ===== CYBERPUNK INPUT COMPONENT =====
interface APEXInputProps {
  type?: 'text' | 'password' | 'email' | 'search';
  placeholder?: string;
  value?: string;
  onChange?: (value: string) => void;
  icon?: React.ReactNode;
  label?: string;
  error?: string;
  glowColor?: string;
}

const APEXInput: React.FC<APEXInputProps> = ({
  type = 'text',
  placeholder,
  value,
  onChange,
  icon,
  label,
  error,
  glowColor
}) => {
  const [isFocused, setIsFocused] = useState(false);
  const [isTyping, setIsTyping] = useState(false);

  return (
    <div className="apex-input-wrapper" style={{ marginBottom: '16px' }}>
      {label && (
        <motion.label
          style={{
            display: 'block',
            marginBottom: '8px',
            color: APEX_TOKENS.colors.text,
            fontSize: '0.875rem',
            fontWeight: 500,
            fontFamily: 'Space Grotesk, sans-serif',
            textShadow: isFocused ? `0 0 10px ${glowColor || APEX_TOKENS.colors.primary}` : 'none',
          }}
          animate={{
            color: isFocused ? (glowColor || APEX_TOKENS.colors.primary) : APEX_TOKENS.colors.text,
          }}
        >
          {label}
        </motion.label>
      )}

      <div style={{ position: 'relative' }}>
        {icon && (
          <div style={{
            position: 'absolute',
            left: '12px',
            top: '50%',
            transform: 'translateY(-50%)',
            color: isFocused ? (glowColor || APEX_TOKENS.colors.primary) : '#666',
            transition: APEX_TOKENS.effects.smoothTransition,
          }}>
            {icon}
          </div>
        )}

        <motion.input
          type={type}
          placeholder={placeholder}
          value={value}
          onChange={(e) => {
            onChange?.(e.target.value);
            setIsTyping(e.target.value.length > 0);
          }}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          style={{
            width: '100%',
            background: 'rgba(26, 26, 46, 0.9)',
            border: `1px solid ${isFocused ? (glowColor || APEX_TOKENS.colors.primary) : APEX_TOKENS.colors.primary}4d`,
            borderRadius: '8px',
            color: APEX_TOKENS.colors.text,
            fontSize: '1rem',
            padding: icon ? '12px 16px 12px 44px' : '12px 16px',
            transition: APEX_TOKENS.effects.smoothTransition,
            fontFamily: 'Space Grotesk, sans-serif',
            outline: 'none',
          }}
          animate={{
            boxShadow: isFocused
              ? [
                  `0 0 0 3px ${glowColor || APEX_TOKENS.colors.primary}1a`,
                  `0 0 20px ${glowColor || APEX_TOKENS.colors.primary}33`
                ].join(', ')
              : 'none',
          }}
        />

        {/* Typing indicator */}
        {isTyping && (
          <motion.div
            style={{
              position: 'absolute',
              right: '12px',
              top: '50%',
              transform: 'translateY(-50%)',
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: glowColor || APEX_TOKENS.colors.accent,
            }}
            animate={{
              scale: [1, 1.2, 1],
              opacity: [0.5, 1, 0.5],
            }}
            transition={{
              duration: 1.5,
              repeat: Infinity,
            }}
          />
        )}
      </div>

      {error && (
        <motion.div
          initial={{ opacity: 0, y: -5 }}
          animate={{ opacity: 1, y: 0 }}
          style={{
            color: '#ff0040',
            fontSize: '0.875rem',
            marginTop: '4px',
            fontFamily: 'Space Grotesk, sans-serif',
          }}
        >
          {error}
        </motion.div>
      )}
    </div>
  );
};

// ===== HOLOGRAPHIC TITLE COMPONENT =====
interface APEXTitleProps {
  level?: 1 | 2 | 3 | 4 | 5 | 6;
  children: string;
  animated?: boolean;
  glowColor?: string;
}

const APEXTitle: React.FC<APEXTitleProps> = ({
  level = 1,
  children,
  animated = false,
  glowColor
}) => {
  const { displayText, isComplete } = useHolographicText(children, 2000);
  const Tag = `h${level}` as keyof JSX.IntrinsicElements;

  const sizes = {
    1: '3rem',
    2: '2.25rem',
    3: '1.875rem',
    4: '1.5rem',
    5: '1.25rem',
    6: '1.125rem',
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6 }}
    >
      <Tag
        style={{
          fontSize: sizes[level],
          fontWeight: 700,
          fontFamily: 'Orbitron, Space Grotesk, sans-serif',
          background: `linear-gradient(45deg, ${glowColor || APEX_TOKENS.colors.primary}, ${APEX_TOKENS.colors.secondary}, ${APEX_TOKENS.colors.accent})`,
          backgroundSize: '300% 300%',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          textShadow: `0 0 10px ${glowColor || APEX_TOKENS.colors.primary}66`,
          lineHeight: 1.1,
          margin: 0,
        }}
      >
        <motion.span
          animate={{
            backgroundPosition: ['0% 50%', '100% 50%', '0% 50%'],
          }}
          transition={{
            duration: 3,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        >
          {animated ? displayText : children}
        </motion.span>

        {animated && !isComplete && (
          <motion.span
            style={{
              color: glowColor || APEX_TOKENS.colors.primary,
              opacity: 1,
            }}
            animate={{ opacity: [1, 0] }}
            transition={{ duration: 1, repeat: Infinity }}
          >
            |
          </motion.span>
        )}
      </Tag>
    </motion.div>
  );
};

// ===== CYBERPUNK NAVIGATION COMPONENT =====
interface APEXNavProps {
  items: Array<{
    label: string;
    icon?: React.ReactNode;
    active?: boolean;
    onClick?: () => void;
  }>;
  orientation?: 'horizontal' | 'vertical';
}

const APEXNav: React.FC<APEXNavProps> = ({
  items,
  orientation = 'horizontal'
}) => {
  return (
    <motion.nav
      className="apex-nav"
      style={{
        display: 'flex',
        flexDirection: orientation === 'horizontal' ? 'row' : 'column',
        gap: '8px',
        background: 'rgba(26, 26, 46, 0.9)',
        backdropFilter: 'blur(10px)',
        padding: '16px',
        borderRadius: '12px',
        border: `1px solid ${APEX_TOKENS.colors.primary}33`,
      }}
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4 }}
    >
      {items.map((item, index) => (
        <motion.button
          key={index}
          className="apex-nav-item"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '12px 16px',
            background: item.active
              ? `linear-gradient(135deg, ${APEX_TOKENS.colors.primary}1a, ${APEX_TOKENS.colors.secondary}1a)`
              : 'transparent',
            color: item.active ? APEX_TOKENS.colors.primary : APEX_TOKENS.colors.text,
            border: item.active
              ? `1px solid ${APEX_TOKENS.colors.primary}66`
              : '1px solid transparent',
            borderRadius: '8px',
            fontSize: '0.875rem',
            fontWeight: 500,
            fontFamily: 'Space Grotesk, sans-serif',
            cursor: 'pointer',
            transition: APEX_TOKENS.effects.smoothTransition,
          }}
          whileHover={{
            background: `linear-gradient(135deg, ${APEX_TOKENS.colors.primary}0d, ${APEX_TOKENS.colors.secondary}0d)`,
            border: `1px solid ${APEX_TOKENS.colors.primary}4d`,
            boxShadow: `0 0 15px ${APEX_TOKENS.colors.primary}33`,
          }}
          whileTap={{ scale: 0.98 }}
          onClick={item.onClick}
        >
          {item.icon && <span>{item.icon}</span>}
          <span>{item.label}</span>
        </motion.button>
      ))}
    </motion.nav>
  );
};

// ===== CYBERPUNK LOADING COMPONENT =====
interface APEXLoadingProps {
  size?: 'sm' | 'md' | 'lg';
  variant?: 'spinner' | 'dots' | 'digital';
  color?: string;
}

const APEXLoading: React.FC<APEXLoadingProps> = ({
  size = 'md',
  variant = 'spinner',
  color
}) => {
  const sizes = {
    sm: '24px',
    md: '32px',
    lg: '48px',
  };

  const glowColor = color || APEX_TOKENS.colors.primary;

  if (variant === 'digital') {
    return (
      <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
        {[...Array(8)].map((_, i) => (
          <motion.div
            key={i}
            style={{
              width: '4px',
              height: size === 'sm' ? '20px' : size === 'md' ? '28px' : '40px',
              background: glowColor,
              borderRadius: '2px',
              boxShadow: `0 0 10px ${glowColor}`,
            }}
            animate={{
              scaleY: [0.3, 1, 0.3],
              opacity: [0.3, 1, 0.3],
            }}
            transition={{
              duration: 1,
              repeat: Infinity,
              delay: i * 0.1,
            }}
          />
        ))}
      </div>
    );
  }

  if (variant === 'dots') {
    return (
      <div style={{ display: 'flex', gap: '8px' }}>
        {[...Array(3)].map((_, i) => (
          <motion.div
            key={i}
            style={{
              width: size === 'sm' ? '8px' : size === 'md' ? '12px' : '16px',
              height: size === 'sm' ? '8px' : size === 'md' ? '12px' : '16px',
              borderRadius: '50%',
              background: glowColor,
              boxShadow: `0 0 15px ${glowColor}`,
            }}
            animate={{
              scale: [1, 1.5, 1],
              opacity: [0.5, 1, 0.5],
            }}
            transition={{
              duration: 1,
              repeat: Infinity,
              delay: i * 0.2,
            }}
          />
        ))}
      </div>
    );
  }

  // Default spinner
  return (
    <motion.div
      style={{
        width: sizes[size],
        height: sizes[size],
        border: `3px solid transparent`,
        borderTop: `3px solid ${glowColor}`,
        borderRadius: '50%',
        boxShadow: `0 0 20px ${glowColor}66`,
      }}
      animate={{ rotate: 360 }}
      transition={{
        duration: 1,
        repeat: Infinity,
        ease: 'linear',
      }}
    />
  );
};

// ===== 3D PARTICLE BACKGROUND COMPONENT =====
const APEXParticleBackground: React.FC<{ density?: number }> = ({
  density = 100
}) => {
  const mountRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<THREE.Scene>();
  const rendererRef = useRef<THREE.WebGLRenderer>();

  useEffect(() => {
    if (!mountRef.current) return;

    // Three.js scene setup
    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
    const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });

    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.setClearColor(0x000000, 0);
    mountRef.current.appendChild(renderer.domElement);

    // Create particles
    const particles = new THREE.BufferGeometry();
    const particleCount = density;
    const positions = new Float32Array(particleCount * 3);
    const colors = new Float32Array(particleCount * 3);

    const primaryColor = new THREE.Color(APEX_TOKENS.colors.primary);
    const secondaryColor = new THREE.Color(APEX_TOKENS.colors.secondary);
    const accentColor = new THREE.Color(APEX_TOKENS.colors.accent);

    for (let i = 0; i < particleCount; i++) {
      positions[i * 3] = (Math.random() - 0.5) * 20;
      positions[i * 3 + 1] = (Math.random() - 0.5) * 20;
      positions[i * 3 + 2] = (Math.random() - 0.5) * 20;

      // Random color selection
      const colorChoice = Math.random();
      const selectedColor = colorChoice < 0.33 ? primaryColor :
                           colorChoice < 0.66 ? secondaryColor : accentColor;

      colors[i * 3] = selectedColor.r;
      colors[i * 3 + 1] = selectedColor.g;
      colors[i * 3 + 2] = selectedColor.b;
    }

    particles.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    particles.setAttribute('color', new THREE.BufferAttribute(colors, 3));

    const material = new THREE.PointsMaterial({
      size: 0.05,
      vertexColors: true,
      transparent: true,
      opacity: 0.8,
    });

    const particleSystem = new THREE.Points(particles, material);
    scene.add(particleSystem);

    camera.position.z = 5;

    sceneRef.current = scene;
    rendererRef.current = renderer;

    // Animation loop
    const animate = () => {
      requestAnimationFrame(animate);

      particleSystem.rotation.x += 0.001;
      particleSystem.rotation.y += 0.002;

      renderer.render(scene, camera);
    };

    animate();

    // Handle resize
    const handleResize = () => {
      camera.aspect = window.innerWidth / window.innerHeight;
      camera.updateProjectionMatrix();
      renderer.setSize(window.innerWidth, window.innerHeight);
    };

    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      if (mountRef.current && renderer.domElement) {
        mountRef.current.removeChild(renderer.domElement);
      }
      renderer.dispose();
    };
  }, [density]);

  return (
    <div
      ref={mountRef}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        zIndex: -1,
        pointerEvents: 'none',
      }}
    />
  );
};

// ===== THEME PROVIDER =====
interface APEXThemeContextType {
  theme: 'cyberpunk' | 'matrix' | 'synthwave' | 'neonCity';
  setTheme: (theme: 'cyberpunk' | 'matrix' | 'synthwave' | 'neonCity') => void;
}

const APEXThemeContext = React.createContext<APEXThemeContextType | undefined>(undefined);

const APEXThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [theme, setTheme] = useState<'cyberpunk' | 'matrix' | 'synthwave' | 'neonCity'>('cyberpunk');

  return (
    <APEXThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </APEXThemeContext.Provider>
  );
};

const useAPEXTheme = () => {
  const context = React.useContext(APEXThemeContext);
  if (!context) {
    throw new Error('useAPEXTheme must be used within APEXThemeProvider');
  }
  return context;
};

// ===== EXPORT ALL COMPONENTS =====
export {
  APEX_TOKENS,
  useHolographicText,
  APEXButton,
  APEXCard,
  APEXInput,
  APEXTitle,
  APEXNav,
  APEXLoading,
  APEXParticleBackground,
  APEXThemeProvider,
  useAPEXTheme,
};
