import { useEffect } from 'react';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
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

  return (
    <SidebarLayout
      isSidebarOpen={isSidebarOpen}
      dms={
        isLoading ? (
          <SidebarSkeletonList isSidebarOpen={isSidebarOpen} count={6} />
        ) : (
          dms.map((item) => (
            <SidebarItem
              key={item.id}
              item={item}
              isSidebarOpen={isSidebarOpen}
              onClick={() => setActiveRoom(item.id)}
            />
          ))
        )
      }
      spaces={
        isLoading ? (
          <SidebarSkeletonList isSidebarOpen={isSidebarOpen} count={6} />
        ) : (
          spaces.map((item) => (
            <SidebarItem
              key={item.id}
              item={item}
              isSidebarOpen={isSidebarOpen}
              onClick={() => setActiveRoom(item.id)}
            />
          ))
        )
      }
    />
  );
};

// ------------------------------------------------------------------
// Sidebar Layout (shared)
// ------------------------------------------------------------------
interface SidebarLayoutProps {
  isSidebarOpen: boolean;
  dms: React.ReactNode;
  spaces: React.ReactNode;
}

const SidebarLayout = ({ isSidebarOpen, dms, spaces }: SidebarLayoutProps) => (
  <div className="flex flex-col h-full py-4">
    <SidebarSection title="Direct Messages" isSidebarOpen={isSidebarOpen}>
      {dms}
    </SidebarSection>

    {isSidebarOpen && <div className="mx-2 my-2 border-t border-surface-200/50 flex-shrink-0" />}

    <SidebarSection title="Spaces" isSidebarOpen={isSidebarOpen}>
      {spaces}
    </SidebarSection>
  </div>
);

interface SidebarSectionProps {
  title: string;
  isSidebarOpen: boolean;
  children: React.ReactNode;
}

const SidebarSection = ({ title, isSidebarOpen, children }: SidebarSectionProps) => (
  <div className="flex-1 flex flex-col min-h-0">
    {isSidebarOpen && (
      <div className="px-2 py-1 text-[9px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between">
        <span>{title}</span>
      </div>
    )}
    <div className="overflow-y-auto px-2 flex flex-col" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
      {children}
    </div>
  </div>
);

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
          <img
            src={item.iconUrl}
            alt=""
            className="rounded-md object-cover bg-surface-200"
            style={{ width: 'var(--layout-sidebar-item-icon-size)', height: 'var(--layout-sidebar-item-icon-size)' }}
          />
        ) : (
          <div
            className={`rounded-md flex items-center justify-center font-semibold text-[10px] ${
              item.isActive ? 'bg-surface-200 text-surface-800' : 'bg-surface-200 text-surface-500'
            }`}
            style={{ width: 'var(--layout-sidebar-item-icon-size)', height: 'var(--layout-sidebar-item-icon-size)' }}
          >
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
            <div className={`truncate font-medium text-[var(--layout-sidebar-item-text-size)] ${item.unreadCount > 0 ? 'text-surface-900 font-semibold' : ''}`}>
              {item.displayName}
            </div>
          </div>
          <div className="flex items-center gap-1">
            {item.unreadCount > 0 ? (
              <span className="h-2 w-2 rounded-full bg-[var(--layout-sidebar-accent)]" />
            ) : item.isPinned ? (
              <PushPinOutlinedIcon className="!h-3 !w-3 text-surface-500" fontSize="inherit" />
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
const SidebarSkeletonList = ({ isSidebarOpen, count }: { isSidebarOpen: boolean; count: number }) => (
  <div className="flex flex-col animate-pulse" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
    {Array.from({ length: count }).map((_, i) => (
      <div
        key={i}
        className={`rounded-[var(--layout-sidebar-item-radius)] bg-[var(--layout-sidebar-skeleton)] ${
          isSidebarOpen ? 'w-full' : 'w-8'
        }`}
        style={{ height: 'var(--layout-sidebar-item-height)' }}
      />
    ))}
  </div>
);