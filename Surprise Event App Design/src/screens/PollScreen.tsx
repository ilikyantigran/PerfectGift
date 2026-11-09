import { useState } from 'react'

interface Question {
  id: string
  text: string
  type: 'single' | 'multi' | 'text'
  options?: string[]
  placeholder?: string
}

const QUESTIONS: Question[] = [
  {
    id: 'activity',
    text: "What type of experience do you enjoy most?",
    type: 'single',
    options: ['Outdoor Adventure', 'Cozy Night In', 'Fine Dining', 'Arts & Culture'],
  },
  {
    id: 'treats',
    text: "What are your favorite treats? Choose all that apply.",
    type: 'multi',
    options: ['Fresh Flowers', 'Chocolates', 'Scented Candles', 'Skincare & Spa', 'Wine & Drinks', 'Jewelry'],
  },
  {
    id: 'style',
    text: "How would you describe your personal style?",
    type: 'single',
    options: ['Classic & Elegant', 'Bohemian & Free', 'Modern & Minimal', 'Bold & Playful'],
  },
  {
    id: 'love_language',
    text: "What is your love language?",
    type: 'single',
    options: ['Quality Time', 'Words of Affirmation', 'Acts of Service', 'Receiving Gifts', 'Physical Touch'],
  },
  {
    id: 'notes',
    text: "Any special notes, wishes, or dreams?",
    type: 'text',
    placeholder: 'A dream destination, a favorite memory, something you always wanted to try...',
  },
]

interface Props {
  onComplete: () => void
  onBack: () => void
}

export default function PollScreen({ onComplete, onBack }: Props) {
  const [step, setStep] = useState(0)
  const [answers, setAnswers] = useState<Record<string, string | string[]>>({})
  const [textValue, setTextValue] = useState('')

  const q = QUESTIONS[step]
  const isLast = step === QUESTIONS.length - 1

  const selectSingle = (opt: string) => {
    setAnswers((a) => ({ ...a, [q.id]: opt }))
  }

  const toggleMulti = (opt: string) => {
    setAnswers((a) => {
      const prev = (a[q.id] as string[]) || []
      return {
        ...a,
        [q.id]: prev.includes(opt) ? prev.filter((x) => x !== opt) : [...prev, opt],
      }
    })
  }

  const currentAnswer = answers[q.id]

  const canAdvance =
    q.type === 'text' ? true : q.type === 'single' ? !!currentAnswer : ((currentAnswer as string[]) || []).length > 0

  const advance = () => {
    if (q.type === 'text') setAnswers((a) => ({ ...a, [q.id]: textValue }))
    if (isLast) onComplete()
    else setStep((s) => s + 1)
  }

  return (
    <div
      className="min-h-full flex flex-col"
      style={{ background: 'linear-gradient(180deg, #130d18 0%, #07040a 100%)' }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', padding: '12px 24px 16px' }}>
        <button
          onClick={step === 0 ? onBack : () => setStep((s) => s - 1)}
          style={{
            width: 36, height: 36, borderRadius: 12, border: '1px solid #2e1b2a',
            background: '#1a1020', display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer', marginRight: 12,
          }}
        >
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <path d="M11 5L7 9L11 13" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
        <div style={{ flex: 1 }}>
          <p style={{ color: '#a48a96', fontSize: 11, fontFamily: 'var(--font-body)', textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            Partner Poll
          </p>
          <h2 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 18, fontWeight: 600 }}>
            Tell us about you
          </h2>
        </div>
      </div>

      {/* Progress */}
      <div style={{ padding: '0 24px 24px' }}>
        <div style={{ display: 'flex', gap: 6 }}>
          {QUESTIONS.map((_, i) => (
            <div
              key={i}
              style={{
                flex: 1, height: 3, borderRadius: 2,
                background: i <= step ? '#d4778a' : '#2e1b2a',
                transition: 'background 0.3s',
              }}
            />
          ))}
        </div>
        <p style={{ color: '#a48a96', fontSize: 12, fontFamily: 'var(--font-body)', marginTop: 8 }}>
          Question {step + 1} of {QUESTIONS.length}
        </p>
      </div>

      {/* Question */}
      <div style={{ padding: '0 24px', flex: 1 }}>
        <h3
          style={{
            fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 22,
            fontWeight: 600, lineHeight: 1.35, marginBottom: 24,
          }}
        >
          {q.text}
        </h3>

        {q.type === 'text' && (
          <textarea
            value={textValue}
            onChange={(e) => setTextValue(e.target.value)}
            placeholder={q.placeholder}
            rows={5}
            style={{
              width: '100%', background: '#1a1020', border: '1px solid #2e1b2a',
              borderRadius: 18, padding: '14px 16px', color: '#f5ebe8', fontSize: 14,
              fontFamily: 'var(--font-body)', outline: 'none', resize: 'none',
              lineHeight: 1.6, boxSizing: 'border-box',
            }}
          />
        )}

        {(q.type === 'single' || q.type === 'multi') && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {q.options!.map((opt) => {
              const selected =
                q.type === 'single'
                  ? currentAnswer === opt
                  : ((currentAnswer as string[]) || []).includes(opt)

              return (
                <button
                  key={opt}
                  onClick={() => q.type === 'single' ? selectSingle(opt) : toggleMulti(opt)}
                  style={{
                    padding: '15px 16px', borderRadius: 16, cursor: 'pointer',
                    background: selected ? 'rgba(212,119,138,0.15)' : '#1a1020',
                    border: selected ? '1px solid rgba(212,119,138,0.5)' : '1px solid #2e1b2a',
                    display: 'flex', alignItems: 'center', gap: 12,
                    textAlign: 'left', transition: 'all 0.15s',
                  } as React.CSSProperties}
                >
                  <div
                    style={{
                      width: 20, height: 20, borderRadius: q.type === 'multi' ? 6 : '50%',
                      border: selected ? '2px solid #d4778a' : '2px solid #2e1b2a',
                      background: selected ? '#d4778a' : 'transparent',
                      flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
                      transition: 'all 0.15s',
                    }}
                  >
                    {selected && (
                      <svg width="10" height="8" viewBox="0 0 10 8" fill="none">
                        <path d="M1 4L3.8 7L9 1" stroke="#07040a" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    )}
                  </div>
                  <span style={{ color: selected ? '#f5ebe8' : '#a48a96', fontSize: 14, fontFamily: 'var(--font-body)', fontWeight: selected ? 500 : 400 }}>
                    {opt}
                  </span>
                </button>
              )
            })}
          </div>
        )}
      </div>

      {/* Next button */}
      <div style={{ padding: '24px 24px 8px' }}>
        <button
          onClick={advance}
          disabled={!canAdvance}
          style={{
            width: '100%', padding: '16px 0', borderRadius: 18, border: 'none', cursor: canAdvance ? 'pointer' : 'not-allowed',
            background: canAdvance ? 'linear-gradient(135deg, #d4778a, #b85a70)' : '#1a1020',
            color: canAdvance ? '#f5ebe8' : '#a48a96',
            fontSize: 15, fontWeight: 600, fontFamily: 'var(--font-body)',
            boxShadow: canAdvance ? '0 8px 28px rgba(212,119,138,0.35)' : 'none',
            transition: 'all 0.2s',
          }}
        >
          {isLast ? 'Submit Poll' : 'Next Question'}
        </button>
      </div>
    </div>
  )
}
