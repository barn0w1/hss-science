import { useState, useEffect } from 'react';
import { 
  PanelLeftClose, 
  PanelLeftOpen, 
  Search, 
  MoreHorizontal,
  ChevronDown
} from 'lucide-react';

// components
import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';
import { PanelLayout } from '@/shared/ui/PanelLayout';
import { PanelHeader } from '@/shared/ui/PanelHeader';
import { useResizable, ResizeHandle } from '@/shared/ui/Resizable';

// ----------------------------------------------------------------------
// Helper Hook: 画面サイズを取得 (レスポンシブ計算用)
// ----------------------------------------------------------------------
const useWindowSize = () => {
  const [windowSize, setWindowSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    // クライアントサイドでのみ実行
    const handleResize = () => {
      setWindowSize({
        width: window.innerWidth,
        height: window.innerHeight,
      });
    };
    
    // 初期値設定
    handleResize();

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  return windowSize;
};

// ----------------------------------------------------------------------
// Main Component
// ----------------------------------------------------------------------

export const ChatPage = () => {
  // 1. 画面幅を取得
  const { width: windowWidth } = useWindowSize();

  // 2. 開閉状態管理
  const [isLeftOpen, setIsLeftOpen] = useState(true);

  // 3. レスポンシブな制約設定 (ここがポイント)
  //    - minWidth: テキストが崩れない最低ライン (200px)
  //    - maxWidth: 画面幅の 40% までしか広がらないように制限 (Desktopなら広く、Laptopなら狭く)
  //    - defaultWidth: 画面幅の 20% くらいを初期値に
  
  // windowWidthが0(初期ロード時)の場合は安全策で固定値を入れる
  const calculatedMaxWidth = windowWidth ? windowWidth * 0.4 : 400; 
  const calculatedDefaultWidth = windowWidth ? windowWidth * 0.2 : 260;

  // 4. リサイズ管理
  const { width, isResizing, startResizing } = useResizable({
    defaultWidth: calculatedDefaultWidth,
    minWidth: 200, 
    maxWidth: calculatedMaxWidth, // 動的に変わる
  });

  // 5. 実際の表示幅 (閉じているときはボタン幅の 56px 固定)
  const sidebarWidth = isLeftOpen ? width : 56;

  return (
    <MainLayout
      header={
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center text-white font-bold">
            A
          </div>
          <span className="font-bold text-lg text-slate-700">App Name</span>
        </div>
      }
    >
      <MainAreaLayout
        // --- 左パネル ---
        left={
          <div 
            style={{ width: sidebarWidth }}
            className="h-full flex-none" 
          >
            <PanelLayout
              header={
                isLeftOpen ? (
                  <PanelHeader
                    left={
                      <button className="flex items-center gap-1 font-bold text-slate-700 hover:bg-slate-100 px-2 py-1 rounded -ml-2 transition-colors truncate">
                        <span>My Workspace</span>
                        <ChevronDown size={14} className="opacity-50" />
                      </button>
                    }
                    right={
                      <>
                        <button className="p-1.5 hover:bg-slate-100 rounded text-slate-500">
                          <Search size={18} />
                        </button>
                        <button 
                          onClick={() => setIsLeftOpen(false)}
                          className="p-1.5 hover:bg-slate-100 rounded text-slate-500"
                        >
                          <PanelLeftClose size={18} />
                        </button>
                      </>
                    }
                  />
                ) : (
                  <div className="w-full flex justify-center">
                    <button 
                      onClick={() => setIsLeftOpen(true)}
                      className="p-1.5 hover:bg-slate-100 rounded text-slate-500"
                    >
                      <PanelLeftOpen size={18} />
                    </button>
                  </div>
                )
              }
            >
              {isLeftOpen ? (
                <div className="p-2 space-y-1">
                  <div className="p-2 bg-slate-50 rounded text-sm font-medium">Channel A</div>
                  <div className="p-2 hover:bg-slate-50 rounded text-sm">Channel B</div>
                  <div className="p-2 hover:bg-slate-50 rounded text-sm">Direct Messages</div>
                </div>
              ) : (
                <div className="flex flex-col items-center pt-4 gap-4">
                  {/* Closed State Content */}
                </div>
              )}
            </PanelLayout>
          </div>
        }

        // --- リサイザー ---
        resizer={
          // 開いている時: 操作可能なハンドルを表示
          // 閉じている時: 操作不可だが、余白(Gap)として16pxの透明なdivを残す
          isLeftOpen ? (
            <ResizeHandle 
              onMouseDown={startResizing} 
              isResizing={isResizing} 
            />
          ) : (
            <div className="w-full h-full" />
          )
        }

        // --- 右パネル ---
        right={
          <PanelLayout
            header={
              <PanelHeader
                left={<span className="font-bold text-slate-700 truncate"># Channel A</span>}
                right={
                  <button className="p-1.5 hover:bg-slate-100 rounded text-slate-500">
                    <MoreHorizontal size={18} />
                  </button>
                }
              />
            }
          >
            <div className="p-8 text-slate-400 text-center">
              Chat Content
            </div>
          </PanelLayout>
        }
      />
    </MainLayout>
  );
};