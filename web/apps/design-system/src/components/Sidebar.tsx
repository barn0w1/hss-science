import { type Page } from '@/data/tokens'
import {
  LayoutGrid,
  Palette,
  Type,
  Square,
  Zap,
} from 'lucide-react'

interface NavItem {
  id: Page
  label: string
  icon: React.ReactNode
}

const navItems: NavItem[] = [
  { id: 'overview',    label: 'Overview',    icon: <LayoutGrid size={18} /> },
  { id: 'colors',      label: 'Colors',      icon: <Palette    size={18} /> },
  { id: 'typography',  label: 'Typography',  icon: <Type       size={18} /> },
  { id: 'shape',       label: 'Shape',       icon: <Square     size={18} /> },
  { id: 'motion',      label: 'Motion',      icon: <Zap        size={18} /> },
]

interface SidebarProps {
  current: Page
  onNavigate: (page: Page) => void
}

export default function Sidebar({ current, onNavigate }: SidebarProps) {
  return (
    <aside
      style={{
        width: 220,
        minWidth: 220,
        backgroundColor: 'var(--color-surface-variant)',
        borderRight: '1px solid var(--color-outline-variant)',
        display: 'flex',
        flexDirection: 'column',
        padding: '0',
        height: '100vh',
        position: 'sticky',
        top: 0,
      }}
    >
      {/* Brand */}
      <div
        style={{
          padding: '24px 20px 20px',
          borderBottom: '1px solid var(--color-outline-variant)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
          }}
        >
          <div
            style={{
              width: 28,
              height: 28,
              borderRadius: 'var(--shape-sm)',
              backgroundColor: 'var(--color-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <rect x="1" y="1" width="5" height="5" rx="1" fill="white" opacity="0.9"/>
              <rect x="8" y="1" width="5" height="5" rx="1" fill="white" opacity="0.6"/>
              <rect x="1" y="8" width="5" height="5" rx="1" fill="white" opacity="0.6"/>
              <rect x="8" y="8" width="5" height="5" rx="1" fill="white" opacity="0.3"/>
            </svg>
          </div>
          <div>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-on-surface)', letterSpacing: '-0.01em' }}>
              HSS Design
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-on-surface-muted)', marginTop: 1 }}>
              Token Reference
            </div>
          </div>
        </div>
      </div>

      {/* Nav */}
      <nav style={{ padding: '12px 10px', flex: 1 }}>
        <div style={{ fontSize: 10, fontWeight: 600, color: 'var(--color-on-surface-muted)', letterSpacing: '0.08em', textTransform: 'uppercase', padding: '4px 10px 8px' }}>
          Foundations
        </div>
        {navItems.map((item) => {
          const isActive = current === item.id
          return (
            <button
              key={item.id}
              onClick={() => onNavigate(item.id)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                width: '100%',
                padding: '8px 10px',
                borderRadius: 'var(--shape-sm)',
                border: 'none',
                background: isActive ? 'var(--color-primary-container)' : 'transparent',
                color: isActive ? 'var(--color-on-primary-container)' : 'var(--color-on-surface-muted)',
                fontSize: 13,
                fontWeight: isActive ? 600 : 400,
                fontFamily: 'var(--font-family-base)',
                cursor: 'pointer',
                textAlign: 'left',
                transition: `background var(--motion-duration-short2) var(--motion-easing-standard), color var(--motion-duration-short2) var(--motion-easing-standard)`,
                marginBottom: 2,
              }}
              onMouseEnter={(e) => {
                if (!isActive) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-outline-variant)'
                  ;(e.currentTarget as HTMLButtonElement).style.color = 'var(--color-on-surface)'
                }
              }}
              onMouseLeave={(e) => {
                if (!isActive) {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent'
                  ;(e.currentTarget as HTMLButtonElement).style.color = 'var(--color-on-surface-muted)'
                }
              }}
            >
              <span style={{ opacity: isActive ? 1 : 0.75 }}>{item.icon}</span>
              {item.label}
            </button>
          )
        })}
      </nav>

      {/* Footer */}
      <div
        style={{
          padding: '12px 20px',
          borderTop: '1px solid var(--color-outline-variant)',
          fontSize: 11,
          color: 'var(--color-on-surface-muted)',
        }}
      >
        @hss/tokens v1.0.0
      </div>
    </aside>
  )
}
