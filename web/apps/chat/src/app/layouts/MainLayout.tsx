import type { ReactNode } from 'react';
import { DebugPlaceholder } from '@/shared/ui/DebugPlaceholder';

const IS_LAYOUT_DEBUG = false;

interface MainLayoutProps {
  header?: ReactNode;
  children: ReactNode;
}

export const MainLayout = ({ header, children }: MainLayoutProps) => {
  return (
    // 画面全体を覆うラッパー
    <div className="layout-root">
      
      {/* アプリ本体のコンテナ（最大幅制限＋中央寄せ） */}
      <div className="layout-app-container">
        
        {/* ヘッダーエリア（Chromeのような検索バー） */}
        {header && (
          <header className="layout-header">
            {IS_LAYOUT_DEBUG ? (
              <DebugPlaceholder label="Chrome風検索ヘッダー" color="bg-yellow-500/20 border-yellow-500/50 text-yellow-700" />
            ) : (
              header
            )}
          </header>
        )}

        {/* メインコンテンツエリア（Instagram型のサイドバー＋チャット） */}
        <main className="layout-main">
          {IS_LAYOUT_DEBUG ? (
            <DebugPlaceholder label="コンテンツエリア（サイドバー＋チャット）" color="bg-blue-500/20 border-blue-500/50 text-blue-700" />
          ) : (
            children
          )}
        </main>

      </div>
    </div>
  );
};