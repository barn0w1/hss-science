import { useState, useCallback, useEffect, useRef } from 'react';

// ----------------------------------------------------------------------
// 1. Hook: ロジック (変更なし)
// ----------------------------------------------------------------------

type ResizeDirection = 'left' | 'right';

interface UseResizableProps {
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  direction?: ResizeDirection;
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
  const startX = useRef<number>(0);
  const startWidth = useRef<number>(0);

  const startResizing = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    startX.current = e.clientX;
    startWidth.current = width;
    setIsResizing(true);
  }, [width]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    const currentX = e.clientX;
    const deltaX = currentX - startX.current;
    let newWidth = startWidth.current;

    if (direction === 'left') {
      newWidth = startWidth.current + deltaX;
    } else {
      newWidth = startWidth.current - deltaX;
    }

    if (newWidth < minWidth) newWidth = minWidth;
    if (newWidth > maxWidth) newWidth = maxWidth;

    setWidth(newWidth);
  }, [direction, minWidth, maxWidth]);

  const handleMouseUp = useCallback(() => {
    setIsResizing(false);
    if (onResizeEnd) onResizeEnd(width);
  }, [onResizeEnd, width]);

  useEffect(() => {
    if (isResizing) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    }
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, handleMouseMove, handleMouseUp]);

  return { width, isResizing, startResizing };
};

// ----------------------------------------------------------------------
// 2. Component: リサイズハンドル (グリップのみ・超ミニマル版)
// ----------------------------------------------------------------------

interface ResizeHandleProps {
  onMouseDown: (e: React.MouseEvent) => void;
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
        cursor-col-resize select-none touch-none 
        outline-none 
        ${className}
      `}
      onMouseDown={onMouseDown}
      onDoubleClick={onDoubleClick}
    >
      {/* グリップ部分
         - shadow-sm を削除
         - 色だけ（グレーの濃淡）で状態を表現
      */}
      <div 
        className={`
          h-12 w-1.5 rounded-full 
          transition-colors duration-200 ease-out
          ${isResizing 
            ? 'bg-slate-400' // ドラッグ中: 濃いグレー (影なし)
            : 'bg-slate-300 group-hover:bg-slate-400' // 通常: 薄いグレー
          }
        `} 
      />
    </div>
  );
};