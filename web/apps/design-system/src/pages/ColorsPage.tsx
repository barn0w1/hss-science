import { primitiveColors, semanticColorGroups } from '@/data/tokens'

function groupBy<T, K extends string>(arr: T[], key: (item: T) => K): Record<K, T[]> {
  return arr.reduce((acc, item) => {
    const k = key(item)
    if (!acc[k]) acc[k] = []
    acc[k].push(item)
    return acc
  }, {} as Record<K, T[]>)
}

export default function ColorsPage() {
  const byGroup = groupBy(primitiveColors, (c) => c.group)

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
          Colors
        </h1>
        <p style={{ margin: 0, fontSize: 'var(--typescale-body-large)', color: 'var(--color-on-surface-muted)', lineHeight: 1.6 }}>
          The color system uses a two-tier model: <strong>primitive tokens</strong> define the raw palette,
          while <strong>semantic tokens</strong> assign meaning to those values for consistent usage in UI.
        </p>
      </div>

      {/* Primitive Palette */}
      <section style={{ marginBottom: 64 }}>
        <SectionLabel>Primitive Palette</SectionLabel>
        <p style={{ fontSize: 'var(--typescale-body-medium)', color: 'var(--color-on-surface-muted)', margin: '0 0 28px', lineHeight: 1.5 }}>
          Raw color values. Reference these only when defining semantic tokens — never directly in components.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          {(Object.entries(byGroup) as [string, typeof primitiveColors[number][]][]).map(([group, tokens]) => (
            <div key={group}>
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-on-surface-muted)', marginBottom: 10, letterSpacing: '0.04em', textTransform: 'uppercase' }}>
                {group}
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                {tokens.map((token) => (
                  <div key={token.variable} style={{ flex: 1 }}>
                    <div
                      style={{
                        height: 64,
                        borderRadius: 'var(--shape-md)',
                        backgroundColor: token.value,
                        border: '1px solid rgba(0,0,0,0.06)',
                        marginBottom: 8,
                        cursor: 'default',
                        transition: `transform var(--motion-duration-short2) var(--motion-easing-spring)`,
                      }}
                      title={`${token.variable}: ${token.value}`}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.transform = 'scale(1.06)' }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.transform = 'scale(1)' }}
                    />
                    <div style={{ fontSize: 11, color: 'var(--color-on-surface)', fontWeight: 500, marginBottom: 2 }}>
                      {token.shade}
                    </div>
                    <div style={{ fontFamily: 'var(--font-family-mono)', fontSize: 10, color: 'var(--color-on-surface-muted)' }}>
                      {token.value}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Semantic Colors */}
      <section>
        <SectionLabel>Semantic Colors</SectionLabel>
        <p style={{ fontSize: 'var(--typescale-body-medium)', color: 'var(--color-on-surface-muted)', margin: '0 0 28px', lineHeight: 1.5 }}>
          Semantic tokens carry intent. Use these throughout your components — they map to the palette above.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
          {semanticColorGroups.map((group) => (
            <div key={group.title}>
              <div style={{ marginBottom: 12 }}>
                <span
                  style={{
                    fontSize: 'var(--typescale-title-small)',
                    fontWeight: 600,
                    color: 'var(--color-on-surface)',
                  }}
                >
                  {group.title}
                </span>
                <span
                  style={{
                    marginLeft: 10,
                    fontSize: 12,
                    color: 'var(--color-on-surface-muted)',
                  }}
                >
                  {group.description}
                </span>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 12 }}>
                {group.tokens.map((token) => (
                  <div
                    key={token.variable}
                    style={{
                      borderRadius: 'var(--shape-md)',
                      overflow: 'hidden',
                      border: '1px solid var(--color-outline-variant)',
                    }}
                  >
                    {/* Color preview */}
                    <div
                      style={{
                        height: 72,
                        backgroundColor: token.value,
                        padding: 10,
                        display: 'flex',
                        alignItems: 'flex-end',
                      }}
                    >
                      <span
                        style={{
                          fontSize: 11,
                          fontWeight: 600,
                          color: token.textOnColor === 'light' ? 'rgba(255,255,255,0.85)' : 'rgba(0,0,0,0.55)',
                        }}
                      >
                        {token.label}
                      </span>
                    </div>

                    {/* Token info */}
                    <div
                      style={{
                        padding: '10px 12px',
                        backgroundColor: 'var(--color-surface-variant)',
                      }}
                    >
                      <div
                        style={{
                          fontFamily: 'var(--font-family-mono)',
                          fontSize: 10,
                          color: 'var(--color-on-surface-muted)',
                          marginBottom: 4,
                          wordBreak: 'break-all',
                        }}
                      >
                        {token.variable}
                      </div>
                      <div
                        style={{
                          fontFamily: 'var(--font-family-mono)',
                          fontSize: 11,
                          color: 'var(--color-on-surface)',
                          fontWeight: 500,
                        }}
                      >
                        {token.value}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
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
        marginBottom: 8,
        letterSpacing: '-0.01em',
        color: 'var(--color-on-surface)',
      }}
    >
      {children}
    </div>
  )
}
