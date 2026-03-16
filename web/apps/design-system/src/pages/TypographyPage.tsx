import { typescaleTokens } from '@/data/tokens'

const groups = ['Display', 'Headline', 'Title', 'Body', 'Label'] as const

const sampleTexts: Record<typeof groups[number], string> = {
  Display:  'The quick brown fox',
  Headline: 'Building with design tokens',
  Title:    'Component foundations',
  Body:     'Design tokens are the single source of truth for design decisions — from color to motion.',
  Label:    'Caption · Badge · Chip',
}

export default function TypographyPage() {
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
          Typography
        </h1>
        <p
          style={{
            margin: 0,
            fontSize: 'var(--typescale-body-large)',
            color: 'var(--color-on-surface-muted)',
            lineHeight: 1.6,
          }}
        >
          The type scale is adapted from Material Design 3. Fifteen roles across five groups
          create a clear visual hierarchy.
        </p>
      </div>

      {/* Font Families */}
      <section style={{ marginBottom: 56 }}>
        <SectionLabel>Font Families</SectionLabel>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          {[
            {
              variable: '--font-family-base',
              family: "'Google Sans', 'Roboto', system-ui, sans-serif",
              label: 'Base',
              sample: 'Aa Bb Cc 123',
              mono: false,
            },
            {
              variable: '--font-family-mono',
              family: "'Roboto Mono', monospace",
              label: 'Mono',
              sample: 'Aa Bb Cc 123',
              mono: true,
            },
          ].map((f) => (
            <div
              key={f.variable}
              style={{
                background: 'var(--color-surface-variant)',
                border: '1px solid var(--color-outline-variant)',
                borderRadius: 'var(--shape-lg)',
                padding: 24,
              }}
            >
              <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-on-surface-muted)', letterSpacing: '0.04em', textTransform: 'uppercase', marginBottom: 12 }}>
                {f.label}
              </div>
              <div
                style={{
                  fontFamily: f.mono ? 'var(--font-family-mono)' : 'var(--font-family-base)',
                  fontSize: 36,
                  fontWeight: 400,
                  letterSpacing: '-0.01em',
                  color: 'var(--color-on-surface)',
                  marginBottom: 16,
                  lineHeight: 1,
                }}
              >
                {f.sample}
              </div>
              <code
                style={{
                  fontFamily: 'var(--font-family-mono)',
                  fontSize: 11,
                  color: 'var(--color-on-surface-muted)',
                  display: 'block',
                  marginBottom: 4,
                }}
              >
                {f.variable}
              </code>
              <div style={{ fontSize: 11, color: 'var(--color-on-surface-muted)', wordBreak: 'break-all' }}>
                {f.family}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Type Scale */}
      <section>
        <SectionLabel>Type Scale</SectionLabel>
        <p
          style={{
            fontSize: 'var(--typescale-body-medium)',
            color: 'var(--color-on-surface-muted)',
            margin: '0 0 28px',
            lineHeight: 1.5,
          }}
        >
          Each role combines a size token with an appropriate weight. Live text samples below.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 40 }}>
          {groups.map((group) => {
            const groupTokens = typescaleTokens.filter((t) => t.group === group)
            return (
              <div key={group}>
                {/* Group label */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 12,
                    marginBottom: 16,
                  }}
                >
                  <span
                    style={{
                      fontSize: 11,
                      fontWeight: 700,
                      letterSpacing: '0.08em',
                      textTransform: 'uppercase',
                      color: 'var(--color-primary)',
                    }}
                  >
                    {group}
                  </span>
                  <hr
                    style={{
                      flex: 1,
                      border: 'none',
                      borderTop: '1px solid var(--color-outline-variant)',
                    }}
                  />
                </div>

                {/* Tokens in this group */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
                  {groupTokens.map((token, i) => (
                    <div
                      key={token.variable}
                      style={{
                        display: 'grid',
                        gridTemplateColumns: '200px 1fr',
                        alignItems: 'baseline',
                        padding: '14px 0',
                        borderBottom: i < groupTokens.length - 1 ? '1px solid var(--color-outline-variant)' : 'none',
                        gap: 24,
                      }}
                    >
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
                        <div style={{ fontSize: 12, color: 'var(--color-on-surface-muted)', fontWeight: 500 }}>
                          {token.value}
                        </div>
                      </div>

                      {/* Live sample */}
                      <div
                        style={{
                          fontSize: `var(${token.variable})`,
                          fontFamily: 'var(--font-family-base)',
                          color: 'var(--color-on-surface)',
                          lineHeight: 1.2,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {sampleTexts[group]}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
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
