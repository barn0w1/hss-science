import type { ReactNode } from 'react';

interface PanelLayoutProps {
  /** * パネルのヘッダー部分。
   * タイトルや閉じるボタンなど、すべてのヘッダー要素をここに渡します。
   * スクロールは発生しません。
   */
  header?: ReactNode;
  
  /** * メインコンテンツ。
   * スクロール制御（overflow-y-autoなど）は、このchildrenの直下で行ってください。
   */
  children: ReactNode;
  
  /** 追加のスタイル */
  className?: string;
}

export const PanelLayout = ({ 
  header, 
  children, 
  className = '' 
}: PanelLayoutProps) => {
  return (
    <section className={`panel-card ${className}`}>
      {/* --- Header Area (Fixed & No Scroll) --- */}
      {header && (
        <header className="panel-header">
          {/* ヘッダーの中身は呼び出し側で制御します。
            例: <div className="flex w-full justify-between items-center">...</div>
          */}
          {header}
        </header>
      )}

      {/* --- Main Content Container --- */}
      <div className="panel-content">
        {children}
      </div>
    </section>
  );
};