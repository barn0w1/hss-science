import type { ReactNode } from 'react';

interface PanelHeaderProps {
  /** 左寄せエリア (タイトル、プルダウンメニューなど) */
  left?: ReactNode;
  /** 右寄せエリア (閉じるボタン、アクションアイコンなど) */
  right?: ReactNode;
  className?: string;
}

export const PanelHeader = ({ left, right, className = '' }: PanelHeaderProps) => {
  return (
    <div className={`flex w-full h-full items-center justify-between gap-3 ${className}`}>
      
      {/* [LEFT Area] 
        - flex-1: 空いているスペースをすべて埋めるようにします。
        - min-w-0: これがないと、中身のテキストが長い時にパネルを突き破ってしまいます（Flexboxの仕様）。
                   これを指定することで、親の幅に合わせてテキストが '...' になることを許可します。
      */}
      <div className="flex-1 flex items-center gap-2 min-w-0 text-slate-700">
        {left}
      </div>

      {/* [RIGHT Area]
        - flex-none: どんなに画面が狭くなっても、ボタン類は絶対に縮ませない・隠さない。
      */}
      <div className="flex-none flex items-center gap-1 text-slate-400">
        {right}
      </div>
      
    </div>
  );
};