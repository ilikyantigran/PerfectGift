import { useState } from 'react'

interface Props {
  onNext: () => void
}

export default function RegistrationScreen({ onNext }: Props) {
  const [isLogin, setIsLogin] = useState(false)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const inputStyle = {
    width: '100%',
    background: '#1a1020',
    border: '1px solid #2e1b2a',
    borderRadius: 16,
    padding: '14px 16px',
    color: '#f5ebe8',
    fontSize: 14,
    fontFamily: 'var(--font-body)',
    outline: 'none',
  }

  return (
    <div
      className="min-h-full flex flex-col pb-8"
      style={{ background: 'linear-gradient(180deg, #140c18 0%, #0d0612 60%, #07040a 100%)' }}
    >
      {/* Logo */}
      <div className="flex flex-col items-center pt-10 pb-8">
        <div
          style={{
            width: 72, height: 72, borderRadius: 24,
            background: 'linear-gradient(135deg, rgba(212,119,138,0.25), rgba(232,160,122,0.15))',
            border: '1px solid rgba(212,119,138,0.3)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 12px 40px rgba(212,119,138,0.2)',
            marginBottom: 16,
          }}
        >
          <svg width="36" height="36" viewBox="0 0 36 36" fill="none">
            <path
              d="M18 30s-13-8.4-13-17a9 9 0 0 1 13-8.06A9 9 0 0 1 31 13c0 8.6-13 17-13 17Z"
              fill="url(#rh)"
            />
            <defs>
              <linearGradient id="rh" x1="5" y1="4" x2="31" y2="30" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="#e8a07a" />
                <stop offset="100%" stopColor="#d4778a" />
              </linearGradient>
            </defs>
          </svg>
        </div>
        <h1 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 32, fontWeight: 700, letterSpacing: '-0.01em' }}>
          Amore
        </h1>
        <p style={{ color: '#a48a96', fontSize: 13, marginTop: 4, fontFamily: 'var(--font-body)' }}>
          Make love unforgettable
        </p>
      </div>

      <div className="px-7 flex-1 flex flex-col">
        {/* Tab toggle */}
        <div
          style={{
            display: 'flex', background: '#1a1020', borderRadius: 18,
            padding: 4, marginBottom: 24, border: '1px solid #2e1b2a',
          }}
        >
          {(['Sign Up', 'Sign In'] as const).map((label, i) => {
            const active = (i === 0) !== isLogin
            return (
              <button
                key={label}
                onClick={() => setIsLogin(i === 1)}
                style={{
                  flex: 1, padding: '10px 0', borderRadius: 14, border: 'none', cursor: 'pointer',
                  fontFamily: 'var(--font-body)', fontSize: 13, fontWeight: 500, transition: 'all 0.2s',
                  background: active ? 'linear-gradient(135deg, #d4778a, #b85a70)' : 'transparent',
                  color: active ? '#f5ebe8' : '#a48a96',
                  boxShadow: active ? '0 4px 16px rgba(212,119,138,0.3)' : 'none',
                }}
              >
                {label}
              </button>
            )
          })}
        </div>

        {/* Fields */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
          <div>
            <label style={{ color: '#a48a96', fontSize: 11, fontFamily: 'var(--font-body)', textTransform: 'uppercase', letterSpacing: '0.1em', display: 'block', marginBottom: 6 }}>
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="your@email.com"
              style={{ ...inputStyle }}
            />
          </div>
          <div>
            <label style={{ color: '#a48a96', fontSize: 11, fontFamily: 'var(--font-body)', textTransform: 'uppercase', letterSpacing: '0.1em', display: 'block', marginBottom: 6 }}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              style={{ ...inputStyle }}
            />
          </div>
        </div>

        {isLogin && (
          <div style={{ textAlign: 'right', marginBottom: 16 }}>
            <button style={{ color: '#d4778a', fontSize: 12, fontFamily: 'var(--font-body)', background: 'none', border: 'none', cursor: 'pointer' }}>
              Forgot password?
            </button>
          </div>
        )}

        {/* CTA */}
        <button
          onClick={onNext}
          style={{
            width: '100%', padding: '16px 0', borderRadius: 18, border: 'none', cursor: 'pointer',
            background: 'linear-gradient(135deg, #d4778a 0%, #b85a70 100%)',
            color: '#f5ebe8', fontSize: 15, fontWeight: 600, fontFamily: 'var(--font-body)',
            boxShadow: '0 8px 28px rgba(212,119,138,0.4)',
            marginBottom: 24,
            transition: 'transform 0.1s',
          }}
          onMouseDown={(e) => (e.currentTarget.style.transform = 'scale(0.98)')}
          onMouseUp={(e) => (e.currentTarget.style.transform = 'scale(1)')}
        >
          {isLogin ? 'Sign In' : 'Create Account'}
        </button>

        {/* Divider */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
          <div style={{ flex: 1, height: 1, background: '#2e1b2a' }} />
          <span style={{ color: '#a48a96', fontSize: 12, fontFamily: 'var(--font-body)' }}>or continue with</span>
          <div style={{ flex: 1, height: 1, background: '#2e1b2a' }} />
        </div>

        {/* Social buttons */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <button
            onClick={onNext}
            style={{
              width: '100%', padding: '13px 0', borderRadius: 16,
              background: 'transparent', border: '1px solid #2e1b2a', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10,
              color: '#f5ebe8', fontSize: 13, fontFamily: 'var(--font-body)',
            }}
          >
            <svg width="18" height="18" viewBox="0 0 18 18">
              <path d="M17.64 9.2a10.34 10.34 0 0 0-.164-1.84H9v3.481h4.844a4.14 4.14 0 0 1-1.796 2.716v2.259h2.908c1.702-1.567 2.684-3.875 2.684-6.615z" fill="#4285F4" />
              <path d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z" fill="#34A853" />
              <path d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z" fill="#FBBC05" />
              <path d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z" fill="#EA4335" />
            </svg>
            Continue with Google
          </button>
          <button
            onClick={onNext}
            style={{
              width: '100%', padding: '13px 0', borderRadius: 16,
              background: 'transparent', border: '1px solid #2e1b2a', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10,
              color: '#f5ebe8', fontSize: 13, fontFamily: 'var(--font-body)',
            }}
          >
            <svg width="16" height="18" viewBox="0 0 814 1000" fill="#f5ebe8">
              <path d="M788.1 340.9c-5.8 4.5-108.2 62.2-108.2 190.5 0 148.4 130.3 200.9 134.2 202.2-.6 3.2-20.7 71.9-68.7 141.9-42.8 61.6-87.5 123.1-155.5 123.1s-85.5-39.5-164-39.5c-76 0-103.7 40.8-165.9 40.8s-105.2-57.8-155.5-127.4C46 790.8 0 694.5 0 604c0-224.7 144.4-343.7 284.1-343.7 75.1 0 137.7 49.3 184.9 49.3 45.1 0 116.5-52.7 199.3-52.7zM450 237.8C467.6 215.4 480 184 480 152.6c0-4.5-.4-9.1-1.2-12.7-44.8 1.6-98.4 29.9-130 66.3-15.3 17.3-30 47.3-30 77.5 0 4.9.7 9.8 1.2 11.4 2.1.4 5.6.7 8.3.7 40.6 0 91.5-26.3 121.7-58z" />
            </svg>
            Continue with Apple
          </button>
        </div>
      </div>
    </div>
  )
}
