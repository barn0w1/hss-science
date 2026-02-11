import type { ReactNode } from 'react';

interface MainAreaLayoutProps {
  left?: ReactNode;    // 左パネル
  right: ReactNode;    // 右パネル (必須)
  resizer?: ReactNode; // リサイズハンドル
  className?: string;
}

export const MainAreaLayout = ({ left, right, resizer, className = '' }: MainAreaLayoutProps) => {
  return (
    <div className={`layout-area ${className}`}>
      {/* 左パネルエリア */}
      {/* widthは子要素(div style={{ width }})側で制御されることを想定 */}
      {left && (
        <aside className="layout-area-left">
          {left}
        </aside>
      )}

      {/* リサイザーエリア 
        - 左パネルが存在するときだけ表示
        - これ自体が「Left」と「Right」の間のマージン（隙間）の役割も果たします
      */}
      {left && resizer && (
        <div className="layout-area-resizer">
          {resizer}
        </div>
      )}

      {/* 右パネルエリア
        - 残りの幅をすべて埋める (flex-1)
        - min-w-0 は重要 (これがないと中のTruncateやスクロールが壊れる)
      */}
      <main className="layout-area-right">
        {right}
      </main>
    </div>
  );
};