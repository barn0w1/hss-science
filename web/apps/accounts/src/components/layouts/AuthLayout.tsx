import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, title, subtitle }) => {
  return (
    <div className="min-h-screen w-full flex flex-col bg-white text-slate-900 relative overflow-hidden">
      <div className="absolute inset-0 -z-10">
        <div className="absolute inset-0 bg-white" />
        <div className="absolute right-[-25%] top-[-15%] h-[85vh] w-[85vh] rounded-full blur-[170px] opacity-90" style={{ background: 'radial-gradient(circle, rgba(124, 58, 237, 0.55) 0%, rgba(124, 58, 237, 0) 70%)' }} />
        <div className="absolute right-[-12%] top-[18%] h-[90vh] w-[90vh] rounded-full blur-[190px] opacity-80" style={{ background: 'radial-gradient(circle, rgba(244, 63, 94, 0.55) 0%, rgba(244, 63, 94, 0) 70%)' }} />
        <div className="absolute right-[5%] bottom-[-25%] h-[95vh] w-[95vh] rounded-full blur-[200px] opacity-80" style={{ background: 'radial-gradient(circle, rgba(59, 130, 246, 0.4) 0%, rgba(59, 130, 246, 0) 70%)' }} />
        <div className="absolute inset-y-0 left-0 w-[60%]" style={{ background: 'linear-gradient(to right, rgba(255,255,255,1) 58%, rgba(255,255,255,0) 100%)' }} />
      </div>

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

          <div className="rounded-2xl bg-white/90 backdrop-blur-[2px] p-8 shadow-[0_28px_70px_rgba(15,23,42,0.08),0_2px_6px_rgba(15,23,42,0.04)]">
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
