import { useEffect } from 'react';
import { useChatStore } from '../state';
import { useSidebarData, type SidebarItemData } from '../hooks/useSidebarData';

interface ChatSidebarProps {
  isSidebarOpen: boolean;
}

export const ChatSidebar = ({ isSidebarOpen }: ChatSidebarProps) => {
  const fetchRooms = useChatStore((state) => state.fetchRooms);
  const setActiveRoom = useChatStore((state) => state.setActiveRoom);
  
  // Hookから「表示するだけ」の状態になったデータを受け取る
  const { spaces, dms, isLoading } = useSidebarData();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  if (isLoading) return <SidebarSkeleton isSidebarOpen={isSidebarOpen} />;

  return (
    <div className="flex flex-col h-full py-4">
      {/* DMエリア */}
      <div className="flex-shrink-0 max-h-[40%] flex flex-col min-h-0">
        {isSidebarOpen && (
          <div className="px-2 py-1 text-[9px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between">
            <span>Direct Messages</span>
          </div>
        )}
        <div className="overflow-y-auto px-2" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
          {dms.map((item) => (
            <SidebarItem
              key={item.id}
              item={item}
              isSidebarOpen={isSidebarOpen}
              onClick={() => setActiveRoom(item.id)}
            />
          ))}
        </div>
      </div>

      {isSidebarOpen && <div className="mx-2 my-2 border-t border-surface-200/50 flex-shrink-0" />}

      {/* Spaceエリア */}
      <div className="flex-1 flex flex-col min-h-0">
        {isSidebarOpen && (
          <div className="px-2 py-1 text-[9px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between">
            <span>Spaces</span>
          </div>
        )}
        <div className="overflow-y-auto px-2" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
          {spaces.map((item) => (
            <SidebarItem
              key={item.id}
              item={item}
              isSidebarOpen={isSidebarOpen}
              onClick={() => setActiveRoom(item.id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
};

// ------------------------------------------------------------------
// SidebarItem Component
// ロジックがなくなり、Propsを表示するだけの「Pure Component」になった
// ------------------------------------------------------------------
interface SidebarItemProps {
  item: SidebarItemData; // Hookで作ったViewModelを受け取る
  isSidebarOpen: boolean;
  onClick: () => void;
}

const SidebarItem = ({ item, isSidebarOpen, onClick }: SidebarItemProps) => {
  return (
    <button
      onClick={onClick}
      className={`relative group w-full flex items-center outline-none rounded-[var(--layout-sidebar-item-radius)] ${
        isSidebarOpen ? 'px-3' : 'justify-center px-0'
      } ${
        item.isActive
          ? 'bg-[var(--layout-sidebar-bg-active)] text-[var(--layout-sidebar-text-active)]'
          : 'text-[var(--layout-sidebar-text)] hover:bg-[var(--layout-sidebar-bg-hover)]'
      }`}
      style={{ height: 'var(--layout-sidebar-item-height)' }}
    >
      {!isSidebarOpen && item.isActive && (
        <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-4 bg-[var(--layout-sidebar-accent)] rounded-r-full" />
      )}

      <div className="relative flex-shrink-0">
        {item.iconUrl ? (
          <img src={item.iconUrl} alt="" className="w-6 h-6 rounded-md object-cover bg-surface-200" />
        ) : (
          <div className={`
            w-6 h-6 rounded-md flex items-center justify-center text-[10px] font-semibold
            ${item.isActive ? 'bg-surface-200 text-surface-800' : 'bg-surface-200 text-surface-500'}
          `}>
            {item.fallbackInitial}
          </div>
        )}
        
        {/* 閉じた時のバッジ */}
        {!isSidebarOpen && item.unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 h-2 w-2 rounded-full bg-[var(--layout-sidebar-accent)] ring-2 ring-white" />
        )}
        {!isSidebarOpen && item.unreadCount === 0 && item.isPinned && (
          <span className="absolute -top-0.5 -right-0.5 h-2 w-2 rounded-full bg-surface-300 ring-2 ring-white" />
        )}
      </div>

      {isSidebarOpen && (
        <>
          <div className="ml-3 flex-1 text-left min-w-0">
            <div className={`text-xs font-medium truncate ${item.unreadCount > 0 ? 'text-surface-900 font-semibold' : ''}`}>
              {item.displayName}
            </div>
          </div>
          <div className="flex items-center gap-1">
            {item.unreadCount > 0 ? (
              <span className="h-2 w-2 rounded-full bg-[var(--layout-sidebar-accent)]" />
            ) : item.isPinned ? (
              <svg viewBox="0 0 16 16" className="h-2.5 w-2.5 text-surface-400" aria-hidden="true">
                <path
                  d="M10 2l4 4-2 2-4-4-3 3-1 5-1-1 5-1 3-3-4-4 3-3z"
                  fill="currentColor"
                />
              </svg>
            ) : null}
          </div>
        </>
      )}
      
      {/* 閉じた時のツールチップ */}
      {!isSidebarOpen && (
        <div className="absolute left-full ml-2 px-2 py-1 bg-surface-800 text-white text-[10px] rounded opacity-0 group-hover:opacity-100 pointer-events-none whitespace-nowrap z-50">
          {item.displayName}
        </div>
      )}
    </button>
  );
};

// ------------------------------------------------------------------
// サブコンポーネント: スケルトン
// ------------------------------------------------------------------
const SidebarSkeleton = ({ isSidebarOpen }: { isSidebarOpen: boolean }) => (
  <div className="flex flex-col h-full gap-3 pt-3 px-2 animate-pulse">
    <div className="space-y-2">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className={`rounded-[var(--layout-sidebar-item-radius)] bg-[var(--layout-sidebar-skeleton)] ${
            isSidebarOpen ? 'w-full' : 'w-8'
          }`}
          style={{ height: 'var(--layout-sidebar-item-height)' }}
        />
      ))}
    </div>

    <div className="flex-1 space-y-2 pt-4 border-t border-surface-200">
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          key={i}
          className={`rounded-[var(--layout-sidebar-item-radius)] bg-[var(--layout-sidebar-skeleton)] ${
            isSidebarOpen ? 'w-full' : 'w-8'
          }`}
          style={{ height: 'var(--layout-sidebar-item-height)' }}
        />
      ))}
    </div>
  </div>
);