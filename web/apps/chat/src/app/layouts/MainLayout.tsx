import type { ReactNode } from 'react';

interface MainLayoutProps {
  header?: ReactNode;
  sidebar: ReactNode;
  children: ReactNode;

  isSidebarOpen?: boolean;
}

export const MainLayout = ({ header, sidebar, isSidebarOpen = true, children }: MainLayoutProps) => {
  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-surface-50 text-surface-900">
      {header && (
        <header className="fixed top-0 left-0 right-0 z-10 h-[var(--spacing-header)] flex items-center px-6 bg-surface-50/90 backdrop-blur-sm">
          {header}
        </header>
      )}

      <main className="flex-1 flex min-w-0 px-[var(--spacing-gutter)] pb-[var(--spacing-gutter)] pt-[calc(var(--spacing-header)+var(--spacing-gutter))]">
        <aside
          className={`flex-shrink-0 flex flex-col py-6 transition-all duration-180 ease-out overflow-hidden ${
            isSidebarOpen
              ? 'w-[var(--sidebar-width)] px-5 opacity-100'
              : 'w-[var(--sidebar-collapsed-width)] px-3 opacity-100'
          }`}
        >
          {sidebar}
        </aside>

        <section className="flex-1 flex flex-col min-w-0 ml-[var(--spacing-gutter)] rounded-[var(--radius-card)] bg-white/90 backdrop-blur-sm shadow-[var(--shadow-card)] overflow-hidden ring-1 ring-surface-900/5">
          {children}
        </section>
      </main>
    </div>
  );
};