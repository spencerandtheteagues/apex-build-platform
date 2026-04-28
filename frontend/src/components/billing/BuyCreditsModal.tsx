// APEX-BUILD — Buy Credits Modal
// Unified premium purchase modal matching the current workspace UI

import React, { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import { X, CreditCard, Zap, Check, AlertCircle, Loader2 } from 'lucide-react'
import apiService from '@/services/api'

interface CreditPack {
  amountUsd: number
  creditUsd: number
  label: string
  popular?: boolean
}

const DEFAULT_PACKS: CreditPack[] = [
  { amountUsd: 10, creditUsd: 10, label: '$10' },
  { amountUsd: 25, creditUsd: 25, label: '$25' },
  { amountUsd: 50, creditUsd: 50, label: '$50', popular: true },
  { amountUsd: 100, creditUsd: 100, label: '$100' },
]

interface Props {
  onClose: () => void
  reason?: string
  defaultAmount?: number
}

export const BuyCreditsModal: React.FC<Props> = ({ onClose, reason, defaultAmount }) => {
  const [packs, setPacks] = useState<CreditPack[]>(DEFAULT_PACKS)
  const [selected, setSelected] = useState<number>(defaultAmount ?? 50)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    const loadPacks = async () => {
      try {
        const result = await apiService.getCreditBalance()
        const rawPacks = result.data?.available_packs
        if (!Array.isArray(rawPacks) || rawPacks.length === 0 || cancelled) return

        const normalized = rawPacks
          .map((pack) => ({
            amountUsd: pack.amount_usd,
            creditUsd: pack.credit_usd,
            label: `$${pack.amount_usd}`,
          }))
          .filter((pack) => Number.isFinite(pack.amountUsd) && Number.isFinite(pack.creditUsd))
          .sort((a, b) => a.amountUsd - b.amountUsd)
          .map((pack, index, list) => ({
            ...pack,
            popular: index === Math.min(1, list.length - 1),
          }))

        if (normalized.length === 0) return

        setPacks(normalized)
        setSelected((current) => (
          normalized.some((pack) => pack.amountUsd === current)
            ? current
            : normalized[Math.min(1, normalized.length - 1)].amountUsd
        ))
      } catch {
        // Fall back to baked-in packs if billing metadata is unavailable.
      }
    }

    void loadPacks()
    return () => {
      cancelled = true
    }
  }, [])

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

  const activePack = packs.find((p) => p.amountUsd === selected) ?? packs[0]

  return (
    <div
      className="fixed inset-0 z-[1000] flex items-center justify-center bg-black/80 p-4 backdrop-blur-md"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <motion.div
        initial={{ opacity: 0, scale: 0.96, y: 12 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 12 }}
        transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
        className="w-full max-w-2xl overflow-hidden rounded-[28px] border border-sky-500/20 bg-[#05070c] shadow-[0_30px_90px_rgba(0,0,0,0.65)]"
      >
        <div className="border-b border-gray-800 bg-[radial-gradient(circle_at_top_left,rgba(56,189,248,0.18),transparent_36%)] px-6 py-5 sm:px-7">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-sky-500/30 bg-sky-500/10 text-sky-300">
                <Zap size={18} />
              </div>
              <div>
                <div className="text-[11px] font-black uppercase tracking-[0.26em] text-sky-300/80">
                  Managed Usage
                </div>
                <h2 className="mt-2 text-2xl font-black text-white">Buy Credits</h2>
                <p className="mt-2 max-w-xl text-sm leading-6 text-gray-400">
                  Add one-time AI usage runway to your account through Stripe without changing your plan tier.
                </p>
              </div>
            </div>

            <button
              onClick={onClose}
              aria-label="Close"
              className="flex h-10 w-10 items-center justify-center rounded-xl border border-gray-800 bg-gray-950/70 text-gray-400 transition hover:border-gray-700 hover:text-white"
            >
              <X size={16} />
            </button>
          </div>
        </div>

        <div className="grid gap-6 p-6 sm:p-7 lg:grid-cols-[1.2fr_0.8fr]">
          <div className="space-y-5">
            <div className="flex flex-wrap gap-2">
              {['One-time purchase', 'Secure Stripe checkout', 'Adds usage runway only'].map((item) => (
                <span
                  key={item}
                  className="rounded-full border border-gray-800 bg-gray-950/70 px-3 py-1.5 text-[11px] font-bold uppercase tracking-[0.18em] text-gray-500"
                >
                  {item}
                </span>
              ))}
            </div>

            {reason && (
              <div className="flex items-start gap-3 rounded-2xl border border-sky-500/25 bg-sky-500/10 px-4 py-3 text-sm text-sky-100">
                <AlertCircle size={16} className="mt-0.5 shrink-0 text-sky-300" />
                <p className="leading-6">{reason}</p>
              </div>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
              {packs.map((pack) => {
                const isSelected = pack.amountUsd === selected
                return (
                  <button
                    key={pack.amountUsd}
                    type="button"
                    onClick={() => setSelected(pack.amountUsd)}
                    className={`relative rounded-3xl border p-4 text-left transition ${
                      isSelected
                        ? 'border-sky-500/45 bg-sky-500/10 shadow-[0_0_24px_rgba(56,189,248,0.18)]'
                        : 'border-gray-800 bg-gray-950/60 hover:border-gray-700 hover:bg-gray-950/90'
                    }`}
                  >
                    {pack.popular && (
                      <div className="absolute -top-2.5 right-4 rounded-full bg-emerald-400 px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.18em] text-black">
                        Popular
                      </div>
                    )}

                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="text-[11px] font-black uppercase tracking-[0.2em] text-gray-500">
                          Credit Pack
                        </div>
                        <div className="mt-3 font-mono text-3xl font-black text-white">{pack.label}</div>
                        <div className="mt-2 text-sm text-gray-400">
                          Adds ${pack.creditUsd.toFixed(0)} in managed AI usage
                        </div>
                      </div>
                      <div className={`flex h-7 w-7 items-center justify-center rounded-full border ${
                        isSelected
                          ? 'border-sky-300 bg-sky-500 text-slate-950'
                          : 'border-gray-700 bg-gray-900 text-gray-500'
                      }`}>
                        {isSelected ? <Check size={12} /> : null}
                      </div>
                    </div>
                  </button>
                )
              })}
            </div>

            {error && (
              <div className="rounded-2xl border border-sky-500/25 bg-sky-500/10 px-4 py-3 text-sm text-sky-200">
                {error}
              </div>
            )}
          </div>

          <div className="space-y-4 rounded-3xl border border-gray-800 bg-gray-950/70 p-5">
            <div>
              <div className="text-[11px] font-black uppercase tracking-[0.2em] text-gray-500">Order Summary</div>
              <div className="mt-3 text-3xl font-black text-white">${selected}.00</div>
              <div className="mt-1 text-sm text-emerald-300">
                ${activePack.creditUsd.toFixed(2)} credits added after payment
              </div>
            </div>

            <div className="grid gap-3">
              {[
                { label: 'Unlocks plan features', value: 'No' },
                { label: 'Renews automatically', value: 'No' },
                { label: 'Best use', value: 'Usage overages' },
              ].map((item) => (
                <div key={item.label} className="rounded-2xl border border-gray-800 bg-black/40 p-4">
                  <div className="text-[11px] font-black uppercase tracking-[0.18em] text-gray-500">{item.label}</div>
                  <div className="mt-2 text-sm font-semibold text-gray-200">{item.value}</div>
                </div>
              ))}
            </div>

            <p className="text-sm leading-6 text-gray-400">
              Credit packs extend managed AI usage on your account. They do not unlock backend, publish, or BYOK access without an active paid plan.
            </p>

            <button
              onClick={handlePurchase}
              disabled={loading}
              className="inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl border border-sky-500/40 bg-gradient-to-r from-sky-600 to-blue-800 px-4 py-3 text-sm font-bold text-white shadow-[0_0_28px_rgba(56,189,248,0.22)] transition hover:from-sky-500 hover:to-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {loading ? (
                <>
                  <Loader2 size={16} className="animate-spin" />
                  Redirecting to Stripe…
                </>
              ) : (
                <>
                  <CreditCard size={16} />
                  Pay ${selected}.00
                </>
              )}
            </button>

            <p className="text-xs leading-5 text-gray-500">
              Credits are added after payment confirmation. Credit packs are non-refundable and cover usage only.
            </p>
          </div>
        </div>
      </motion.div>
    </div>
  )
}

export default BuyCreditsModal
