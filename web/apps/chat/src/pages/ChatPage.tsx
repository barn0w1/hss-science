import { useState } from 'react';
import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';
import { PanelLayout } from '@/shared/ui/PanelLayout';
import { useResizable, ResizeHandle } from '@/shared/ui/Resizable';

export const ChatPage = () => {
  const { width, isResizing, startResizing } = useResizable({ defaultWidth: 320 });
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

  return (
    <MainLayout
      header={
        <div className="flex items-center justify-between px-4 w-full">
          <span className="font-bold text-lg text-slate-700">App Title</span>
          <button onClick={() => setIsSidebarOpen(!isSidebarOpen)} className="text-sm text-slate-500 hover:text-slate-800">
            {isSidebarOpen ? 'Close Sidebar' : 'Open Sidebar'}
          </button>
        </div>
      }
    >
      <MainAreaLayout
        /* --- Left Panel --- */
        left={isSidebarOpen && (
          <div style={{ width }} className="h-full flex-none transition-[width] duration-0">
            <PanelLayout header={<span className="font-semibold text-slate-700">Source</span>}>
              {/* Scrollable Content Area */}
              <div className="h-full w-full overflow-y-auto p-4">
                {Array.from({ length: 20 }).map((_, i) => (
                  <div key={i} className="mb-4 p-4 border rounded bg-slate-50 text-xs text-slate-400">
                    Source Item {i + 1}
                  </div>
                ))}
              </div>
            </PanelLayout>
          </div>
        )}

        /* --- Resizer --- */
        resizer={isSidebarOpen && (
          <ResizeHandle onMouseDown={startResizing} isResizing={isResizing} />
        )}

        /* --- Right Panel --- */
        right={
          <PanelLayout header={<span className="font-semibold text-slate-700">Chat</span>}>
            {/* Scrollable Content Area */}
            <div className="h-full w-full overflow-y-auto p-8 bg-white">
              <div className="max-w-3xl mx-auto space-y-6">
                {Array.from({ length: 10 }).map((_, i) => (
                  <div key={i} className={`flex ${i % 2 === 0 ? 'justify-end' : 'justify-start'}`}>
                    <div className={`p-4 rounded-2xl max-w-[80%] ${i % 2 === 0 ? 'bg-slate-100' : 'bg-blue-50'}`}>
                      <p className="text-sm text-slate-700">Message content simulation {i + 1}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </PanelLayout>
        }
      />
    </MainLayout>
  );
};