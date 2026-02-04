import type { ReactNode } from 'react';

interface MainLayoutProps {
  header?: ReactNode; 
  sidebar: ReactNode; 
  children: ReactNode;
}

export const MainLayout = ({ header, sidebar, children }: MainLayoutProps) => {
  return (
    <div className="flex h-screen w-full overflow-hidden bg-surface-50 text-surface-900">
      
      {/* 
        1. 左側: サイドバー
        ここは変わらず、背景に溶け込ませます。
      */}
      <aside className="flex-shrink-0 flex flex-col px-5 py-6 w-[var(--sidebar-width)]">
        {sidebar}
      </aside>

      {/* 
        2. 右側: コンテンツエリアのラッパー
        ここを縦(flex-col)に並べることで、「Header」と「Card」を分離します。
      */}
      <main className="flex-1 flex flex-col min-w-0">
        
        {/* 
          A. Header (カードの外側)
          白い背景を持たせず、グレーの背景の上に直接文字を置きます。
          これにより「部屋の看板」のような役割になります。
        */}
        {header && (
          <header className="h-[var(--spacing-header)] flex items-center px-8 flex-shrink-0">
            {/* 
              必要であれば、ここで文字色やフォントサイズを調整して
              「タイトルっぽさ」を出します。
            */}
            {header}
          </header>
        )}

        {/* 
          B. Main Content (浮遊するカード)
          Headerの下に配置。
          mr-4 (右) mb-4 (下) の余白を入れることで、
          画面の右下に独立して浮いているように見せます。
        */}
        <section className="flex-1 flex flex-col min-w-0 mr-[var(--spacing-gutter)] mb-[var(--spacing-gutter)] rounded-[var(--radius-card)] bg-white/90 backdrop-blur-sm shadow-[var(--shadow-card)] overflow-hidden ring-1 ring-surface-900/5">
          {children}
        </section>

      </main>
    </div>
  );
};