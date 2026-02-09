import type { ReactNode } from 'react';

interface MainAreaLayoutProps {
  left?: ReactNode;    // 左パネル (コンテンツ + 幅指定は親から)
  right: ReactNode;    // 右パネル
  resizer?: ReactNode; // リサイズハンドル等のためのスロット
  className?: string;
}

export const MainAreaLayout = ({ left, right, resizer, className = '' }: MainAreaLayoutProps) => {
  return (
    <div className={`layout-area ${className}`}>
      {/* 左パネル */}
      {left && (
        <aside className="layout-area-left">
          {left}
        </aside>
      )}

      {/* リサイズハンドル用スロット */}
      {left && resizer && (
        <div className="layout-area-resizer">
          {resizer}
        </div>
      )}

      {/* 右パネル */}
      <main className="layout-area-right">
        {right}
      </main>
    </div>
  );
};