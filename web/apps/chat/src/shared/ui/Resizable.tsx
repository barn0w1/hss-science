import { useState, useCallback, useEffect, useRef } from 'react';

// ----------------------------------------------------------------------
// 1. Hook: ロジック (レスポンシブ対応強化版)
// ----------------------------------------------------------------------

type ResizeDirection = 'left' | 'right';

interface UseResizableProps {
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  direction?: ResizeDirection; // 'left' = 左パネル用(右へドラッグで拡大), 'right' = 右パネル用(左へドラッグで拡大)
  onResizeEnd?: (finalWidth: number) => void;
}

export const useResizable = ({
  defaultWidth = 320,
  minWidth = 240,
  maxWidth = 600,
  direction = 'left',
  onResizeEnd,
}: UseResizableProps = {}) => {
  const [width, setWidth] = useState(defaultWidth);
  const [isResizing, setIsResizing] = useState(false);
  
  // 座標計算用のRef
  const startX = useRef<number>(0);
  const startWidth = useRef<number>(0);

  // ---------------------------------------------
  // ★追加 1: 外部制約 (maxWidth/minWidth) が変わったら強制的にリサイズする
  // ---------------------------------------------
  useEffect(() => {
    setWidth((prevWidth) => {
      // 現在の幅が新しい許容範囲を外れていたら補正する
      if (prevWidth > maxWidth) return maxWidth;
      if (prevWidth < minWidth) return minWidth;
      return prevWidth;
    });
  }, [minWidth, maxWidth]);

  // ---------------------------------------------
  // リサイズ開始処理 (Mouse & Touch)
  // ---------------------------------------------
  const startResizing = useCallback((e: React.MouseEvent | React.TouchEvent) => {
    // タッチデバイスでのスクロールなどを防ぐ
    // (ただしpassive event listenerの問題が出る場合はCSSの touch-action: none で対応する)
    
    // 座標取得 (マウス or タッチ)
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;

    startX.current = clientX;
    startWidth.current = width;
    setIsResizing(true);
  }, [width]);

  // ---------------------------------------------
  // リサイズ中処理 (Window Global Events)
  // ---------------------------------------------
  const handleMove = useCallback((clientX: number) => {
    const deltaX = clientX - startX.current;
    let newWidth = startWidth.current;

    // 左パネル用: 右に動かすと(deltaがプラス)、幅が増える
    // 右パネル用: 右に動かすと(deltaがプラス)、幅が減る
    if (direction === 'left') {
      newWidth = startWidth.current + deltaX;
    } else {
      newWidth = startWidth.current - deltaX;
    }

    // 制約チェック
    if (newWidth < minWidth) newWidth = minWidth;
    if (newWidth > maxWidth) newWidth = maxWidth;

    setWidth(newWidth);
  }, [direction, minWidth, maxWidth]);

  const onMouseMove = useCallback((e: MouseEvent) => handleMove(e.clientX), [handleMove]);
  const onTouchMove = useCallback((e: TouchEvent) => handleMove(e.touches[0].clientX), [handleMove]);

  // ---------------------------------------------
  // リサイズ終了処理
  // ---------------------------------------------
  const stopResizing = useCallback(() => {
    setIsResizing(false);
    if (onResizeEnd) onResizeEnd(width);
  }, [onResizeEnd, width]);

  // ---------------------------------------------
  // イベントリスナーの登録/解除
  // ---------------------------------------------
  useEffect(() => {
    if (isResizing) {
      // Mouse events
      window.addEventListener('mousemove', onMouseMove);
      window.addEventListener('mouseup', stopResizing);
      
      // ★追加 2: Touch events (iPad/Tablet対応)
      window.addEventListener('touchmove', onTouchMove);
      window.addEventListener('touchend', stopResizing);

      // UI調整
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none'; // テキスト選択防止
    }
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', stopResizing);
      window.removeEventListener('touchmove', onTouchMove);
      window.removeEventListener('touchend', stopResizing);

      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, onMouseMove, onTouchMove, stopResizing]);

  return { width, isResizing, startResizing };
};

// ----------------------------------------------------------------------
// 2. Component: リサイズハンドル (グリップのみ・超ミニマル版)
// ----------------------------------------------------------------------

interface ResizeHandleProps {
  onMouseDown: (e: React.MouseEvent | React.TouchEvent) => void; // 型定義をTouch対応に拡張
  isResizing?: boolean;
  onDoubleClick?: () => void;
  className?: string;
}

export const ResizeHandle = ({ 
  onMouseDown, 
  isResizing = false,
  onDoubleClick,
  className = '' 
}: ResizeHandleProps) => {
  return (
    <div
      className={`
        group 
        h-full w-full 
        flex flex-col justify-center items-center 
        cursor-col-resize select-none 
        touch-none
        outline-none 
        ${className}
      `}
      // MouseDown と TouchStart 両方に対応
      onMouseDown={onMouseDown}
      onTouchStart={onMouseDown}
      onDoubleClick={onDoubleClick}
    >
      {/* グリップ部分 */}
      <div 
        className={`
          h-12 w-1 rounded-full 
          transition-colors duration-200 ease-out
          ${isResizing 
            ? 'bg-slate-400' 
            : 'bg-slate-300 group-hover:bg-slate-400'
          }
        `} 
      />
    </div>
  );
};