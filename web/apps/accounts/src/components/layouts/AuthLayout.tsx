import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, title, subtitle }) => {
  return (
    <div className="min-h-screen w-full flex flex-col text-slate-900">
      <main className="flex-grow flex items-center justify-center px-6 py-16">
        <div className="w-full max-w-[440px]">
          <div className="flex flex-col items-center mb-6">
            <div className="h-10 w-10 rounded-full bg-slate-900 text-white flex items-center justify-center text-sm font-semibold">
              H
            </div>
            <div className="mt-3 text-center">
              <p className="text-sm font-semibold text-slate-900 tracking-tight">HSS Science</p>
            </div>
          </div>

          <div className="rounded-2xl bg-white/92 backdrop-blur-[2px] p-8 shadow-[0_28px_70px_rgba(15,23,42,0.08),0_2px_6px_rgba(15,23,42,0.04)]">
            <div className="mb-8">
              {title && (
                <h1 className="text-2xl font-semibold text-slate-900 mb-2">
                  {title}
                </h1>
              )}
              {subtitle && (
                <p className="text-sm text-slate-500">
                  {subtitle}
                </p>
              )}
            </div>

            <div className="w-full">
              {children}
            </div>
          </div>
        </div>
      </main>

    </div>
  );
};
