import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, title, subtitle }) => {
  return (
    <div className="min-h-screen w-full flex flex-col bg-white text-slate-900">
      <header className="w-full px-6 py-6 md:px-12 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-full bg-slate-900 text-white flex items-center justify-center text-sm font-semibold">
            H
          </div>
          <div className="flex flex-col leading-tight">
            <span className="text-lg font-semibold text-slate-900">HSS Science</span>
            <span className="text-xs text-slate-500">Identity & Access</span>
          </div>
        </div>
        <div className="text-xs text-slate-500 hidden sm:flex items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-full border border-slate-200 px-2 py-1">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
            Secure SSO
          </span>
        </div>
      </header>

      <main className="flex-grow flex items-center justify-center px-6 py-10">
        <div className="w-full max-w-[440px]">
          <div className="rounded-2xl border border-slate-200 bg-white p-8 shadow-[0_8px_30px_rgba(15,23,42,0.06)]">
            <div className="mb-8">
              {title && (
                <h1 className="text-2xl font-semibold text-slate-900 mb-2">
                  {title}
                </h1>
              )}
              {subtitle && (
                <p className="text-sm text-slate-600">
                  {subtitle}
                </p>
              )}
            </div>

            <div className="w-full">
              {children}
            </div>

            <div className="mt-6 text-xs text-slate-500 flex items-center gap-2">
              <span className="inline-flex items-center gap-1">
                <svg className="h-4 w-4 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M12 2l7 4v5c0 5-3.5 9-7 11-3.5-2-7-6-7-11V6l7-4z" />
                  <path d="M9 12l2 2 4-4" />
                </svg>
                Protected by HSS Identity Platform
              </span>
            </div>
          </div>
        </div>
      </main>

      <footer className="w-full px-6 py-6 md:px-12 text-xs text-slate-500">
        <div className="flex items-center justify-between">
          <span>HSS Science SSO</span>
          <span>© {new Date().getFullYear()}</span>
        </div>
      </footer>
    </div>
  );
};
