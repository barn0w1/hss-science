import type { ReactNode } from 'react';

interface PanelLayoutProps {
  /** ヘッダー部分 (ボタン、タイトル、検索窓なんでもOK) */
  header?: ReactNode;
  /** コンテンツ部分 */
  children: ReactNode;
  className?: string;
}

export const PanelLayout = ({ header, children, className = '' }: PanelLayoutProps) => {
  return (
    <section className={`panel-card ${className}`}>
      {/* ヘッダーエリア: 高さ固定・ボーダーあり */}
      {header && (
        <header className="panel-header">
          {header}
        </header>
      )}

      {/* コンテンツエリア: 残りの高さを埋める */}
      <div className="panel-content">
        {children}
      </div>
    </section>
  );
};