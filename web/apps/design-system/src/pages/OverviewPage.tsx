import { Palette, Type, Square, Zap } from 'lucide-react'
import { type Page } from '@/data/tokens'

interface OverviewPageProps {
  onNavigate: (page: Page) => void
}

interface CardLink {
  id: Page
  icon: React.ReactNode
  title: string
  description: string
  accent: string
  tokenCount: number
}

const cards: CardLink[] = [
  {
    id: 'colors',
    icon: <Palette size={22} />,
    title: 'Colors',
    description: 'Primitive palette and semantic color roles — primary, surface, error, success.',
    accent: '#2563eb',
    tokenCount: 18,
  },
  {
    id: 'typography',
    icon: <Type size={22} />,
    title: 'Typography',
    description: 'Type scale from display to label, plus font family definitions.',
    accent: '#146c2e',
    tokenCount: 17,
  },
  {
    id: 'shape',
    icon: <Square size={22} />,
    title: 'Shape',
    description: 'Border radius scale from zero to fully-rounded pill shapes.',
    accent: '#5f6368',
    tokenCount: 8,
  },
  {
    id: 'motion',
    icon: <Zap size={22} />,
    title: 'Motion',
    description: 'Duration and easing tokens following Material Design physics.',
    accent: '#ba1a1a',
    tokenCount: 11,
  },
]

export default function OverviewPage({ onNavigate }: OverviewPageProps) {
  return (
    <div style={{ maxWidth: 860, margin: '0 auto', padding: '56px 40px' }}>
      {/* Hero */}
      <div style={{ marginBottom: 64 }}>
        <div
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 8,
            backgroundColor: 'var(--color-primary-container)',
            color: 'var(--color-on-primary-container)',
            padding: '4px 12px',
            borderRadius: 'var(--shape-full)',
            fontSize: 12,
            fontWeight: 600,
            letterSpacing: '0.04em',
            marginBottom: 20,
          }}
        >
          @hss/tokens v1.0.0
        </div>

        <h1
          style={{
            fontSize: 'var(--typescale-display-small)',
            fontWeight: 700,
            color: 'var(--color-on-surface)',
            margin: '0 0 16px',
            lineHeight: 1.1,
            letterSpacing: '-0.02em',
          }}
        >
          HSS Design System
        </h1>

        <p
          style={{
            fontSize: 'var(--typescale-body-large)',
            color: 'var(--color-on-surface-muted)',
            maxWidth: 560,
            margin: 0,
            lineHeight: 1.6,
          }}
        >
          A living token reference for building consistent, accessible interfaces across HSS Science products. Every token is a deliberate design decision.
        </p>
      </div>

      {/* Stats row */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(4, 1fr)',
          gap: 16,
          marginBottom: 56,
        }}
      >
        {[
          { value: '54', label: 'Total tokens' },
          { value: '4',  label: 'Categories' },
          { value: 'M3', label: 'Design basis' },
          { value: 'v4', label: 'Tailwind CSS' },
        ].map((stat) => (
          <div
            key={stat.label}
            style={{
              backgroundColor: 'var(--color-surface-variant)',
              borderRadius: 'var(--shape-md)',
              padding: '20px 16px',
              border: '1px solid var(--color-outline-variant)',
            }}
          >
            <div
              style={{
                fontSize: 28,
                fontWeight: 700,
                color: 'var(--color-primary)',
                letterSpacing: '-0.02em',
                lineHeight: 1,
                marginBottom: 6,
              }}
            >
              {stat.value}
            </div>
            <div style={{ fontSize: 12, color: 'var(--color-on-surface-muted)', fontWeight: 500 }}>
              {stat.label}
            </div>
          </div>
        ))}
      </div>

      {/* Category cards */}
      <div>
        <h2
          style={{
            fontSize: 'var(--typescale-title-large)',
            fontWeight: 600,
            marginBottom: 20,
            color: 'var(--color-on-surface)',
          }}
        >
          Foundations
        </h2>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          {cards.map((card) => (
            <button
              key={card.id}
              onClick={() => onNavigate(card.id)}
              style={{
                textAlign: 'left',
                background: 'var(--color-surface-variant)',
                border: '1px solid var(--color-outline-variant)',
                borderRadius: 'var(--shape-lg)',
                padding: 24,
                cursor: 'pointer',
                transition: `
                  box-shadow var(--motion-duration-short2) var(--motion-easing-standard),
                  border-color var(--motion-duration-short2) var(--motion-easing-standard),
                  transform var(--motion-duration-short2) var(--motion-easing-standard)
                `,
                fontFamily: 'var(--font-family-base)',
              }}
              onMouseEnter={(e) => {
                const el = e.currentTarget
                el.style.borderColor = 'var(--color-outline)'
                el.style.boxShadow = '0 4px 16px rgba(0,0,0,0.08)'
                el.style.transform = 'translateY(-2px)'
              }}
              onMouseLeave={(e) => {
                const el = e.currentTarget
                el.style.borderColor = 'var(--color-outline-variant)'
                el.style.boxShadow = 'none'
                el.style.transform = 'translateY(0)'
              }}
            >
              <div
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 'var(--shape-sm)',
                  backgroundColor: card.accent,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: '#fff',
                  marginBottom: 16,
                }}
              >
                {card.icon}
              </div>

              <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 8 }}>
                <span
                  style={{
                    fontSize: 'var(--typescale-title-medium)',
                    fontWeight: 600,
                    color: 'var(--color-on-surface)',
                  }}
                >
                  {card.title}
                </span>
                <span
                  style={{
                    fontSize: 11,
                    color: 'var(--color-on-surface-muted)',
                    backgroundColor: 'var(--color-outline-variant)',
                    padding: '1px 7px',
                    borderRadius: 'var(--shape-full)',
                  }}
                >
                  {card.tokenCount} tokens
                </span>
              </div>

              <p
                style={{
                  margin: 0,
                  fontSize: 'var(--typescale-body-medium)',
                  color: 'var(--color-on-surface-muted)',
                  lineHeight: 1.5,
                }}
              >
                {card.description}
              </p>
            </button>
          ))}
        </div>
      </div>

      {/* Usage hint */}
      <div
        style={{
          marginTop: 48,
          padding: '20px 24px',
          backgroundColor: 'var(--color-primary-container)',
          borderRadius: 'var(--shape-lg)',
          border: '1px solid #c8d8ff',
        }}
      >
        <div
          style={{
            fontSize: 12,
            fontWeight: 600,
            color: 'var(--color-on-primary-container)',
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            marginBottom: 8,
          }}
        >
          Quick start
        </div>
        <code
          style={{
            fontFamily: 'var(--font-family-mono)',
            fontSize: 13,
            color: 'var(--color-on-primary-container)',
          }}
        >
          @import "@hss/tokens"; /* in your CSS entry */
        </code>
      </div>
    </div>
  )
}
