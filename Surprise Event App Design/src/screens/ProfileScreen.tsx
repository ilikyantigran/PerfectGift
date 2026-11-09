interface Props {
  onGeneratePoll: () => void
  onSignOut: () => void
  onBack: () => void
}

const menuItems = [
  {
    id: 'poll',
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <path d="M3 5h14M3 10h9M3 15h6" stroke="#d4778a" strokeWidth="1.5" strokeLinecap="round" />
        <circle cx="15" cy="13" r="3" stroke="#d4778a" strokeWidth="1.5" />
        <path d="M17 15.5l2 2" stroke="#d4778a" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    ),
    iconBg: 'rgba(212,119,138,0.15)',
    label: 'Generate Poll',
    sub: 'Share with your partner to learn her wishes',
    color: '#f5ebe8',
  },
  {
    id: 'signout',
    icon: (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <path d="M7 17H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h3" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        <path d="M13 14l4-4-4-4M17 10H7" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    ),
    iconBg: 'rgba(255,255,255,0.05)',
    label: 'Sign Out',
    sub: null,
    color: '#a48a96',
  },
]

export default function ProfileScreen({ onGeneratePoll, onSignOut, onBack }: Props) {
  return (
    <div
      className="min-h-full flex flex-col"
      style={{ background: 'linear-gradient(180deg, #130d18 0%, #0d0612 60%, #07040a 100%)' }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', padding: '12px 24px 20px' }}>
        <button
          onClick={onBack}
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
        <h2 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 20, fontWeight: 600 }}>Profile</h2>
      </div>

      {/* Avatar card */}
      <div style={{ padding: '0 24px 32px', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <div
          style={{
            width: 84, height: 84, borderRadius: '50%', marginBottom: 14,
            background: 'linear-gradient(135deg, rgba(212,119,138,0.35), rgba(232,160,122,0.2))',
            border: '2px solid rgba(212,119,138,0.4)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 8px 32px rgba(212,119,138,0.25)',
          }}
        >
          <span style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 28, fontWeight: 600 }}>A</span>
        </div>
        <h3 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 22, fontWeight: 600, marginBottom: 4 }}>
          Alex
        </h3>
        <p style={{ color: '#a48a96', fontSize: 13, fontFamily: 'var(--font-body)' }}>alex@email.com</p>

        {/* Stats row */}
        <div
          style={{
            display: 'flex', gap: 1, marginTop: 24, width: '100%',
            background: '#1a1020', border: '1px solid #2e1b2a', borderRadius: 18, overflow: 'hidden',
          }}
        >
          {[['12', 'Surprises'], ['3', 'Polls sent'], ['∞', 'Love given']].map(([val, lbl], i) => (
            <div
              key={lbl}
              style={{
                flex: 1, padding: '14px 8px', textAlign: 'center',
                borderRight: i < 2 ? '1px solid #2e1b2a' : 'none',
              }}
            >
              <div style={{ fontFamily: 'var(--font-display)', color: '#d4778a', fontSize: 20, fontWeight: 700 }}>{val}</div>
              <div style={{ color: '#a48a96', fontSize: 10, fontFamily: 'var(--font-body)', marginTop: 3, textTransform: 'uppercase', letterSpacing: '0.08em' }}>{lbl}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Menu items */}
      <div style={{ padding: '0 24px', display: 'flex', flexDirection: 'column', gap: 10 }}>
        {menuItems.map((item) => (
          <button
            key={item.id}
            onClick={item.id === 'poll' ? onGeneratePoll : onSignOut}
            style={{
              width: '100%', padding: '16px', borderRadius: 18, border: '1px solid #2e1b2a',
              background: '#1a1020', display: 'flex', alignItems: 'center', gap: 14,
              cursor: 'pointer', textAlign: 'left', transition: 'background 0.15s',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = '#251729')}
            onMouseLeave={(e) => (e.currentTarget.style.background = '#1a1020')}
          >
            <div
              style={{
                width: 44, height: 44, borderRadius: 14, flexShrink: 0,
                background: item.iconBg,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
            >
              {item.icon}
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ color: item.color, fontSize: 14, fontWeight: 500, fontFamily: 'var(--font-body)' }}>
                {item.label}
              </div>
              {item.sub && (
                <div style={{ color: '#a48a96', fontSize: 12, fontFamily: 'var(--font-body)', marginTop: 2 }}>
                  {item.sub}
                </div>
              )}
            </div>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M6 4L10 8L6 12" stroke="#2e1b2a" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        ))}
      </div>
    </div>
  )
}
