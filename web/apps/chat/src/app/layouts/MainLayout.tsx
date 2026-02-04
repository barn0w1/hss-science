import type { ReactNode } from 'react';

interface MainLayoutProps {
  header?: ReactNode;
  sidebar: ReactNode;
  children: ReactNode;

  isSidebarOpen?: boolean;
}

export const MainLayout = ({ header, sidebar, isSidebarOpen = true, children }: MainLayoutProps) => {
  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-surface-50 text-surface-900 pt-[var(--spacing-header)]">
      {header && (
        <header className="fixed top-0 left-0 right-0 z-10 h-[var(--spacing-header)] flex items-center px-[var(--spacing-header-x)] bg-surface-50/90 backdrop-blur-sm">
          {header}
        </header>
      )}

      <main className="flex-1 flex min-w-0 px-[var(--spacing-gutter)] pb-[var(--spacing-gutter)] min-h-0">
        <aside
          className={`flex-shrink-0 flex flex-col py-6 overflow-hidden ${
            isSidebarOpen
              ? 'w-[var(--sidebar-width)] px-5 opacity-100'
              : 'w-[var(--sidebar-collapsed-width)] px-3 opacity-100'
          }`}
        >
          {sidebar}
        </aside>

        <section className="flex-1 flex flex-col min-w-0 ml-[var(--spacing-gutter)] rounded-[var(--radius-card)] bg-white/80 backdrop-blur-sm overflow-hidden border border-surface-200">
          {children}
        </section>
      </main>
    </div>
  );
};