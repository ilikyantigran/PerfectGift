import type { ReactNode } from 'react'

export default function PhoneFrame({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex-shrink-0" style={{ width: 390, height: 844 }}>
      {/* Outer shell gradient border */}
      <div
        className="absolute inset-0 rounded-[52px]"
        style={{
          background: 'linear-gradient(145deg, rgba(255,255,255,0.12) 0%, rgba(255,255,255,0.03) 40%, rgba(255,255,255,0.06) 100%)',
          boxShadow: '0 60px 120px rgba(0,0,0,0.85), 0 0 0 1px rgba(255,255,255,0.07), inset 0 0 0 1px rgba(255,255,255,0.04)',
        }}
      />

      {/* Volume buttons left */}
      {[{ top: 128, h: 36 }, { top: 180, h: 60 }, { top: 252, h: 60 }].map((b, i) => (
        <div
          key={i}
          className="absolute rounded-l-sm"
          style={{
            left: -3, top: b.top, width: 4, height: b.h,
            background: 'linear-gradient(180deg, #2a1a2e, #1a1020)',
            boxShadow: '-1px 0 2px rgba(0,0,0,0.5)',
          }}
        />
      ))}
      {/* Power button right */}
      <div
        className="absolute rounded-r-sm"
        style={{
          right: -3, top: 160, width: 4, height: 80,
          background: 'linear-gradient(180deg, #2a1a2e, #1a1020)',
          boxShadow: '1px 0 2px rgba(0,0,0,0.5)',
        }}
      />

      {/* Screen bezel */}
      <div
        className="absolute rounded-[50px] overflow-hidden"
        style={{
          inset: 3,
          background: '#07040a',
          boxShadow: 'inset 0 0 60px rgba(0,0,0,0.4)',
        }}
      >
        {/* Top reflection */}
        <div
          className="absolute top-0 left-0 right-0 h-32 pointer-events-none z-10"
          style={{
            background: 'linear-gradient(180deg, rgba(255,255,255,0.04) 0%, transparent 100%)',
            borderRadius: '47px 47px 0 0',
          }}
        />

        {/* Dynamic island */}
        <div
          className="absolute z-50"
          style={{
            top: 13, left: '50%', transform: 'translateX(-50%)',
            width: 120, height: 34,
            background: '#000',
            borderRadius: 20,
          }}
        />

        {/* Status bar */}
        <div
          className="absolute top-0 left-0 right-0 h-14 flex items-end justify-between z-40"
          style={{ padding: '0 28px 8px' }}
        >
          <span style={{ color: '#f5ebe8', fontSize: 14, fontWeight: 600, fontFamily: 'var(--font-body)' }}>9:41</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            {/* Signal bars */}
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, height: 12 }}>
              {[4, 7, 10, 12].map((h, i) => (
                <div
                  key={i}
                  style={{
                    width: 3, height: h, borderRadius: 1.5,
                    background: i < 3 ? '#f5ebe8' : 'rgba(245,235,232,0.3)',
                  }}
                />
              ))}
            </div>
            {/* WiFi */}
            <svg width="16" height="12" viewBox="0 0 16 12" fill="none">
              <path d="M8 9a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3z" fill="#f5ebe8" />
              <path d="M4.5 6.8A5 5 0 0 1 8 5.5a5 5 0 0 1 3.5 1.3" stroke="#f5ebe8" strokeWidth="1.4" strokeLinecap="round" fill="none" />
              <path d="M2 4A8 8 0 0 1 8 1.5 8 8 0 0 1 14 4" stroke="#f5ebe8" strokeWidth="1.4" strokeLinecap="round" fill="none" />
            </svg>
            {/* Battery */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <div style={{ width: 24, height: 12, borderRadius: 3, border: '1px solid rgba(245,235,232,0.5)', position: 'relative', overflow: 'hidden' }}>
                <div style={{ position: 'absolute', inset: '1.5px', right: '25%', background: '#f5ebe8', borderRadius: 1.5 }} />
              </div>
              <div style={{ width: 2, height: 6, background: 'rgba(245,235,232,0.5)', borderRadius: '0 1px 1px 0' }} />
            </div>
          </div>
        </div>

        {/* Screen content */}
        <div className="absolute inset-0 pt-14 overflow-y-auto scrollbar-hide">
          {children}
        </div>

        {/* Home indicator */}
        <div
          className="absolute bottom-2 left-1/2 -translate-x-1/2 z-40"
          style={{ width: 134, height: 5, borderRadius: 3, background: 'rgba(245,235,232,0.2)' }}
        />
      </div>
    </div>
  )
}
