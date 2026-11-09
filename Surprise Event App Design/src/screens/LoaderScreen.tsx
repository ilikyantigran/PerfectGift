import { useEffect, useState } from 'react'
import type { AppData } from '../App'

interface Props {
  onComplete: () => void
  data: AppData
}

const STEPS = [
  'Analyzing your preferences...',
  'Exploring romantic possibilities...',
  'Curating the perfect ideas...',
  'Adding the finishing touches...',
]

export default function LoaderScreen({ onComplete, data }: Props) {
  const [stepIdx, setStepIdx] = useState(0)
  const [particles, setParticles] = useState<{ id: number; x: number; delay: number; size: number }[]>([])

  useEffect(() => {
    setParticles(
      Array.from({ length: 8 }, (_, i) => ({
        id: i,
        x: 30 + Math.random() * 100,
        delay: Math.random() * 2,
        size: 4 + Math.random() * 8,
      }))
    )
  }, [])

  useEffect(() => {
    const interval = setInterval(() => {
      setStepIdx((s) => {
        if (s >= STEPS.length - 1) {
          clearInterval(interval)
          return s
        }
        return s + 1
      })
    }, 700)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    const timeout = setTimeout(onComplete, 3200)
    return () => clearTimeout(timeout)
  }, [onComplete])

  return (
    <div
      className="min-h-full flex flex-col items-center justify-center"
      style={{
        background: 'linear-gradient(180deg, #130d18 0%, #07040a 100%)',
        padding: '0 32px',
        textAlign: 'center',
      }}
    >
      {/* Glow */}
      <div
        style={{
          position: 'absolute',
          width: 240, height: 240,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(212,119,138,0.2) 0%, transparent 70%)',
          pointerEvents: 'none',
        }}
      />

      {/* Animated heart */}
      <div style={{ position: 'relative', width: 120, height: 120, marginBottom: 40 }}>
        {/* Ring */}
        <svg
          width="120" height="120" viewBox="0 0 120 120"
          style={{ position: 'absolute', inset: 0, animation: 'spin-slow 4s linear infinite' }}
        >
          <circle cx="60" cy="60" r="54" fill="none" stroke="rgba(212,119,138,0.15)" strokeWidth="2" />
          <circle
            cx="60" cy="60" r="54"
            fill="none" stroke="url(#ring-grad)" strokeWidth="2"
            strokeLinecap="round"
            strokeDasharray="100 240"
          />
          <defs>
            <linearGradient id="ring-grad" x1="0" y1="0" x2="120" y2="120" gradientUnits="userSpaceOnUse">
              <stop offset="0%" stopColor="#e8a07a" />
              <stop offset="100%" stopColor="#d4778a" />
            </linearGradient>
          </defs>
        </svg>

        {/* Heart */}
        <div
          style={{
            position: 'absolute', inset: 0,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            animation: 'pulse-heart 1.4s ease-in-out infinite',
          }}
        >
          <svg width="52" height="52" viewBox="0 0 52 52" fill="none">
            <path
              d="M26 46s-21-13.7-21-27.5a13 13 0 0 1 21-10.2A13 13 0 0 1 47 18.5c0 13.8-21 27.5-21 27.5Z"
              fill="url(#heart-loader)"
            />
            <defs>
              <linearGradient id="heart-loader" x1="5" y1="6" x2="47" y2="46" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="#e8a07a" />
                <stop offset="100%" stopColor="#d4778a" />
              </linearGradient>
            </defs>
          </svg>
        </div>

        {/* Floating particles */}
        {particles.map((p) => (
          <div
            key={p.id}
            style={{
              position: 'absolute',
              left: p.x - 30,
              bottom: 0,
              width: p.size,
              height: p.size,
              borderRadius: '50%',
              background: p.id % 2 === 0 ? '#d4778a' : '#e8a07a',
              animation: `float-particle ${1.5 + p.delay}s ease-out ${p.delay}s infinite`,
              opacity: 0.7,
            }}
          />
        ))}
      </div>

      {/* Title */}
      <h2 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 26, fontWeight: 700, marginBottom: 8, lineHeight: 1.25 }}>
        Crafting your<br />
        <span style={{ color: '#d4778a', fontStyle: 'italic' }}>perfect surprise</span>
      </h2>

      {data.holiday && (
        <p style={{ color: '#a48a96', fontSize: 13, fontFamily: 'var(--font-body)', marginBottom: 32 }}>
          For {data.holiday}{data.budget ? ` · ${data.budget}` : ''}
        </p>
      )}

      {!data.holiday && (
        <p style={{ color: '#a48a96', fontSize: 13, fontFamily: 'var(--font-body)', marginBottom: 32 }}>
          Making it special just for her
        </p>
      )}

      {/* Step indicator */}
      <div
        style={{
          background: '#1a1020', border: '1px solid #2e1b2a',
          borderRadius: 14, padding: '12px 20px', minWidth: 260,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div
            style={{
              width: 8, height: 8, borderRadius: '50%', background: '#d4778a',
              boxShadow: '0 0 8px rgba(212,119,138,0.8)',
              animation: 'pulse-heart 0.8s ease-in-out infinite',
            }}
          />
          <span style={{ color: '#f5ebe8', fontSize: 13, fontFamily: 'var(--font-body)' }}>
            {STEPS[stepIdx]}
          </span>
        </div>
      </div>

      {/* Dots progress */}
      <div style={{ display: 'flex', gap: 6, marginTop: 24 }}>
        {STEPS.map((_, i) => (
          <div
            key={i}
            style={{
              width: i === stepIdx ? 20 : 6, height: 6, borderRadius: 3,
              background: i <= stepIdx ? '#d4778a' : '#2e1b2a',
              transition: 'all 0.4s ease',
            }}
          />
        ))}
      </div>
    </div>
  )
}
