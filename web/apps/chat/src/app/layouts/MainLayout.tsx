import type { ReactNode } from 'react';

interface MainLayoutProps {
  header?: ReactNode; 
  sidebar: ReactNode; 
  children: ReactNode;
}

export const MainLayout = ({ header, sidebar, children }: MainLayoutProps) => {
  return (
    // ■ 全体の背景:
    // ここを少し濃いめの色（surface-200など）にすると、
    // 上に乗る要素の「白さ」や「輝き」がより際立ちます。
    <div className="flex h-screen w-full overflow-hidden bg-surface-200/50 text-surface-900">
      
      {/* 
        ■ サイドバーエリア:
        【改善点】
        1. 背景色を削除（透明に）。親の背景をそのまま見せることで「箱感」を消す。
        2. `p-4` (内側の余白) を追加。
           これで中身（リスト）が画面端にへばりつかず、
           メインエリアと同じような「浮遊感」の予兆を作ります。
      */}
      <aside className="w-80 flex-shrink-0 flex flex-col p-4">
        {/* 
          この中に配置されるリストアイテム（部屋一覧など）に
          `hover:bg-surface-200/50 rounded-lg` などを適用すると、
          文字だけが空間に浮いているような、モダンな見た目になります。
        */}
        {sidebar}
      </aside>

      {/* 
        ■ メインエリア:
        【改善点】
        margin (m-4) を削除し、親の flex gap や padding で制御しても良いですが、
        ここでは「右側と上下に余白を持つ」形に整えます。
        
        my-4 mr-4: 上・下・右に余白。左はサイドバーとの距離感で調整。
      */}
      <main className="flex-1 flex flex-col min-w-0 my-4 mr-4 rounded-3xl bg-white shadow-xl shadow-surface-300/20 overflow-hidden ring-1 ring-surface-900/5">
        
        {header && (
          // ヘッダーの高さを少し広げ(h-18)、ゆったりさせるのも高級感を出すコツ
          <header className="h-[72px] flex items-center px-8 border-b border-surface-100 bg-white/80 backdrop-blur-md z-10">
            {header}
          </header>
        )}

        <section className="flex-1 relative overflow-hidden bg-white">
          {children}
        </section>
      </main>
    </div>
  );
};