import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, title, subtitle }) => {
  return (
    <div className="min-h-screen flex flex-col bg-white relative overflow-hidden font-sans">
      
      {/* Background Gradients - Backblaze style simulation */}
      <div className="absolute inset-0 z-0 pointer-events-none overflow-hidden">
        {/* White base */}
        <div className="absolute inset-0 bg-white" />
        
        {/* Colorful Mesh Gradient on the Right */}
        <div 
          className="absolute top-[-20%] right-[-10%] w-[80%] h-[120%] opacity-80 blur-[80px]"
          style={{
            background: 'radial-gradient(circle at center, rgba(235, 60, 0, 0.15) 0%, rgba(130, 40, 200, 0.15) 40%, rgba(50, 100, 250, 0.05) 70%, transparent 100%)'
          }}
        />
         {/* Second subtle blob */}
         <div 
          className="absolute bottom-[-10%] right-[10%] w-[50%] h-[50%] opacity-60 blur-[100px]"
          style={{
            background: 'radial-gradient(circle at center, rgba(255, 100, 100, 0.2) 0%, transparent 70%)'
          }}
        />
      </div>

      {/* Header */}
      <header className="relative z-10 w-full px-6 py-6 md:px-12 flex items-center justify-between">
         <div className="flex items-center gap-2">
            {/* Logo */}
            <div className="flex items-center gap-2">
                 <div className="h-8 w-8 bg-red-600 rounded flex items-center justify-center text-white font-bold text-lg">
                    H
                 </div>
                 <span className="text-xl font-bold text-gray-900 tracking-tight">HSS Science</span>
            </div>
         </div>
      </header>

      {/* Main Content */}
      <main className="relative z-10 flex-grow flex items-center justify-start md:pl-[15%] p-6">
        <div className="w-full max-w-[440px] bg-white lg:bg-transparent rounded-2xl lg:rounded-none shadow-xl lg:shadow-none p-8 lg:p-0 border border-gray-100 lg:border-none">
             
             <div className="mb-8">
                {title && (
                    <h1 className="text-3xl font-bold text-gray-900 mb-2 tracking-tight">
                        {title}
                    </h1>
                )}
                {subtitle && (
                    <p className="text-gray-600 text-base">
                        {subtitle}
                    </p>
                )}
             </div>

            <div className="space-y-6">
                {children}
            </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="relative z-10 w-full py-6 px-6 md:px-12">
        <div className="flex flex-col md:flex-row justify-between items-center gap-4 text-sm text-gray-500">
             <div className="flex gap-6 font-medium">
                 <span className="font-bold text-gray-900">HSS Science</span>
                 <span>A Scientific Community</span>
             </div>
             
             <div className="flex gap-8">
                <a href="#" className="hover:text-gray-900 transition-colors">Company</a>
                <a href="#" className="hover:text-gray-900 transition-colors">Contact</a>
                <a href="#" className="hover:text-gray-900 transition-colors">Privacy</a>
                <a href="#" className="hover:text-gray-900 transition-colors">Terms</a>
             </div>
        </div>
      </footer>
    </div>
  );
};
