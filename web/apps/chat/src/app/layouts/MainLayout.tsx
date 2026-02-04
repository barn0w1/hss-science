import { ReactNode } from 'react';

interface MainLayoutProps {
  sidebar: ReactNode; // 左側のサイドバー (ルーム一覧など)
  children: ReactNode; // 右側のメインコンテンツ (チャット画面)
}

export const MainLayout = ({ sidebar, children }: MainLayoutProps) => {
  return (
    // ■ 全体枠
    // h-screen: 画面いっぱいの高さ
    // overflow-hidden: 大外のスクロールを禁止
    <div className="flex h-screen w-full overflow-hidden bg-surface-50 text-surface-900">
      
      {/* ■ サイドバーエリア (Desktop用) */}
      {/* hidden md:flex: モバイルでは隠し、PC(md以上)で表示 */}
      {/* w-80: 幅320px固定 */}
      <aside className="hidden md:flex w-80 flex-col border-r border-surface-200 bg-surface-100 flex-shrink-0">
        {sidebar}
      </aside>

      {/* ■ メインエリア */}
      {/* flex-1: 残りの幅を全部使う */}
      {/* min-w-0: Flexアイテムが中身に合わせて肥大化するのを防ぐ(重要) */}
      <main className="flex-1 flex flex-col relative min-w-0">
        {children}
      </main>
      
    </div>
  );
};