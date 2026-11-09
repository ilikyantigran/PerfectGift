import { useState } from 'react'
import type { AppData } from '../App'

const OCCASIONS = ["Valentine's Day", 'Birthday', 'Anniversary', 'Just Because', 'Christmas', "New Year's", 'Other']
const BUDGETS = ['Under $50', '$50–$150', '$150–$500', '$500+']

interface Props {
  onGenerate: (data: AppData) => void
  onProfile: () => void
}

export default function MainScreen({ onGenerate, onProfile }: Props) {
  const [occasion, setOccasion] = useState('')
  const [budget, setBudget] = useState('')
  const [preferences, setPreferences] = useState('')
  const [showPicker, setShowPicker] = useState(false)

  const s = {
    label: { color: '#a48a96', fontSize: 10, fontFamily: 'var(--font-body)', textTransform: 'uppercase' as const, letterSpacing: '0.12em', display: 'block', marginBottom: 8 },
    card: { background: '#1a1020', border: '1px solid #2e1b2a', borderRadius: 18, padding: '14px 16px' },
  }

  return (
    <div
      className="min-h-full flex flex-col pb-10"
      style={{ background: 'linear-gradient(180deg, #130d18 0%, #0d0612 50%, #07040a 100%)' }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 24px 20px' }}>
        <div>
          <p style={{ color: '#a48a96', fontSize: 12, fontFamily: 'var(--font-body)' }}>Good evening,</p>
          <h2 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 20, fontWeight: 600, marginTop: 1 }}>
            Alex
          </h2>
        </div>
        <button
          onClick={onProfile}
          style={{
            width: 40, height: 40, borderRadius: '50%', cursor: 'pointer',
            background: 'linear-gradient(135deg, rgba(212,119,138,0.3), rgba(232,160,122,0.2))',
            border: '1px solid rgba(212,119,138,0.35)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 4px 16px rgba(212,119,138,0.2)',
          } as React.CSSProperties}
        >
          <span style={{ color: '#f5ebe8', fontSize: 15, fontWeight: 600, fontFamily: 'var(--font-body)' }}>A</span>
        </button>
      </div>

      {/* Hero */}
      <div style={{ padding: '0 24px 28px' }}>
        <h1 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 30, lineHeight: 1.25, fontWeight: 700 }}>
          Plan the perfect<br />
          <span style={{ color: '#d4778a', fontStyle: 'italic' }}>surprise</span> for her
        </h1>
        <p style={{ color: '#a48a96', fontSize: 13, fontFamily: 'var(--font-body)', marginTop: 8, lineHeight: 1.6 }}>
          Tell us about the occasion and we'll craft something she'll never forget.
        </p>
      </div>

      {/* Form */}
      <div style={{ padding: '0 24px', display: 'flex', flexDirection: 'column', gap: 16 }}>
        {/* Occasion */}
        <div>
          <label style={s.label}>Occasion</label>
          <div style={{ position: 'relative' }}>
            <button
              onClick={() => setShowPicker(!showPicker)}
              style={{
                ...s.card, width: '100%', display: 'flex', alignItems: 'center',
                justifyContent: 'space-between', cursor: 'pointer',
                border: showPicker ? '1px solid rgba(212,119,138,0.5)' : '1px solid #2e1b2a',
                transition: 'border-color 0.2s',
              } as React.CSSProperties}
            >
              <span style={{ color: occasion ? '#f5ebe8' : 'rgba(164,138,150,0.5)', fontSize: 14, fontFamily: 'var(--font-body)' }}>
                {occasion || 'Select occasion'}
              </span>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" style={{ transform: showPicker ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }}>
                <path d="M4 6L8 10L12 6" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>

            {showPicker && (
              <div
                style={{
                  position: 'absolute', top: 'calc(100% + 4px)', left: 0, right: 0, zIndex: 30,
                  background: '#251729', border: '1px solid #2e1b2a', borderRadius: 16,
                  overflow: 'hidden', boxShadow: '0 16px 40px rgba(0,0,0,0.5)',
                }}
              >
                {OCCASIONS.map((o) => (
                  <button
                    key={o}
                    onClick={() => { setOccasion(o); setShowPicker(false) }}
                    style={{
                      width: '100%', padding: '12px 16px', textAlign: 'left', cursor: 'pointer',
                      background: o === occasion ? 'rgba(212,119,138,0.1)' : 'transparent',
                      border: 'none', color: o === occasion ? '#d4778a' : '#f5ebe8',
                      fontSize: 14, fontFamily: 'var(--font-body)', borderBottom: '1px solid rgba(46,27,42,0.5)',
                    }}
                  >
                    {o}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Budget */}
        <div>
          <label style={s.label}>Budget</label>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
            {BUDGETS.map((b) => (
              <button
                key={b}
                onClick={() => setBudget(b)}
                style={{
                  padding: '12px 8px', borderRadius: 14, cursor: 'pointer',
                  background: b === budget ? 'rgba(212,119,138,0.15)' : '#1a1020',
                  border: b === budget ? '1px solid rgba(212,119,138,0.5)' : '1px solid #2e1b2a',
                  color: b === budget ? '#d4778a' : '#a48a96',
                  fontSize: 13, fontFamily: 'var(--font-body)', fontWeight: b === budget ? 500 : 400,
                  transition: 'all 0.15s',
                } as React.CSSProperties}
              >
                {b}
              </button>
            ))}
          </div>
        </div>

        {/* Preferences */}
        <div>
          <label style={s.label}>
            Preferences <span style={{ textTransform: 'none', opacity: 0.6 }}>(optional)</span>
          </label>
          <textarea
            value={preferences}
            onChange={(e) => setPreferences(e.target.value)}
            placeholder="She loves Italian food, hates heights, her favorite scent is vanilla..."
            rows={4}
            style={{
              width: '100%', background: '#1a1020', border: '1px solid #2e1b2a',
              borderRadius: 18, padding: '14px 16px', color: '#f5ebe8', fontSize: 14,
              fontFamily: 'var(--font-body)', outline: 'none', resize: 'none',
              lineHeight: 1.6, boxSizing: 'border-box',
            }}
          />
        </div>

        {/* Generate button */}
        <button
          onClick={() => onGenerate({ holiday: occasion, budget, preferences })}
          style={{
            width: '100%', padding: '17px 0', borderRadius: 18, border: 'none', cursor: 'pointer',
            background: 'linear-gradient(135deg, #d4778a 0%, #b85a70 100%)',
            color: '#f5ebe8', fontSize: 16, fontWeight: 600, fontFamily: 'var(--font-body)',
            boxShadow: '0 10px 36px rgba(212,119,138,0.45)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            marginTop: 4, transition: 'transform 0.1s, box-shadow 0.1s',
          }}
          onMouseDown={(e) => { e.currentTarget.style.transform = 'scale(0.98)'; e.currentTarget.style.boxShadow = '0 6px 20px rgba(212,119,138,0.3)' }}
          onMouseUp={(e) => { e.currentTarget.style.transform = 'scale(1)'; e.currentTarget.style.boxShadow = '0 10px 36px rgba(212,119,138,0.45)' }}
        >
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <path d="M9 1L11.39 6.26L17 6.93L12.9 11L14.12 16.66L9 13.77L3.88 16.66L5.1 11L1 6.93L6.61 6.26L9 1Z" fill="currentColor" />
          </svg>
          Generate Magic
        </button>
      </div>
    </div>
  )
}
