import { useEffect } from 'react';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import { useChatStore } from '../state';
import { useSidebarData, type SidebarItemData } from '../hooks/useSidebarData';

// ------------------------------------------------------------------
// Main Component
// ------------------------------------------------------------------
export const ChatSidebar = ({ isSidebarOpen }: { isSidebarOpen: boolean }) => {
  const { fetchRooms, setActiveRoom } = useChatStore(); // Destructuring for cleaner access
  const { spaces, dms, isLoading } = useSidebarData();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  return (
    <div className="flex flex-col h-full py-4">
      <SidebarSection
        title="Direct Messages"
        items={dms}
        isLoading={isLoading}
        isOpen={isSidebarOpen}
        onSelect={setActiveRoom}
      />

      {/* Divider */}
      <div className={`mx-2 flex-shrink-0 border-t border-surface-200/50 transition-all ${isSidebarOpen ? 'my-2' : 'my-2 opacity-0'}`} />

      <SidebarSection
        title="Spaces"
        items={spaces}
        isLoading={isLoading}
        isOpen={isSidebarOpen}
        onSelect={setActiveRoom}
      />
    </div>
  );
};

// ------------------------------------------------------------------
// Section Layout
// ------------------------------------------------------------------
interface SidebarSectionProps {
  title: string;
  items: SidebarItemData[];
  isLoading: boolean;
  isOpen: boolean;
  onSelect: (id: string) => void;
}

const SidebarSection = ({ title, items, isLoading, isOpen, onSelect }: SidebarSectionProps) => {
  // Loading State
  if (isLoading) {
    return (
      <div className="flex-1 px-2 flex flex-col gap-[2px] overflow-hidden">
        {isOpen && <div className="h-4 mb-1" />} {/* Title placeholder */}
        {Array.from({ length: 5 }).map((_, i) => (
          <div 
            key={i} 
            className={`bg-surface-100 animate-pulse rounded-[var(--layout-sidebar-item-radius)] ${isOpen ? 'w-full' : 'w-full'}`} 
            style={{ height: 'var(--layout-sidebar-item-height)' }} 
          />
        ))}
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Title (Open only) */}
      {isOpen && (
        <div className="px-2 py-1 text-[9px] font-semibold uppercase tracking-wider text-surface-500 flex-shrink-0">
          {title}
        </div>
      )}

      {/* List */}
      <div className="overflow-y-auto px-2 flex flex-col" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
        {items.map((item) => (
          <SidebarItem
            key={item.id}
            item={item}
            isOpen={isOpen}
            onClick={() => onSelect(item.id)}
          />
        ))}
      </div>
    </div>
  );
};

// ------------------------------------------------------------------
// Item Component
// ------------------------------------------------------------------
const SidebarItem = ({ item, isOpen, onClick }: { item: SidebarItemData; isOpen: boolean; onClick: () => void }) => {
  return (
    <button
      onClick={onClick}
      className={`
        relative group w-full flex items-center outline-none transition-all duration-200
        rounded-[var(--layout-sidebar-item-radius)]
        ${isOpen ? 'px-3' : 'justify-center px-0'}
        ${item.isActive 
          ? 'bg-[var(--layout-sidebar-bg-active)] text-[var(--layout-sidebar-text-active)]' 
          : 'text-[var(--layout-sidebar-text)] hover:bg-[var(--layout-sidebar-bg-hover)]'
        }
      `}
      style={{ height: 'var(--layout-sidebar-item-height)' }}
    >
      {/* Active Indicator (Bar) */}
      {!isOpen && item.isActive && (
        <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-3 bg-[var(--layout-sidebar-accent)] rounded-r-full" />
      )}

      {/* Icon (共通化) */}
      <RoomIcon item={item} />

      {/* Content (Open only) */}
      {isOpen && (
        <>
          <div className={`ml-3 flex-1 text-left truncate text-[13px] ${item.unreadCount > 0 ? 'font-semibold text-surface-900' : 'font-medium'}`}>
            {item.displayName}
          </div>
          
          {/* Metadata (Badge / Pin) */}
          <div className="flex items-center gap-1.5">
            {item.unreadCount > 0 ? (
              <span className="h-2 w-2 rounded-full bg-[var(--layout-sidebar-accent)]" />
            ) : item.isPinned ? (
              <PushPinOutlinedIcon className="text-surface-400 rotate-45" style={{ fontSize: 13 }} />
            ) : null}
          </div>
        </>
      )}

      {/* Tooltip (Closed only) */}
      {!isOpen && (
        <div className="absolute left-full ml-2 px-2 py-1 bg-surface-800 text-white text-[10px] rounded opacity-0 group-hover:opacity-100 pointer-events-none z-50 whitespace-nowrap shadow-sm">
          {item.displayName}
        </div>
      )}
    </button>
  );
};

// ------------------------------------------------------------------
// Icon Helper Component
// ------------------------------------------------------------------
const RoomIcon = ({ item }: { item: SidebarItemData }) => {
  const sizeClass = "w-6 h-6"; // アイコンサイズの一元管理

  if (item.iconUrl) {
    return (
      <img
        src={item.iconUrl}
        alt=""
        className={`${sizeClass} rounded-md object-cover bg-surface-200 flex-shrink-0`}
      />
    );
  }

  return (
    <div
      className={`
        ${sizeClass} rounded-md flex items-center justify-center text-[10px] font-bold flex-shrink-0 transition-colors
        ${item.isActive ? 'bg-white/50 text-[var(--layout-sidebar-text-active)]' : 'bg-surface-200 text-surface-500'}
      `}
    >
      {item.fallbackInitial}
    </div>
  );
};