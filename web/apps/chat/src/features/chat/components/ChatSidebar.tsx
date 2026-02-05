import { useEffect } from 'react';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import { useChatStore } from '../state';
import { useSidebarData, type SidebarItemData } from '../hooks/useSidebarData';

// ------------------------------------------------------------------
// Main Component
// ------------------------------------------------------------------
export const ChatSidebar = ({ isSidebarOpen }: { isSidebarOpen: boolean }) => {
  const { fetchRooms, setActiveRoom } = useChatStore();
  const { spaces, dms, isLoading } = useSidebarData();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  return (
    <div className="flex flex-col h-full py-4">
      {/* DM Section */}
      <SidebarSection
        title="Direct Messages"
        items={dms}
        isLoading={isLoading}
        isOpen={isSidebarOpen}
        onSelect={setActiveRoom}
      />

      {/* Divider */}
      <div 
        className={`mx-2 border-t border-surface-200/50 flex-shrink-0 transition-all duration-300 ${
          isSidebarOpen ? 'my-3 opacity-100' : 'my-2 opacity-0'
        }`} 
      />

      {/* Space Section */}
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
  if (isLoading) {
    return (
      <div className="flex-1 px-2 flex flex-col gap-[2px] overflow-hidden">
        {isOpen && <div className="h-3 w-20 bg-surface-100 rounded mb-2 animate-pulse" />}
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3 p-1">
            <div className="w-8 h-8 rounded-lg bg-surface-100 animate-pulse flex-shrink-0" />
            {isOpen && <div className="h-4 w-full bg-surface-100 rounded animate-pulse" />}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {isOpen && (
        <div className="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-surface-400 flex-shrink-0 hover:text-surface-600 cursor-default transition-colors">
          {title}
        </div>
      )}

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
  // Shape Strategy: DM is Circle, Space is Rounded Square
  const shapeClass = item.type === 'dm' ? 'rounded-full' : 'rounded-lg';

  return (
    <button
      onClick={onClick}
      className={`
        relative group w-full flex items-center outline-none transition-all duration-200
        rounded-xl border border-transparent
        ${isOpen ? 'px-2' : 'justify-center px-0'}
        ${item.isActive 
          ? 'bg-surface-100/80 text-primary-700' // Active: 薄いグレー背景 + テーマ色テキスト
          : 'text-surface-500 hover:bg-surface-50 hover:text-surface-900' // Inactive
        }
      `}
      style={{ height: '40px' }} // タップしやすい高さを確保
    >
      {/* Active Indicator (Left Bar) */}
      {item.isActive && (
        <div 
          className={`absolute left-0 top-1/2 -translate-y-1/2 w-1 bg-primary-500 rounded-r-full transition-all duration-300 ${
            isOpen ? 'h-4' : 'h-3'
          }`} 
        />
      )}

      {/* Icon Wrapper */}
      <RoomIcon item={item} shapeClass={shapeClass} />

      {/* Content (Open only) */}
      {isOpen && (
        <>
          <div className="ml-3 flex-1 text-left min-w-0">
            <div className={`truncate text-[13px] leading-tight ${item.isActive ? 'font-semibold' : 'font-medium'}`}>
              {item.displayName}
            </div>
          </div>
          
          {/* Metadata */}
          <div className="flex items-center gap-2">
            {item.isPinned && (
              <PushPinOutlinedIcon 
                className={`rotate-45 transition-colors ${item.isActive ? 'text-primary-400' : 'text-surface-300'}`} 
                style={{ fontSize: 14 }} 
              />
            )}
            {item.unreadCount > 0 && (
              <span className="flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full bg-primary-500 text-[10px] font-bold text-white shadow-sm">
                {item.unreadCount}
              </span>
            )}
          </div>
        </>
      )}

      {/* Tooltip (Closed only) */}
      {!isOpen && (
        <div className="absolute left-full ml-3 px-2.5 py-1.5 bg-surface-800 text-white text-[11px] font-medium rounded-lg opacity-0 group-hover:opacity-100 pointer-events-none z-50 whitespace-nowrap shadow-lg translate-x-1 group-hover:translate-x-0 transition-all">
          {item.displayName}
          {item.unreadCount > 0 && <span className="ml-2 opacity-70">({item.unreadCount})</span>}
        </div>
      )}
    </button>
  );
};

// ------------------------------------------------------------------
// Icon Helper Component
// ------------------------------------------------------------------
const RoomIcon = ({ item, shapeClass }: { item: SidebarItemData; shapeClass: string }) => {
  const sizeStyle = { width: '28px', height: '28px' }; // アイコンサイズ固定

  const content = item.iconUrl ? (
    <img
      src={item.iconUrl}
      alt=""
      className={`object-cover bg-surface-200 ring-1 ring-surface-900/5 ${shapeClass}`}
      style={sizeStyle}
    />
  ) : (
    <div
      className={`
        flex items-center justify-center text-[11px] font-bold transition-colors ring-1 ring-surface-900/5
        ${shapeClass}
        ${item.isActive 
          ? 'bg-white text-primary-600 shadow-sm' 
          : 'bg-surface-100 text-surface-500 group-hover:bg-surface-200'
        }
      `}
      style={sizeStyle}
    >
      {item.fallbackInitial}
    </div>
  );

  return (
    <div className="relative flex-shrink-0">
      {content}
      
      {/* Closed State Badges (アイコンに重ねる) */}
      {!item.isActive && item.unreadCount > 0 && (
        <span className="absolute -top-1 -right-1 h-2.5 w-2.5 rounded-full bg-primary-500 ring-2 ring-white" />
      )}
    </div>
  );
};