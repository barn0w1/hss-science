import { useState, useEffect, useRef } from 'react'
import { motionDurationTokens, motionEasingTokens } from '@/data/tokens'
import { Play } from 'lucide-react'

function DurationDemo({ ms, variable }: { ms: number; variable: string }) {
  const [playing, setPlaying] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const play = () => {
    if (playing) return
    setPlaying(true)
    setTimeout(() => setPlaying(false), ms + 50)
  }

  useEffect(() => {
    if (playing && ref.current) {
      ref.current.style.transform = 'scaleX(1)'
    } else if (!playing && ref.current) {
      ref.current.style.transform = 'scaleX(0)'
    }
  }, [playing])

  return (
    <button
      onClick={play}
      style={{
        display: 'grid',
        gridTemplateColumns: '100px 1fr 80px',
        alignItems: 'center',
        gap: 20,
        width: '100%',
        background: 'none',
        border: 'none',
        padding: '14px 16px',
        borderRadius: 'var(--shape-md)',
        cursor: 'pointer',
        fontFamily: 'var(--font-family-base)',
        transition: `background var(--motion-duration-short1) var(--motion-easing-standard)`,
      }}
      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-surface-variant)' }}
      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none' }}
    >
      {/* Meta */}
      <div style={{ textAlign: 'left' }}>
        <code style={{ fontFamily: 'var(--font-family-mono)', fontSize: 10, color: 'var(--color-on-surface-muted)', display: 'block', marginBottom: 2 }}>
          {variable}
        </code>
        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-on-surface)' }}>{ms}ms</span>
      </div>

      {/* Track */}
      <div
        style={{
          height: 6,
          backgroundColor: 'var(--color-outline-variant)',
          borderRadius: 'var(--shape-full)',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        <div
          ref={ref}
          style={{
            position: 'absolute',
            inset: 0,
            backgroundColor: 'var(--color-primary)',
            transformOrigin: 'left',
            transform: 'scaleX(0)',
            transition: playing ? `transform ${ms}ms var(--motion-easing-standard)` : 'none',
            borderRadius: 'var(--shape-full)',
          }}
        />
      </div>

      {/* Play indicator */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 4, color: playing ? 'var(--color-primary)' : 'var(--color-on-surface-muted)' }}>
        <Play size={12} fill={playing ? 'var(--color-primary)' : 'transparent'} />
        <span style={{ fontSize: 11 }}>{playing ? 'playing' : 'click'}</span>
      </div>
    </button>
  )
}

function EasingDemo({ variable, value, label }: { variable: string; value: string; label: string }) {
  const [playing, setPlaying] = useState(false)
  const ballRef = useRef<HTMLDivElement>(null)

  const play = () => {
    if (playing) return
    setPlaying(true)
    setTimeout(() => setPlaying(false), 650)
  }

  useEffect(() => {
    if (ballRef.current) {
      if (playing) {
        ballRef.current.style.transform = 'translateX(calc(100% + 32px))'
        ballRef.current.style.transition = `transform 600ms ${value}`
      } else {
        ballRef.current.style.transition = 'none'
        ballRef.current.style.transform = 'translateX(0)'
      }
    }
  }, [playing, value])

  return (
    <div
      style={{
        background: 'var(--color-surface-variant)',
        border: '1px solid var(--color-outline-variant)',
        borderRadius: 'var(--shape-lg)',
        padding: 20,
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
        <div>
          <div style={{ fontSize: 'var(--typescale-label-large)', fontWeight: 600, color: 'var(--color-on-surface)', marginBottom: 2 }}>
            {label}
          </div>
          <code style={{ fontFamily: 'var(--font-family-mono)', fontSize: 10, color: 'var(--color-on-surface-muted)' }}>
            {value}
          </code>
        </div>
        <button
          onClick={play}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '5px 12px',
            borderRadius: 'var(--shape-full)',
            border: '1.5px solid var(--color-primary)',
            background: playing ? 'var(--color-primary)' : 'transparent',
            color: playing ? '#fff' : 'var(--color-primary)',
            fontSize: 11,
            fontWeight: 600,
            cursor: playing ? 'default' : 'pointer',
            fontFamily: 'var(--font-family-base)',
            transition: `background var(--motion-duration-short2) var(--motion-easing-standard), color var(--motion-duration-short2) var(--motion-easing-standard)`,
          }}
        >
          <Play size={10} fill="currentColor" />
          {playing ? 'playing…' : 'Play'}
        </button>
      </div>

      {/* Track */}
      <div
        style={{
          height: 32,
          backgroundColor: 'var(--color-outline-variant)',
          borderRadius: 'var(--shape-full)',
          position: 'relative',
          overflow: 'hidden',
          padding: '4px 8px',
          display: 'flex',
          alignItems: 'center',
        }}
      >
        {/* Rail line */}
        <div
          style={{
            position: 'absolute',
            left: 20,
            right: 20,
            height: 1,
            backgroundColor: 'var(--color-outline)',
            opacity: 0.3,
          }}
        />

        {/* Ball */}
        <div
          ref={ballRef}
          style={{
            width: 24,
            height: 24,
            borderRadius: 'var(--shape-full)',
            backgroundColor: 'var(--color-primary)',
            flexShrink: 0,
            position: 'relative',
            zIndex: 1,
            boxShadow: '0 2px 6px rgba(37,99,235,0.35)',
          }}
        />
      </div>

      {/* Token name */}
      <code
        style={{
          fontFamily: 'var(--font-family-mono)',
          fontSize: 10,
          color: 'var(--color-on-surface-muted)',
          display: 'block',
          marginTop: 10,
        }}
      >
        {variable}
      </code>
    </div>
  )
}

export default function MotionPage() {
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
          Motion
        </h1>
        <p style={{ margin: 0, fontSize: 'var(--typescale-body-large)', color: 'var(--color-on-surface-muted)', lineHeight: 1.6 }}>
          Motion makes interfaces feel responsive and alive. Duration tokens control timing;
          easing tokens define the character of movement.
        </p>
      </div>

      {/* Duration */}
      <section style={{ marginBottom: 56 }}>
        <SectionLabel>Duration</SectionLabel>
        <p style={{ fontSize: 'var(--typescale-body-medium)', color: 'var(--color-on-surface-muted)', margin: '0 0 20px', lineHeight: 1.5 }}>
          Click any row to preview the duration. Short durations (50–100ms) for micro-interactions;
          longer ones (450–600ms) for complex spatial transitions.
        </p>

        <div
          style={{
            border: '1px solid var(--color-outline-variant)',
            borderRadius: 'var(--shape-lg)',
            overflow: 'hidden',
          }}
        >
          {motionDurationTokens.map((token, i) => (
            <div
              key={token.variable}
              style={{
                borderBottom: i < motionDurationTokens.length - 1 ? '1px solid var(--color-outline-variant)' : 'none',
              }}
            >
              <DurationDemo ms={token.ms} variable={token.variable} />
            </div>
          ))}
        </div>
      </section>

      {/* Easing */}
      <section>
        <SectionLabel>Easing</SectionLabel>
        <p style={{ fontSize: 'var(--typescale-body-medium)', color: 'var(--color-on-surface-muted)', margin: '0 0 24px', lineHeight: 1.5 }}>
          Physics-based curves from Material Design. Press <strong>Play</strong> on each card to feel the difference.
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 16 }}>
          {motionEasingTokens.map((token) => (
            <EasingDemo
              key={token.variable}
              variable={token.variable}
              value={token.value}
              label={token.label}
            />
          ))}
        </div>

        {/* Easing table */}
        <div
          style={{
            marginTop: 24,
            border: '1px solid var(--color-outline-variant)',
            borderRadius: 'var(--shape-lg)',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr 1fr',
              backgroundColor: 'var(--color-surface-variant)',
              padding: '8px 16px',
              borderBottom: '1px solid var(--color-outline-variant)',
            }}
          >
            {['Token', 'Curve', 'Description'].map((h) => (
              <div key={h} style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-on-surface-muted)', letterSpacing: '0.04em', textTransform: 'uppercase' }}>
                {h}
              </div>
            ))}
          </div>
          {motionEasingTokens.map((token, i) => (
            <div
              key={token.variable}
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr 1fr',
                padding: '12px 16px',
                borderBottom: i < motionEasingTokens.length - 1 ? '1px solid var(--color-outline-variant)' : 'none',
                alignItems: 'center',
              }}
            >
              <code style={{ fontFamily: 'var(--font-family-mono)', fontSize: 10, color: 'var(--color-on-surface-muted)' }}>
                {token.variable}
              </code>
              <code style={{ fontFamily: 'var(--font-family-mono)', fontSize: 10, color: 'var(--color-on-surface)' }}>
                {token.value}
              </code>
              <div style={{ fontSize: 12, color: 'var(--color-on-surface-muted)' }}>
                {token.description}
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
      }}
    >
      {children}
    </div>
  )
}
