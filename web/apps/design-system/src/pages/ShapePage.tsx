import { shapeTokens } from '@/data/tokens'

export default function ShapePage() {
  return (
    <div style={{ maxWidth: 860, margin: '0 auto', padding: '56px 40px' }}>
      {/* Header */}
      <div style={{ marginBottom: 48 }}>
        <h1
          style={{
            fontSize: 'var(--typescale-headline-large)',
            fontWeight: 700,
            margin: '0 0 12px',
            letterSpacing: '-0.02em',
          }}
        >
          Shape
        </h1>
        <p
          style={{
            margin: 0,
            fontSize: 'var(--typescale-body-large)',
            color: 'var(--color-on-surface-muted)',
            lineHeight: 1.6,
          }}
        >
          Eight levels of border radius from none to full. Shape communicates the personality
          of a component — from structured to playful.
        </p>
      </div>

      {/* Visual demo — scale */}
      <section style={{ marginBottom: 56 }}>
        <SectionLabel>Scale</SectionLabel>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {shapeTokens.map((token) => (
            <div
              key={token.variable}
              style={{
                display: 'grid',
                gridTemplateColumns: '60px 120px 1fr',
                alignItems: 'center',
                gap: 24,
              }}
            >
              {/* Badge */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 52,
                  height: 52,
                  backgroundColor: 'var(--color-primary)',
                  borderRadius: `var(${token.variable})`,
                  transition: `border-radius var(--motion-duration-medium1) var(--motion-easing-standard)`,
                }}
              >
                <span style={{ fontSize: 11, fontWeight: 700, color: '#fff' }}>{token.label}</span>
              </div>

              {/* Meta */}
              <div>
                <code
                  style={{
                    fontFamily: 'var(--font-family-mono)',
                    fontSize: 11,
                    color: 'var(--color-on-surface-muted)',
                    display: 'block',
                    marginBottom: 2,
                  }}
                >
                  {token.variable}
                </code>
                <div
                  style={{
                    fontSize: 12,
                    color: 'var(--color-on-surface)',
                    fontWeight: 600,
                  }}
                >
                  {token.value}
                </div>
              </div>

              {/* Description */}
              <div
                style={{
                  fontSize: 'var(--typescale-body-small)',
                  color: 'var(--color-on-surface-muted)',
                }}
              >
                {token.description}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Contextual usage examples */}
      <section>
        <SectionLabel>Usage Examples</SectionLabel>
        <p
          style={{
            fontSize: 'var(--typescale-body-medium)',
            color: 'var(--color-on-surface-muted)',
            margin: '0 0 28px',
            lineHeight: 1.5,
          }}
        >
          Shape tokens applied to common component patterns.
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 16 }}>
          {/* Button examples */}
          <div
            style={{
              background: 'var(--color-surface-variant)',
              border: '1px solid var(--color-outline-variant)',
              borderRadius: 'var(--shape-lg)',
              padding: 24,
            }}
          >
            <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-on-surface-muted)', marginBottom: 16 }}>
              Buttons
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
              {[
                { label: 'Square', shape: '--shape-none' },
                { label: 'XS',     shape: '--shape-xs'   },
                { label: 'SM',     shape: '--shape-sm'   },
                { label: 'MD',     shape: '--shape-md'   },
                { label: 'Full',   shape: '--shape-full' },
              ].map((b) => (
                <button
                  key={b.shape}
                  style={{
                    padding: '8px 16px',
                    borderRadius: `var(${b.shape})`,
                    border: '1.5px solid var(--color-primary)',
                    background: 'transparent',
                    color: 'var(--color-primary)',
                    fontSize: 13,
                    fontWeight: 500,
                    fontFamily: 'var(--font-family-base)',
                    cursor: 'default',
                  }}
                >
                  {b.label}
                </button>
              ))}
            </div>
          </div>

          {/* Card examples */}
          <div
            style={{
              background: 'var(--color-surface-variant)',
              border: '1px solid var(--color-outline-variant)',
              borderRadius: 'var(--shape-lg)',
              padding: 24,
            }}
          >
            <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-on-surface-muted)', marginBottom: 16 }}>
              Cards
            </div>
            <div style={{ display: 'flex', gap: 10 }}>
              {['--shape-sm', '--shape-md', '--shape-lg', '--shape-xl'].map((shape) => (
                <div
                  key={shape}
                  style={{
                    flex: 1,
                    height: 56,
                    borderRadius: `var(${shape})`,
                    backgroundColor: 'var(--color-primary-container)',
                    border: '1px solid rgba(37,99,235,0.15)',
                  }}
                />
              ))}
            </div>
          </div>

          {/* Avatar / chip examples */}
          <div
            style={{
              background: 'var(--color-surface-variant)',
              border: '1px solid var(--color-outline-variant)',
              borderRadius: 'var(--shape-lg)',
              padding: 24,
            }}
          >
            <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-on-surface-muted)', marginBottom: 16 }}>
              Avatars
            </div>
            <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
              {[
                { shape: '--shape-sm',   size: 36 },
                { shape: '--shape-md',   size: 40 },
                { shape: '--shape-xl',   size: 48 },
                { shape: '--shape-full', size: 48 },
              ].map((a) => (
                <div
                  key={a.shape}
                  style={{
                    width: a.size,
                    height: a.size,
                    borderRadius: `var(${a.shape})`,
                    backgroundColor: 'var(--color-primary)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    fontSize: 14,
                    fontWeight: 600,
                  }}
                >
                  H
                </div>
              ))}
            </div>
          </div>

          {/* Input examples */}
          <div
            style={{
              background: 'var(--color-surface-variant)',
              border: '1px solid var(--color-outline-variant)',
              borderRadius: 'var(--shape-lg)',
              padding: 24,
            }}
          >
            <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--color-on-surface-muted)', marginBottom: 16 }}>
              Inputs
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {[
                { shape: '--shape-xs',   label: 'Minimal rounding' },
                { shape: '--shape-md',   label: 'Standard input'   },
                { shape: '--shape-full', label: 'Search field'     },
              ].map((inp) => (
                <div
                  key={inp.shape}
                  style={{
                    padding: '8px 14px',
                    borderRadius: `var(${inp.shape})`,
                    border: '1.5px solid var(--color-outline)',
                    backgroundColor: 'var(--color-surface)',
                    fontSize: 13,
                    color: 'var(--color-on-surface-muted)',
                    fontFamily: 'var(--font-family-base)',
                  }}
                >
                  {inp.label}
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        fontSize: 'var(--typescale-headline-small)',
        fontWeight: 600,
        marginBottom: 16,
        letterSpacing: '-0.01em',
      }}
    >
      {children}
    </div>
  )
}
