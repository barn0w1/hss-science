import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children, title, subtitle }) => {
  return (
    <div>
      <header>
        <img src="/icon.svg" alt="HSS Science" />
        <span>HSS Science</span>
      </header>

      <main>
        <section>
          {title && (
            <h1>{title}</h1>
          )}
          {subtitle && (
            <p>{subtitle}</p>
          )}
          <div>
            {children}
          </div>
        </section>
      </main>
    </div>
  );
};
