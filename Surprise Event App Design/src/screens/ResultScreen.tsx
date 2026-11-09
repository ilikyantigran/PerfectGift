import type { AppData } from '../App'

interface Idea {
  title: string
  description: string
  cost: string
  tags: string[]
  gradient: string
  accentColor: string
  icon: string
}

function getIdeas(data: AppData): Idea[] {
  const budget = data.budget || '$50–$150'

  const pools: Record<string, Idea[]> = {
    low: [
      {
        title: 'Handwritten Love Letter & Picnic',
        description: `Craft a heartfelt letter recounting your favorite shared memories, then spread a blanket at her favorite spot with homemade treats, candles, and a playlist of your songs. Simple things she'll remember forever.`,
        cost: '~$25–40',
        tags: ['Romantic', 'DIY', 'Outdoor'],
        gradient: 'linear-gradient(135deg, rgba(212,119,138,0.15), rgba(212,119,138,0.05))',
        accentColor: '#d4778a',
        icon: '✉️',
      },
      {
        title: 'Movie Night Under the Stars',
        description: `Borrow a projector, set up blankets and fairy lights in the garden or on the balcony, pick her three favorite films, and stock up on her favorite snacks. Add a cozy blanket with her name on it.`,
        cost: '~$30–50',
        tags: ['Cozy', 'Evening', 'At Home'],
        gradient: 'linear-gradient(135deg, rgba(232,160,122,0.15), rgba(232,160,122,0.05))',
        accentColor: '#e8a07a',
        icon: '🎬',
      },
      {
        title: 'Spa Evening at Home',
        description: `Transform the bathroom into a sanctuary — rose petals, bath bombs, essential oils, her favorite music softly playing. Prepare a charcuterie board and wine to enjoy after. She'll feel like a queen.`,
        cost: '~$35–50',
        tags: ['Relaxing', 'Intimate', 'Evening'],
        gradient: 'linear-gradient(135deg, rgba(180,140,200,0.15), rgba(180,140,200,0.05))',
        accentColor: '#c49ed4',
        icon: '🛁',
      },
    ],
    mid: [
      {
        title: 'Sunset Sailing Experience',
        description: `Book a private 2-hour sunset sail on the bay. Pack champagne, her favorite cheese, and fresh strawberries. As the sun dips below the horizon, let the moment speak for itself. Absolutely unforgettable.`,
        cost: '~$90–130',
        tags: ['Adventure', 'Romantic', 'Outdoors'],
        gradient: 'linear-gradient(135deg, rgba(232,160,122,0.18), rgba(212,119,138,0.08))',
        accentColor: '#e8a07a',
        icon: '⛵',
      },
      {
        title: 'Private Chef Dinner at Home',
        description: `Hire a local chef to prepare a 4-course dinner in your home. You set the table with candles and flowers, she arrives to find a restaurant-quality experience made entirely for her. Intimate and extraordinary.`,
        cost: '~$100–150',
        tags: ['Luxurious', 'Intimate', 'Culinary'],
        gradient: 'linear-gradient(135deg, rgba(212,119,138,0.18), rgba(180,140,200,0.08))',
        accentColor: '#d4778a',
        icon: '👨‍🍳',
      },
      {
        title: 'Surprise Weekend Escape',
        description: `Book a charming cabin or boutique hotel 1–2 hours away. Pack her bag in secret, blindfold her for the journey, and reveal the destination. Include a trail hike at sunrise and a fireside evening.`,
        cost: '~$120–150',
        tags: ['Adventure', 'Getaway', 'Surprise'],
        gradient: 'linear-gradient(135deg, rgba(100,180,160,0.15), rgba(100,180,160,0.05))',
        accentColor: '#64b4a0',
        icon: '🏕️',
      },
    ],
    high: [
      {
        title: 'Rooftop Fine Dining Under the Stars',
        description: `Reserve the exclusive rooftop of a Michelin-starred restaurant — an intimate table set with roses and a custom menu featuring her favorite ingredients, followed by a live acoustic performance just for the two of you.`,
        cost: '~$250–400',
        tags: ['Luxurious', 'Fine Dining', 'City'],
        gradient: 'linear-gradient(135deg, rgba(212,119,138,0.2), rgba(212,119,138,0.06))',
        accentColor: '#d4778a',
        icon: '🌆',
      },
      {
        title: 'Spa Retreat Weekend',
        description: `Book two nights at a renowned spa resort — couples massages, thermal baths, forest walks, and farm-to-table dining. No phones, no schedules, just pure presence. A true reset for both of you.`,
        cost: '~$350–500',
        tags: ['Wellness', 'Getaway', 'Romantic'],
        gradient: 'linear-gradient(135deg, rgba(180,140,200,0.2), rgba(180,140,200,0.06))',
        accentColor: '#c49ed4',
        icon: '🌸',
      },
      {
        title: 'Private Gallery Tour & Dinner',
        description: `Arrange an after-hours private tour of a gallery featuring her favorite artist, followed by a candlelit dinner at a restaurant with a custom menu inspired by the artworks. Culture, beauty, and intimacy combined.`,
        cost: '~$300–500',
        tags: ['Cultural', 'Exclusive', 'Art'],
        gradient: 'linear-gradient(135deg, rgba(232,160,122,0.2), rgba(100,150,200,0.06))',
        accentColor: '#e8a07a',
        icon: '🎨',
      },
    ],
  }

  const tier =
    budget === 'Under $50' ? 'low'
    : budget === '$50–$150' ? 'mid'
    : 'high'

  return pools[tier]
}

interface Props {
  onBack: () => void
  data: AppData
}

export default function ResultScreen({ onBack, data }: Props) {
  const ideas = getIdeas(data)

  return (
    <div
      className="min-h-full flex flex-col"
      style={{ background: 'linear-gradient(180deg, #130d18 0%, #07040a 100%)', paddingBottom: 24 }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', padding: '12px 24px 4px' }}>
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
        <div>
          <p style={{ color: '#a48a96', fontSize: 11, fontFamily: 'var(--font-body)', textTransform: 'uppercase', letterSpacing: '0.1em' }}>
            {data.holiday || 'Your surprise'}
          </p>
          <h2 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 20, fontWeight: 600 }}>
            3 Perfect Ideas
          </h2>
        </div>
      </div>

      {/* Subtitle */}
      <div style={{ padding: '12px 24px 20px' }}>
        <p style={{ color: '#a48a96', fontSize: 13, fontFamily: 'var(--font-body)', lineHeight: 1.6 }}>
          Crafted just for her{data.budget ? ` within ${data.budget}` : ''}. Tap an idea to save it.
        </p>
      </div>

      {/* Idea cards */}
      <div style={{ padding: '0 24px', display: 'flex', flexDirection: 'column', gap: 14 }}>
        {ideas.map((idea, i) => (
          <div
            key={i}
            style={{
              borderRadius: 22, border: '1px solid #2e1b2a',
              background: idea.gradient,
              overflow: 'hidden',
              animation: `fade-up 0.5s ease ${i * 0.12}s both`,
            }}
          >
            {/* Card header */}
            <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid rgba(46,27,42,0.6)' }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
                <div
                  style={{
                    width: 44, height: 44, borderRadius: 14, flexShrink: 0,
                    background: `${idea.accentColor}20`,
                    border: `1px solid ${idea.accentColor}35`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 22,
                  }}
                >
                  {idea.icon}
                </div>
                <div style={{ flex: 1 }}>
                  <h3 style={{ fontFamily: 'var(--font-display)', color: '#f5ebe8', fontSize: 17, fontWeight: 600, lineHeight: 1.3, marginBottom: 6 }}>
                    {idea.title}
                  </h3>
                  <div
                    style={{
                      display: 'inline-block', padding: '3px 10px', borderRadius: 8,
                      background: `${idea.accentColor}25`, border: `1px solid ${idea.accentColor}40`,
                    }}
                  >
                    <span style={{ color: idea.accentColor, fontSize: 12, fontFamily: 'var(--font-body)', fontWeight: 500 }}>
                      {idea.cost}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Description */}
            <div style={{ padding: '14px 18px' }}>
              <p style={{ color: '#c8b0bc', fontSize: 13, fontFamily: 'var(--font-body)', lineHeight: 1.7 }}>
                {idea.description}
              </p>
            </div>

            {/* Tags */}
            <div style={{ padding: '0 18px 16px', display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {idea.tags.map((tag) => (
                <span
                  key={tag}
                  style={{
                    padding: '4px 10px', borderRadius: 8, fontSize: 11,
                    color: '#a48a96', background: '#1a1020', border: '1px solid #2e1b2a',
                    fontFamily: 'var(--font-body)', fontWeight: 500, textTransform: 'uppercase', letterSpacing: '0.08em',
                  }}
                >
                  {tag}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Generate more */}
      <div style={{ padding: '20px 24px 0' }}>
        <button
          onClick={onBack}
          style={{
            width: '100%', padding: '14px 0', borderRadius: 18, border: '1px solid #2e1b2a',
            background: '#1a1020', color: '#a48a96', fontSize: 14, fontWeight: 500,
            fontFamily: 'var(--font-body)', cursor: 'pointer', display: 'flex',
            alignItems: 'center', justifyContent: 'center', gap: 6,
          }}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path d="M2 8a6 6 0 1 1 .3 2" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" />
            <path d="M2 4.5V8H5.5" stroke="#a48a96" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          Generate more ideas
        </button>
      </div>
    </div>
  )
}
