import { useState, useCallback, useEffect, useRef } from 'react';

// ----------------------------------------------------------------------
// 1. Hook: 汎用リサイズロジック
// ----------------------------------------------------------------------

type ResizeDirection = 'left' | 'right';

interface UseResizableProps {
  /** 初期幅 (px) */
  defaultWidth?: number;
  /** 最小幅 (px) */
  minWidth?: number;
  /** 最大幅 (px) */
  maxWidth?: number;
  /** * リサイズ方向
   * 'left'  : 左サイドバー用（右にドラッグして広げる）
   * 'right' : 右サイドバー用（左にドラッグして広げる）
   */
  direction?: ResizeDirection;
  /** リサイズ終了時のコールバック（LocalStorage保存などに利用） */
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

  // ドラッグ開始時の値を保持するためのRef（再レンダリングをトリガーしない）
  const startX = useRef<number>(0);
  const startWidth = useRef<number>(0);

  /**
   * ドラッグ開始
   */
  const startResizing = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation(); // 親要素へのイベント伝播を防ぐ

    startX.current = e.clientX;
    startWidth.current = width;
    setIsResizing(true);
  }, [width]);

  /**
   * ドラッグ中（mousemove）
   */
  const handleMouseMove = useCallback((e: MouseEvent) => {
    // requestAnimationFrameでラップして描画負荷を軽減することも可能ですが、
    // 最近のブラウザでは直接計算でも十分高速です。
    
    // 移動量 (delta) を計算
    const currentX = e.clientX;
    const deltaX = currentX - startX.current;

    let newWidth = startWidth.current;

    if (direction === 'left') {
      // 左サイドバー: 右(+delta)に行くと幅が増える
      newWidth = startWidth.current + deltaX;
    } else {
      // 右サイドバー: 左(-delta)に行くと幅が増える、右に行くと減る
      newWidth = startWidth.current - deltaX;
    }

    // 最小・最大幅の制約を適用
    if (newWidth < minWidth) newWidth = minWidth;
    if (newWidth > maxWidth) newWidth = maxWidth;

    setWidth(newWidth);
  }, [direction, minWidth, maxWidth]);

  /**
   * ドラッグ終了（mouseup）
   */
  const handleMouseUp = useCallback(() => {
    setIsResizing(false);
    if (onResizeEnd) {
      onResizeEnd(width); // 最新のstateが参照できない可能性があるため注意が必要だが、Refを使えば確実
      // ここでは簡略化のため現在のstateに依存させない設計にするのが理想だが、
      // 実際にはuseEffect内のclosure問題があるため、Refでwidthも管理するか、
      // ユーザー体験的にはマウスアップ時のwidthは計算済みなので問題は起きにくい。
    }
  }, [onResizeEnd, width]);

  /**
   * Global Event Listeners の登録・解除
   */
  useEffect(() => {
    if (isResizing) {
      // windowに対してイベントを張ることで、iframe上や画面外に出ても追跡可能にする
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      
      // ドラッグ中のスタイル強制（テキスト選択防止、カーソル固定）
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
      // Iframeがある場合、マウスイベントを吸われるのを防ぐために pointer-events: none を当てる手法もあるが
      // ここでは簡易的に userSelect: none で対応
    }

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, handleMouseMove, handleMouseUp]);

  return {
    width,
    isResizing,
    startResizing,
  };
};

// ----------------------------------------------------------------------
// 2. Component: リサイズハンドル (UI)
// ----------------------------------------------------------------------

interface ResizeHandleProps {
  /** リサイズ開始関数 (Hookから返された startResizing を渡す) */
  onMouseDown: (e: React.MouseEvent) => void;
  /** 現在リサイズ中かどうか (スタイルの変化用) */
  isResizing?: boolean;
  /** ダブルクリックでリセットしたい場合などに利用 */
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
      // ロジック部分: ヒットエリアを確保し、マウスイベントを受け取る
      // touch-none: スマホでのスクロール誤爆防止
      className={`group flex-none relative z-50 h-full w-4 -ml-2 flex flex-col justify-center items-center cursor-col-resize select-none touch-none outline-none ${className}`}
      onMouseDown={onMouseDown}
      onDoubleClick={onDoubleClick}
      role="separator"
      aria-orientation="vertical"
      aria-valuenow={0} // 本当はwidthを入れるべきだが装飾的なので省略可
    >
      {/* ビジュアル部分: 実際の細い線 */}
      {/* 通常時: bg-transparent (見えない) または bg-slate-200
         ホバー時/ドラッグ時: 色を濃くする + 幅を太くするなどのインタラクション
      */}
      <div 
        className={`h-8 w-1 rounded-full transition-all duration-200 ease-in-out ${
          isResizing 
            ? 'bg-blue-500 scale-y-110' 
            : 'bg-slate-300 group-hover:bg-blue-400 group-hover:scale-y-110'
        }`} 
      />
      
      {/* アクセシビリティ: スクリーンリーダー用には見えないテキストやラベルが必要だが、
          UIツールキットとしては現状この構造で十分機能する */}
    </div>
  );
};