import type { ReactNode } from 'react';

interface MainLayoutProps {
  header?: ReactNode;
  sidebar: ReactNode;
  children: ReactNode;
}

export const MainLayout = ({ header, sidebar, children }: MainLayoutProps) => {
  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-surface-50 text-surface-900 pt-[var(--layout-header-height)]">
      {header && (
        <header className="fixed top-0 left-0 right-0 z-10 h-[var(--layout-header-height)] flex items-center px-[var(--layout-header-padding-x)] bg-surface-50/90 backdrop-blur-sm">
          {header}
        </header>
      )}

      <main className="flex-1 flex min-w-0 px-[var(--layout-gutter)] pb-[var(--layout-gutter)] min-h-0">
        <aside className="flex-shrink-0 flex flex-col py-6 overflow-hidden w-[var(--layout-sidebar-width)] px-4">
          {sidebar}
        </aside>

        <section className="flex-1 flex flex-col min-w-0 ml-[var(--layout-gutter)] rounded-[var(--radius-panel)] bg-white/80 backdrop-blur-sm overflow-hidden">
          {children}
        </section>
      </main>
    </div>
  );
};