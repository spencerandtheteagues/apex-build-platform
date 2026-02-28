// APEX.BUILD — Buy Credits Modal
// One-time credit top-up via Stripe Checkout

import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, CreditCard, Zap, Check, AlertCircle, Loader2 } from 'lucide-react'
import apiService from '@/services/api'

interface CreditPack {
  amountUsd: number
  creditUsd: number
  label: string
  popular?: boolean
}

const PACKS: CreditPack[] = [
  { amountUsd: 10,  creditUsd: 10,  label: '$10'  },
  { amountUsd: 25,  creditUsd: 25,  label: '$25'  },
  { amountUsd: 50,  creditUsd: 50,  label: '$50',  popular: true },
  { amountUsd: 100, creditUsd: 100, label: '$100' },
]

interface Props {
  onClose: () => void
  /** Optional: highlighted message shown above the packs (e.g. "Your credits have run out.") */
  reason?: string
}

export const BuyCreditsModal: React.FC<Props> = ({ onClose, reason }) => {
  const [selected, setSelected] = useState<number>(50)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handlePurchase = async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await apiService.purchaseCredits({
        amount_usd: selected,
        success_url: window.location.href + '?credits=success',
        cancel_url: window.location.href + '?credits=cancel',
      })
      if (result.success && result.data?.checkout_url) {
        window.location.href = result.data.checkout_url
      } else {
        setError('Failed to create checkout session. Please try again.')
      }
    } catch (err: any) {
      const msg = err?.response?.data?.error || err?.message || 'Failed to start checkout'
      setError(msg)
    } finally {
      setLoading(false)
    }
  }

  const pack = PACKS.find(p => p.amountUsd === selected)!

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 1000,
        background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(8px)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: '1rem',
      }}
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.94, y: 12 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.94, y: 12 }}
        transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
        style={{
          background: '#0a0a0a',
          border: '1px solid rgba(255,0,51,0.22)',
          borderRadius: 16,
          width: '100%', maxWidth: 480,
          boxShadow: '0 0 80px rgba(255,0,51,0.12), 0 24px 60px rgba(0,0,0,0.7)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          padding: '20px 24px 16px',
          borderBottom: '1px solid rgba(255,255,255,0.06)',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{
              width: 36, height: 36,
              background: 'rgba(255,0,51,0.12)', border: '1px solid rgba(255,0,51,0.22)',
              borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <Zap size={17} style={{ color: '#ff0033' }} />
            </div>
            <div>
              <div style={{ fontFamily: '"Orbitron", sans-serif', fontWeight: 800, fontSize: '0.95rem', color: '#f0f0f0', letterSpacing: '0.04em' }}>
                BUY CREDITS
              </div>
              <div style={{ fontSize: '0.72rem', color: 'rgba(255,255,255,0.35)', marginTop: 2 }}>
                Powered by Stripe · secure checkout
              </div>
            </div>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'rgba(255,255,255,0.4)', cursor: 'pointer', padding: 4, borderRadius: 6, display: 'flex' }}
          >
            <X size={18} />
          </button>
        </div>

        <div style={{ padding: '20px 24px 24px' }}>
          {/* Reason banner */}
          {reason && (
            <div style={{
              marginBottom: 18,
              padding: '10px 14px',
              background: 'rgba(255,0,51,0.08)', border: '1px solid rgba(255,0,51,0.2)',
              borderRadius: 8, display: 'flex', gap: 10, alignItems: 'flex-start',
            }}>
              <AlertCircle size={15} style={{ color: '#ff0033', flexShrink: 0, marginTop: 1 }} />
              <p style={{ margin: 0, fontSize: '0.82rem', color: 'rgba(255,255,255,0.7)', lineHeight: 1.55 }}>
                {reason}
              </p>
            </div>
          )}

          {/* Pack selector */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, marginBottom: 20 }}>
            {PACKS.map(p => {
              const isSelected = p.amountUsd === selected
              return (
                <button
                  key={p.amountUsd}
                  onClick={() => setSelected(p.amountUsd)}
                  style={{
                    position: 'relative',
                    background: isSelected ? 'rgba(255,0,51,0.1)' : 'rgba(255,255,255,0.03)',
                    border: `1px solid ${isSelected ? 'rgba(255,0,51,0.45)' : 'rgba(255,255,255,0.08)'}`,
                    borderRadius: 10, padding: '14px 16px',
                    cursor: 'pointer', textAlign: 'left',
                    transition: 'all 0.15s',
                  }}
                >
                  {p.popular && (
                    <div style={{
                      position: 'absolute', top: -9, right: 10,
                      background: 'linear-gradient(135deg,#34d399,#059669)',
                      color: '#000', fontSize: '0.6rem', fontWeight: 800,
                      padding: '2px 8px', borderRadius: 100,
                      letterSpacing: '0.06em', textTransform: 'uppercase',
                    }}>
                      Popular
                    </div>
                  )}
                  <div style={{
                    fontFamily: '"Orbitron", sans-serif', fontWeight: 900,
                    fontSize: '1.3rem', color: isSelected ? '#ff0033' : '#f0f0f0',
                    marginBottom: 4,
                  }}>
                    {p.label}
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'rgba(255,255,255,0.5)' }}>
                    ${p.creditUsd.toFixed(0)} in AI credits
                  </div>
                  {isSelected && (
                    <div style={{
                      position: 'absolute', top: 10, right: 10,
                      width: 16, height: 16,
                      background: '#ff0033', borderRadius: '50%',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                      <Check size={9} style={{ color: '#fff' }} />
                    </div>
                  )}
                </button>
              )
            })}
          </div>

          {/* Summary */}
          <div style={{
            padding: '12px 16px',
            background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)',
            borderRadius: 8, marginBottom: 16,
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          }}>
            <span style={{ fontSize: '0.83rem', color: 'rgba(255,255,255,0.5)' }}>
              Total
            </span>
            <span style={{ fontFamily: '"Orbitron", sans-serif', fontWeight: 800, fontSize: '1.3rem', color: '#f0f0f0' }}>
              ${selected}.00
            </span>
            <span style={{ fontSize: '0.83rem', color: 'rgba(255,255,255,0.5)' }}>
              Added to balance
            </span>
            <span style={{ fontFamily: '"Orbitron", sans-serif', fontWeight: 800, fontSize: '1.3rem', color: '#34d399' }}>
              ${pack.creditUsd.toFixed(0)}
            </span>
          </div>

          {/* Error */}
          {error && (
            <div style={{
              marginBottom: 14, padding: '8px 12px',
              background: 'rgba(255,0,51,0.07)', border: '1px solid rgba(255,0,51,0.2)',
              borderRadius: 7, fontSize: '0.8rem', color: '#f87171',
            }}>
              {error}
            </div>
          )}

          {/* CTA */}
          <button
            onClick={handlePurchase}
            disabled={loading}
            style={{
              width: '100%',
              background: loading ? 'rgba(255,0,51,0.4)' : 'linear-gradient(135deg,#ff0033,#cc0029)',
              border: 'none', color: '#fff',
              padding: '13px 20px', borderRadius: 9,
              cursor: loading ? 'not-allowed' : 'pointer',
              fontSize: '0.95rem', fontFamily: 'Inter, system-ui, sans-serif', fontWeight: 700,
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
              boxShadow: loading ? 'none' : '0 0 30px rgba(255,0,51,0.35)',
              transition: 'all 0.2s',
            }}
          >
            {loading ? (
              <><Loader2 size={16} style={{ animation: 'spin 1s linear infinite' }} /> Redirecting to Stripe…</>
            ) : (
              <><CreditCard size={16} /> Pay ${selected}.00 · Get ${pack.creditUsd.toFixed(2)} Credits</>
            )}
          </button>

          <p style={{
            marginTop: 12, textAlign: 'center',
            fontSize: '0.7rem', color: 'rgba(255,255,255,0.25)', lineHeight: 1.55,
          }}>
            Credits are added instantly after payment. Non-refundable. No subscription created.
          </p>
        </div>
      </motion.div>

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  )
}

export default BuyCreditsModal
