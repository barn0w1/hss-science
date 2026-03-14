import { useState } from 'react'
import { type Page } from '@/data/tokens'
import Sidebar from '@/components/Sidebar'
import OverviewPage from '@/pages/OverviewPage'
import ColorsPage from '@/pages/ColorsPage'
import TypographyPage from '@/pages/TypographyPage'
import ShapePage from '@/pages/ShapePage'
import MotionPage from '@/pages/MotionPage'

function renderPage(page: Page, onNavigate: (p: Page) => void) {
  switch (page) {
    case 'overview':   return <OverviewPage onNavigate={onNavigate} />
    case 'colors':     return <ColorsPage />
    case 'typography': return <TypographyPage />
    case 'shape':      return <ShapePage />
    case 'motion':     return <MotionPage />
  }
}

export default function App() {
  const [page, setPage] = useState<Page>('overview')

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      <Sidebar current={page} onNavigate={setPage} />
      <main
        key={page}
        className="page-enter"
        style={{
          flex: 1,
          overflowY: 'auto',
          backgroundColor: 'var(--color-surface)',
        }}
      >
        {renderPage(page, setPage)}
      </main>
    </div>
  )
}
