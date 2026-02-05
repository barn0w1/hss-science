import type { ReactNode } from 'react';
import { DebugPlaceholder } from '@/shared/ui/DebugPlaceholder';

// ----------------------------------------------------------------------
// Layout Debug Mode
// Set this to true to replace content with colored placeholders
// ----------------------------------------------------------------------
const IS_LAYOUT_DEBUG = true;

interface MainLayoutProps {
  header?: ReactNode;
  sidebar: ReactNode;
  children: ReactNode;
}

export const MainLayout = ({ header, sidebar, children }: MainLayoutProps) => {
  return (
    <div className="layout-screen">
      {header && (
        <header className="layout-header">
          {IS_LAYOUT_DEBUG ? (
            <DebugPlaceholder label="Header Area" color="bg-yellow-500/20 border-yellow-500/50 text-yellow-700" />
          ) : (
            header
          )}
        </header>
      )}

      <main className="layout-main">
        <aside className="layout-sidebar">
          {IS_LAYOUT_DEBUG ? (
            <DebugPlaceholder label="Sidebar" color="bg-red-500/20 border-red-500/50 text-red-700" />
          ) : (
            sidebar
          )}
        </aside>

        <section className="layout-content">
          {IS_LAYOUT_DEBUG ? (
            <DebugPlaceholder label="Content Area" color="bg-blue-500/20 border-blue-500/50 text-blue-700" />
          ) : (
            children
          )}
        </section>
      </main>
    </div>
  );
};