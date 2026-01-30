// APEX.BUILD Premium Input Component
// Futuristic form fields with stunning animations and effects

import React, { forwardRef, useState, useRef, useEffect, useCallback } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { Eye, EyeOff, Check, AlertCircle, AlertTriangle, Terminal, Sparkles } from 'lucide-react'

// CSS-in-JS styles for advanced animations
const inputStyles = `
  @keyframes input-glow-pulse {
    0%, 100% {
      box-shadow: 0 0 5px var(--glow-color, #cc0000),
                  0 0 10px var(--glow-color, #cc0000),
                  0 0 20px color-mix(in srgb, var(--glow-color, #cc0000) 50%, transparent);
    }
    50% {
      box-shadow: 0 0 10px var(--glow-color, #cc0000),
                  0 0 20px var(--glow-color, #cc0000),
                  0 0 40px color-mix(in srgb, var(--glow-color, #cc0000) 50%, transparent);
    }
  }

  @keyframes input-border-flow {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes cursor-blink {
    0%, 50% { opacity: 1; }
    51%, 100% { opacity: 0; }
  }

  @keyframes typing-pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.02); }
  }

  @keyframes shake-error {
    0%, 100% { transform: translateX(0); }
    10%, 30%, 50%, 70%, 90% { transform: translateX(-4px); }
    20%, 40%, 60%, 80% { transform: translateX(4px); }
  }

  @keyframes success-pulse {
    0% { box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.7); }
    70% { box-shadow: 0 0 0 10px rgba(34, 197, 94, 0); }
    100% { box-shadow: 0 0 0 0 rgba(34, 197, 94, 0); }
  }

  @keyframes label-float {
    0% { transform: translateY(0) scale(1); }
    100% { transform: translateY(-1.75rem) scale(0.85); }
  }

  @keyframes placeholder-fade {
    0% { opacity: 1; transform: translateX(0); }
    100% { opacity: 0; transform: translateX(10px); }
  }

  @keyframes icon-slide-in {
    0% { opacity: 0; transform: translateX(-10px); }
    100% { opacity: 1; transform: translateX(0); }
  }

  @keyframes icon-bounce {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-2px); }
  }

  @keyframes holographic-shift {
    0% { filter: hue-rotate(0deg); }
    100% { filter: hue-rotate(360deg); }
  }

  @keyframes circuit-trace {
    0% { stroke-dashoffset: 1000; opacity: 0.3; }
    50% { opacity: 1; }
    100% { stroke-dashoffset: 0; opacity: 0.3; }
  }

  @keyframes counter-glow {
    0%, 100% { text-shadow: 0 0 5px currentColor; }
    50% { text-shadow: 0 0 15px currentColor, 0 0 25px currentColor; }
  }

  @keyframes dropdown-slide {
    0% { opacity: 0; transform: translateY(-10px) scale(0.95); }
    100% { opacity: 1; transform: translateY(0) scale(1); }
  }

  @keyframes item-highlight {
    0% { background-position: -100% 0; }
    100% { background-position: 100% 0; }
  }

  .input-glow-pulse {
    animation: input-glow-pulse 2s ease-in-out infinite;
  }

  .input-typing-pulse {
    animation: typing-pulse 0.15s ease-in-out;
  }

  .input-shake-error {
    animation: shake-error 0.5s ease-in-out;
  }

  .input-success-pulse {
    animation: success-pulse 1s ease-out;
  }

  .label-float-active {
    animation: label-float 0.2s ease-out forwards;
  }

  .placeholder-fade-out {
    animation: placeholder-fade 0.2s ease-out forwards;
  }

  .icon-slide-in {
    animation: icon-slide-in 0.3s ease-out forwards;
  }

  .icon-focus-bounce {
    animation: icon-bounce 0.5s ease-in-out infinite;
  }

  .holographic-border {
    background: linear-gradient(90deg, #ff0080, #00ff80, #0080ff, #ff0080);
    background-size: 300% 100%;
    animation: input-border-flow 3s linear infinite, holographic-shift 5s linear infinite;
  }

  .circuit-bg {
    background-image: url("data:image/svg+xml,%3Csvg width='100' height='100' viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M10 10h80v80H10z' fill='none' stroke='%23cc0000' stroke-width='0.5' opacity='0.2'/%3E%3Cpath d='M30 10v20h40v20H30v20h40' fill='none' stroke='%23cc0000' stroke-width='0.5' opacity='0.3'/%3E%3Ccircle cx='30' cy='30' r='3' fill='%23cc0000' opacity='0.3'/%3E%3Ccircle cx='70' cy='50' r='3' fill='%23cc0000' opacity='0.3'/%3E%3Ccircle cx='70' cy='70' r='3' fill='%23cc0000' opacity='0.3'/%3E%3C/svg%3E");
    background-size: 50px 50px;
  }

  .counter-warning {
    animation: counter-glow 1s ease-in-out infinite;
  }

  .dropdown-enter {
    animation: dropdown-slide 0.2s ease-out forwards;
  }

  .dropdown-item-highlight {
    background: linear-gradient(90deg, transparent, rgba(204, 0, 0, 0.2), transparent);
    background-size: 200% 100%;
    animation: item-highlight 0.5s ease-out;
  }
`

const inputVariants = cva(
  'flex w-full rounded-lg border bg-transparent px-4 py-3 text-sm transition-all duration-300 placeholder:transition-all placeholder:duration-300 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50',
  {
    variants: {
      variant: {
        default: [
          'border-gray-700 bg-gray-900/50 text-white',
          'placeholder:text-gray-500',
          'hover:border-gray-600 hover:bg-gray-900/70',
          'focus:border-red-500 focus:bg-gray-900/80',
          'focus:shadow-[0_0_0_3px_rgba(204,0,0,0.15),0_0_20px_rgba(204,0,0,0.3)]',
        ],
        apex: [
          'border-red-500/50 bg-black/70 text-red-50',
          'placeholder:text-red-900/50',
          'hover:border-red-400/70 hover:bg-black/80',
          'focus:border-red-500 focus:bg-black/90',
          'focus:shadow-[0_0_0_3px_rgba(204,0,0,0.2),0_0_30px_rgba(204,0,0,0.4)]',
        ],
        cyber: [
          'border-red-600/30 bg-black/90 text-red-100',
          'placeholder:text-red-900/40',
          'hover:border-red-500/50',
          'focus:border-red-500',
          'focus:shadow-[0_0_0_2px_rgba(204,0,0,0.3),0_0_25px_rgba(204,0,0,0.5)]',
          'clip-path-cyber',
        ],
        holographic: [
          'border-2 border-transparent bg-black/80 text-white',
          'placeholder:text-gray-400',
          'hover:bg-black/90',
          'focus:bg-black',
        ],
        terminal: [
          'border-green-500/50 bg-black text-green-400 font-mono',
          'placeholder:text-green-900/50',
          'hover:border-green-400/70',
          'focus:border-green-500',
          'focus:shadow-[0_0_0_2px_rgba(34,197,94,0.2),0_0_20px_rgba(34,197,94,0.3)]',
        ],
        cyberpunk: [
          'border-cyan-500/50 bg-gray-900/70 text-cyan-100',
          'placeholder:text-cyan-900/50',
          'hover:border-cyan-400/70',
          'focus:border-cyan-400 focus:shadow-lg focus:shadow-cyan-500/25',
        ],
        matrix: [
          'border-green-500/50 bg-black/70 text-green-400',
          'placeholder:text-green-900/50',
          'focus:border-green-400 focus:ring-green-400/50 focus:shadow-lg focus:shadow-green-500/25',
        ],
        synthwave: [
          'border-pink-500/50 bg-purple-900/50 text-pink-100',
          'placeholder:text-pink-900/50',
          'focus:border-pink-400 focus:ring-pink-400/50 focus:shadow-lg focus:shadow-pink-500/25',
        ],
        neonCity: [
          'border-blue-500/50 bg-blue-900/30 text-blue-100',
          'placeholder:text-blue-900/50',
          'focus:border-blue-400 focus:ring-blue-400/50 focus:shadow-lg focus:shadow-blue-500/25',
        ],
      },
      size: {
        sm: 'h-9 px-3 text-xs',
        md: 'h-11 px-4 text-sm',
        lg: 'h-14 px-5 text-base',
        xl: 'h-16 px-6 text-lg',
      },
      validation: {
        none: '',
        success: [
          'border-green-500 bg-green-900/10',
          'focus:border-green-400 focus:shadow-[0_0_0_3px_rgba(34,197,94,0.2),0_0_20px_rgba(34,197,94,0.3)]',
        ],
        error: [
          'border-red-500 bg-red-900/10',
          'focus:border-red-400 focus:shadow-[0_0_0_3px_rgba(239,68,68,0.2),0_0_20px_rgba(239,68,68,0.3)]',
        ],
        warning: [
          'border-amber-500 bg-amber-900/10',
          'focus:border-amber-400 focus:shadow-[0_0_0_3px_rgba(245,158,11,0.2),0_0_20px_rgba(245,158,11,0.3)]',
        ],
      },
    },
    defaultVariants: {
      variant: 'apex',
      size: 'md',
      validation: 'none',
    },
  }
)

export interface AutocompleteOption {
  value: string
  label: string
  icon?: React.ReactNode
  description?: string
}

export interface InputProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'>,
    VariantProps<typeof inputVariants> {
  label?: string
  floatingLabel?: boolean
  error?: string
  success?: string
  warning?: string
  helperText?: string
  leftIcon?: React.ReactNode
  rightIcon?: React.ReactNode
  showPasswordToggle?: boolean
  showCharacterCount?: boolean
  autocompleteOptions?: AutocompleteOption[]
  onAutocompleteSelect?: (option: AutocompleteOption) => void
  glowOnFocus?: boolean
  pulseOnType?: boolean
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  (
    {
      className,
      variant,
      size,
      validation,
      type,
      label,
      floatingLabel = false,
      error,
      success,
      warning,
      helperText,
      leftIcon,
      rightIcon,
      showPasswordToggle = false,
      showCharacterCount = false,
      maxLength,
      autocompleteOptions = [],
      onAutocompleteSelect,
      glowOnFocus = true,
      pulseOnType = true,
      onChange,
      ...props
    },
    ref
  ) => {
    const [showPassword, setShowPassword] = useState(false)
    const [isFocused, setIsFocused] = useState(false)
    const [hasValue, setHasValue] = useState(!!props.value || !!props.defaultValue)
    const [isTyping, setIsTyping] = useState(false)
    const [charCount, setCharCount] = useState(
      typeof props.value === 'string' ? props.value.length :
      typeof props.defaultValue === 'string' ? props.defaultValue.length : 0
    )
    const [showAutocomplete, setShowAutocomplete] = useState(false)
    const [filteredOptions, setFilteredOptions] = useState<AutocompleteOption[]>([])
    const [highlightedIndex, setHighlightedIndex] = useState(-1)
    const [shakeError, setShakeError] = useState(false)
    const inputRef = useRef<HTMLInputElement>(null)
    const typingTimeoutRef = useRef<NodeJS.Timeout>()
    const containerRef = useRef<HTMLDivElement>(null)

    // Inject styles
    useEffect(() => {
      const styleId = 'apex-input-styles'
      if (!document.getElementById(styleId)) {
        const styleEl = document.createElement('style')
        styleEl.id = styleId
        styleEl.textContent = inputStyles
        document.head.appendChild(styleEl)
      }
    }, [])

    // Handle error shake animation
    useEffect(() => {
      if (error) {
        setShakeError(true)
        const timer = setTimeout(() => setShakeError(false), 500)
        return () => clearTimeout(timer)
      }
    }, [error])

    const inputType = type === 'password' && showPasswordToggle
      ? (showPassword ? 'text' : 'password')
      : type

    // Determine validation state
    const currentValidation = error ? 'error' : success ? 'success' : warning ? 'warning' : validation

    // Handle input change
    const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value
      setHasValue(value.length > 0)
      setCharCount(value.length)

      // Typing animation
      if (pulseOnType) {
        setIsTyping(true)
        if (typingTimeoutRef.current) {
          clearTimeout(typingTimeoutRef.current)
        }
        typingTimeoutRef.current = setTimeout(() => setIsTyping(false), 150)
      }

      // Autocomplete filtering
      if (autocompleteOptions.length > 0) {
        const filtered = autocompleteOptions.filter(
          option => option.label.toLowerCase().includes(value.toLowerCase()) ||
                   option.value.toLowerCase().includes(value.toLowerCase())
        )
        setFilteredOptions(filtered)
        setShowAutocomplete(filtered.length > 0 && value.length > 0)
        setHighlightedIndex(-1)
      }

      onChange?.(e)
    }, [onChange, pulseOnType, autocompleteOptions])

    // Handle autocomplete selection
    const handleAutocompleteSelect = useCallback((option: AutocompleteOption) => {
      if (inputRef.current) {
        const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
          window.HTMLInputElement.prototype,
          'value'
        )?.set
        nativeInputValueSetter?.call(inputRef.current, option.value)
        const event = new Event('input', { bubbles: true })
        inputRef.current.dispatchEvent(event)
      }
      setShowAutocomplete(false)
      onAutocompleteSelect?.(option)
    }, [onAutocompleteSelect])

    // Handle keyboard navigation
    const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
      if (!showAutocomplete) return

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault()
          setHighlightedIndex(prev =>
            prev < filteredOptions.length - 1 ? prev + 1 : 0
          )
          break
        case 'ArrowUp':
          e.preventDefault()
          setHighlightedIndex(prev =>
            prev > 0 ? prev - 1 : filteredOptions.length - 1
          )
          break
        case 'Enter':
          e.preventDefault()
          if (highlightedIndex >= 0) {
            handleAutocompleteSelect(filteredOptions[highlightedIndex])
          }
          break
        case 'Escape':
          setShowAutocomplete(false)
          break
      }

      props.onKeyDown?.(e)
    }, [showAutocomplete, filteredOptions, highlightedIndex, handleAutocompleteSelect, props])

    // Close autocomplete on outside click
    useEffect(() => {
      const handleClickOutside = (e: MouseEvent) => {
        if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
          setShowAutocomplete(false)
        }
      }
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }, [])

    // Character count percentage for warning
    const charPercentage = maxLength ? (charCount / maxLength) * 100 : 0
    const isNearLimit = charPercentage >= 80
    const isAtLimit = charPercentage >= 100

    // Get glow color based on variant
    const getGlowColor = () => {
      if (currentValidation === 'error') return '#ef4444'
      if (currentValidation === 'success') return '#22c55e'
      if (currentValidation === 'warning') return '#f59e0b'
      switch (variant) {
        case 'terminal': return '#22c55e'
        case 'cyberpunk': return '#00ffff'
        case 'matrix': return '#00ff00'
        case 'synthwave': return '#ff00ff'
        case 'neonCity': return '#3b82f6'
        default: return '#cc0000'
      }
    }

    return (
      <div ref={containerRef} className="relative space-y-2">
        {/* Style injection for dynamic glow color */}
        <style>{`
          .input-container-${props.id || 'default'} {
            --glow-color: ${getGlowColor()};
          }
        `}</style>

        {/* Label - Standard or Floating */}
        {label && !floatingLabel && (
          <label
            className={cn(
              "text-sm font-medium block transition-all duration-300",
              isFocused ? "text-red-400" : "text-gray-400",
              currentValidation === 'error' && "text-red-400",
              currentValidation === 'success' && "text-green-400",
              currentValidation === 'warning' && "text-amber-400"
            )}
          >
            {label}
          </label>
        )}

        <div
          className={cn(
            "relative group",
            `input-container-${props.id || 'default'}`
          )}
        >
          {/* Holographic border wrapper */}
          {variant === 'holographic' && (
            <div
              className={cn(
                "absolute -inset-[2px] rounded-lg opacity-0 transition-opacity duration-300",
                "holographic-border",
                isFocused && "opacity-100"
              )}
            />
          )}

          {/* Circuit pattern background for cyber variant */}
          {variant === 'cyber' && (
            <div className="absolute inset-0 rounded-lg circuit-bg opacity-30 pointer-events-none" />
          )}

          {/* Terminal prompt for terminal variant */}
          {variant === 'terminal' && (
            <div className="absolute left-3 top-1/2 -translate-y-1/2 text-green-500 font-mono flex items-center gap-1 z-10">
              <Terminal size={14} className={cn(isFocused && "icon-focus-bounce")} />
              <span className="text-green-400">$</span>
            </div>
          )}

          {/* Left icon */}
          {leftIcon && variant !== 'terminal' && (
            <div
              className={cn(
                "absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 transition-all duration-300 z-10",
                isFocused && "text-red-400 icon-slide-in icon-focus-bounce"
              )}
            >
              {leftIcon}
            </div>
          )}

          {/* Floating label */}
          {label && floatingLabel && (
            <label
              className={cn(
                "absolute left-4 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none",
                "transition-all duration-300 origin-left",
                leftIcon && "left-10",
                variant === 'terminal' && "left-16",
                (isFocused || hasValue) && [
                  "text-xs -translate-y-[1.75rem] scale-[0.85]",
                  isFocused ? "text-red-400" : "text-gray-400",
                  currentValidation === 'error' && "text-red-400",
                  currentValidation === 'success' && "text-green-400",
                  currentValidation === 'warning' && "text-amber-400",
                ]
              )}
            >
              {label}
            </label>
          )}

          {/* Input field */}
          <input
            type={inputType}
            ref={(node) => {
              // Handle both refs
              if (typeof ref === 'function') ref(node)
              else if (ref) ref.current = node
              ;(inputRef as React.MutableRefObject<HTMLInputElement | null>).current = node
            }}
            maxLength={maxLength}
            className={cn(
              inputVariants({ variant, size, validation: currentValidation }),
              leftIcon && variant !== 'terminal' && 'pl-10',
              variant === 'terminal' && 'pl-16 font-mono',
              (rightIcon || showPasswordToggle) && 'pr-10',
              showCharacterCount && maxLength && 'pr-16',
              floatingLabel && 'pt-6 pb-2',
              isFocused && glowOnFocus && 'input-glow-pulse',
              isTyping && pulseOnType && 'input-typing-pulse',
              shakeError && 'input-shake-error',
              currentValidation === 'success' && 'input-success-pulse',
              variant === 'cyber' && 'clip-path-[polygon(0_0,calc(100%-8px)_0,100%_8px,100%_100%,8px_100%,0_calc(100%-8px))]',
              className
            )}
            onFocus={(e) => {
              setIsFocused(true)
              props.onFocus?.(e)
            }}
            onBlur={(e) => {
              setIsFocused(false)
              setHasValue(e.target.value.length > 0)
              props.onBlur?.(e)
            }}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            {...props}
          />

          {/* Right icon / Password toggle */}
          {(rightIcon || showPasswordToggle) && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-2">
              {showPasswordToggle && type === 'password' ? (
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className={cn(
                    "text-gray-400 hover:text-gray-300 transition-all duration-300",
                    "hover:scale-110 active:scale-95",
                    isFocused && "text-red-400 hover:text-red-300"
                  )}
                >
                  <div className="relative">
                    {showPassword ? (
                      <EyeOff size={18} className="transition-transform duration-200" />
                    ) : (
                      <Eye size={18} className="transition-transform duration-200" />
                    )}
                  </div>
                </button>
              ) : (
                rightIcon && (
                  <div
                    className={cn(
                      "text-gray-400 transition-all duration-300",
                      isFocused && "text-red-400"
                    )}
                  >
                    {rightIcon}
                  </div>
                )
              )}
            </div>
          )}

          {/* Character counter */}
          {showCharacterCount && maxLength && (
            <div
              className={cn(
                "absolute right-3 top-1/2 -translate-y-1/2 text-xs font-mono transition-all duration-300",
                rightIcon || showPasswordToggle ? "right-10" : "right-3",
                isAtLimit && "text-red-500 counter-warning",
                isNearLimit && !isAtLimit && "text-amber-500",
                !isNearLimit && "text-gray-500"
              )}
            >
              <span className={cn(isAtLimit && "font-bold")}>{charCount}</span>
              <span className="text-gray-600">/{maxLength}</span>
            </div>
          )}

          {/* Validation icons */}
          {currentValidation === 'success' && !rightIcon && !showPasswordToggle && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 text-green-500">
              <Check size={18} className="icon-slide-in" />
            </div>
          )}
          {currentValidation === 'error' && !rightIcon && !showPasswordToggle && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 text-red-500">
              <AlertCircle size={18} className="icon-slide-in" />
            </div>
          )}
          {currentValidation === 'warning' && !rightIcon && !showPasswordToggle && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 text-amber-500">
              <AlertTriangle size={18} className="icon-slide-in" />
            </div>
          )}

          {/* Corner accents */}
          {variant !== 'cyber' && (
            <>
              <div
                className={cn(
                  "absolute top-0 left-0 w-3 h-3 border-t-2 border-l-2 rounded-tl-lg transition-all duration-300",
                  isFocused ? "border-red-500 opacity-100" : "border-gray-700 opacity-30",
                  currentValidation === 'error' && "border-red-500 opacity-100",
                  currentValidation === 'success' && "border-green-500 opacity-100",
                  currentValidation === 'warning' && "border-amber-500 opacity-100"
                )}
              />
              <div
                className={cn(
                  "absolute top-0 right-0 w-3 h-3 border-t-2 border-r-2 rounded-tr-lg transition-all duration-300",
                  isFocused ? "border-red-500 opacity-100" : "border-gray-700 opacity-30",
                  currentValidation === 'error' && "border-red-500 opacity-100",
                  currentValidation === 'success' && "border-green-500 opacity-100",
                  currentValidation === 'warning' && "border-amber-500 opacity-100"
                )}
              />
              <div
                className={cn(
                  "absolute bottom-0 left-0 w-3 h-3 border-b-2 border-l-2 rounded-bl-lg transition-all duration-300",
                  isFocused ? "border-red-500 opacity-100" : "border-gray-700 opacity-30",
                  currentValidation === 'error' && "border-red-500 opacity-100",
                  currentValidation === 'success' && "border-green-500 opacity-100",
                  currentValidation === 'warning' && "border-amber-500 opacity-100"
                )}
              />
              <div
                className={cn(
                  "absolute bottom-0 right-0 w-3 h-3 border-b-2 border-r-2 rounded-br-lg transition-all duration-300",
                  isFocused ? "border-red-500 opacity-100" : "border-gray-700 opacity-30",
                  currentValidation === 'error' && "border-red-500 opacity-100",
                  currentValidation === 'success' && "border-green-500 opacity-100",
                  currentValidation === 'warning' && "border-amber-500 opacity-100"
                )}
              />
            </>
          )}

          {/* Glow effect layer */}
          {isFocused && glowOnFocus && (
            <div
              className={cn(
                "absolute inset-0 -z-10 rounded-lg opacity-20 blur-md transition-opacity duration-300",
                currentValidation === 'error' && "bg-red-500",
                currentValidation === 'success' && "bg-green-500",
                currentValidation === 'warning' && "bg-amber-500",
                !currentValidation || currentValidation === 'none' ? "bg-red-500" : ""
              )}
            />
          )}

          {/* Autocomplete dropdown */}
          {showAutocomplete && filteredOptions.length > 0 && (
            <div
              className={cn(
                "absolute top-full left-0 right-0 mt-2 z-50",
                "bg-black/90 backdrop-blur-xl border border-red-500/30 rounded-lg",
                "shadow-[0_10px_40px_rgba(204,0,0,0.3)]",
                "overflow-hidden dropdown-enter"
              )}
            >
              <div className="max-h-60 overflow-y-auto">
                {filteredOptions.map((option, index) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => handleAutocompleteSelect(option)}
                    className={cn(
                      "w-full px-4 py-3 text-left flex items-center gap-3",
                      "transition-all duration-200",
                      "hover:bg-red-500/10",
                      highlightedIndex === index && "bg-red-500/20 dropdown-item-highlight"
                    )}
                  >
                    {option.icon && (
                      <span className="text-red-400">{option.icon}</span>
                    )}
                    <div className="flex-1">
                      <div className="text-white font-medium">{option.label}</div>
                      {option.description && (
                        <div className="text-xs text-gray-500">{option.description}</div>
                      )}
                    </div>
                    {highlightedIndex === index && (
                      <Sparkles size={14} className="text-red-400" />
                    )}
                  </button>
                ))}
              </div>
              {/* Dropdown glow border */}
              <div className="absolute inset-0 rounded-lg border border-red-500/50 pointer-events-none" />
            </div>
          )}
        </div>

        {/* Helper text / Error / Success / Warning messages */}
        {(error || success || warning || helperText) && (
          <div
            className={cn(
              "flex items-center gap-1.5 text-xs transition-all duration-300",
              error && "text-red-400",
              success && "text-green-400",
              warning && "text-amber-400",
              !error && !success && !warning && "text-gray-500"
            )}
          >
            <span
              className={cn(
                "w-1.5 h-1.5 rounded-full",
                error && "bg-red-400 animate-pulse",
                success && "bg-green-400",
                warning && "bg-amber-400 animate-pulse",
                !error && !success && !warning && "bg-gray-500"
              )}
            />
            <span>{error || success || warning || helperText}</span>
          </div>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

// Export types for external use
export type { AutocompleteOption as InputAutocompleteOption }
export { Input, inputVariants }
