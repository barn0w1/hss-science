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
          <div className="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between">
            <span>Direct Messages</span>
            <span className="bg-surface-100 text-surface-500 px-1.5 rounded text-[10px]">{dms.length}</span>
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

      {isSidebarOpen && <div className="mx-3 my-2 border-t border-surface-200/50 flex-shrink-0" />}

      {/* Spaceエリア */}
      <div className="flex-1 flex flex-col min-h-0">
        {isSidebarOpen && (
          <div className="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between">
            <span>Spaces</span>
            <button className="text-surface-400 hover:text-surface-700">+</button>
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
        <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-7 bg-surface-700 rounded-r-full" />
      )}

      <div className="relative flex-shrink-0">
        {item.iconUrl ? (
          <img src={item.iconUrl} alt="" className="w-8 h-8 rounded-lg object-cover bg-surface-200" />
        ) : (
          <div className={`
            w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold
            ${item.isActive ? 'bg-surface-200 text-surface-800' : 'bg-surface-200 text-surface-500'}
          `}>
            {item.fallbackInitial}
          </div>
        )}
        
        {/* 閉じた時のバッジ */}
        {!isSidebarOpen && item.unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-surface-800 text-[10px] text-white ring-2 ring-white">
            {item.unreadCount}
          </span>
        )}
      </div>

      {isSidebarOpen && (
        <>
          <div className="ml-3 flex-1 text-left min-w-0">
            <div className={`text-sm font-medium truncate ${item.unreadCount > 0 ? 'text-surface-900 font-semibold' : ''}`}>
              {item.displayName}
            </div>
          </div>
          <div className="flex items-center gap-1">
            {item.isPinned && <span className="text-[10px] text-surface-400">•</span>}
            {item.unreadCount > 0 && (
              <span className="min-w-[18px] h-[18px] px-1 flex items-center justify-center rounded-full bg-surface-800 text-[10px] font-bold text-white">
                {item.unreadCount}
              </span>
            )}
          </div>
        </>
      )}
      
      {/* 閉じた時のツールチップ */}
      {!isSidebarOpen && (
        <div className="absolute left-full ml-2 px-2 py-1 bg-surface-800 text-white text-xs rounded opacity-0 group-hover:opacity-100 pointer-events-none whitespace-nowrap z-50">
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
  <div className="flex flex-col h-full gap-4 pt-4 px-2 animate-pulse">
    <div className="space-y-2">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className={`rounded-[var(--layout-sidebar-item-radius)] bg-[var(--layout-sidebar-skeleton)] ${
            isSidebarOpen ? 'w-full' : 'w-10'
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
            isSidebarOpen ? 'w-full' : 'w-10'
          }`}
          style={{ height: 'var(--layout-sidebar-item-height)' }}
        />
      ))}
    </div>
  </div>
);