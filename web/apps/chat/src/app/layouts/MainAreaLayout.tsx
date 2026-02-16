import type { ReactNode } from 'react';

interface MainAreaLayoutProps {
  left?: ReactNode;    // 左パネル
  right: ReactNode;    // 右パネル (必須)
  className?: string;
}

export const MainAreaLayout = ({ left, right, className = '' }: MainAreaLayoutProps) => {
  return (
    <div className={`layout-area ${className}`}>
      {/* 左パネルエリア (幅はCSS側で固定されます) */}
      {left && (
        <aside className="layout-area-left">
          {left}
        </aside>
      )}

      {/* 右パネルエリア
        - 残りの幅をすべて埋める (flex-1)
        - min-w-0 は必須 (これがないと中のTruncateやスクロールが壊れます)
      */}
      <main className="layout-area-right">
        {right}
      </main>
    </div>
  );
};