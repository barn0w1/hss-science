import { useEffect } from 'react';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import { useChatStore } from '../state';
import { useSidebarData, type SidebarItemData } from '../hooks/useSidebarData';

export const ChatSidebar = () => {
  const fetchRooms = useChatStore((state) => state.fetchRooms);
  const setActiveRoom = useChatStore((state) => state.setActiveRoom);
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
        onSelect={setActiveRoom}
      />

      <SidebarDivider />

      <SidebarSection
        title="Spaces"
        items={spaces}
        isLoading={isLoading}
        onSelect={setActiveRoom}
      />
    </div>
  );
};

interface SidebarSectionProps {
  title: string;
  items: SidebarItemData[];
  isLoading: boolean;
  onSelect: (id: string) => void;
}

const SidebarSection = ({
  title,
  items,
  isLoading,
  onSelect,
}: SidebarSectionProps) => {
  return (
    <div className="flex-1 flex flex-col min-h-0">
      <div className="px-2 py-1 text-[9px] font-semibold uppercase tracking-wider text-surface-500 flex items-center justify-between flex-shrink-0 min-h-[20px]">
        <span>{title}</span>
      </div>
      
      <div
        className="overflow-y-auto px-2 flex flex-col"
        style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}
      >
        {isLoading ? (
          Array.from({ length: 6 }).map((_, i) => (
            <SidebarSkeletonItem key={i} />
          ))
        ) : (
          items.map((item) => (
            <SidebarItem
              key={item.id}
              item={item}
              onClick={() => onSelect(item.id)}
            />
          ))
        )}
      </div>
    </div>
  );
};

const SidebarDivider = () => (
  <div className="mx-2 my-2 border-t border-surface-200/50 flex-shrink-0" />
);

const SidebarSkeletonItem = () => (
  <div
    className="rounded-[var(--layout-sidebar-item-radius)] bg-[var(--layout-sidebar-skeleton)] animate-pulse w-full"
    style={{ height: 'var(--layout-sidebar-item-height)' }}
  />
);

interface SidebarItemProps {
  item: SidebarItemData;
  onClick: () => void;
}

const SidebarItem = ({ item, onClick }: SidebarItemProps) => {
  return (
    <button
      onClick={onClick}
      className={`relative group w-full flex items-center outline-none rounded-[var(--layout-sidebar-item-radius)] transition-all px-3 ${
        item.isActive
          ? 'bg-[var(--layout-sidebar-bg-active)] text-[var(--layout-sidebar-text-active)]'
          : 'text-[var(--layout-sidebar-text)] hover:bg-[var(--layout-sidebar-bg-hover)]'
      }`}
      style={{ height: 'var(--layout-sidebar-item-height)' }}
    >
      <div className="relative flex-shrink-0">
        {item.iconUrl ? (
          <img
            src={item.iconUrl}
            alt=""
            className="rounded-md object-cover bg-surface-200"
            style={{
              width: 'var(--layout-sidebar-item-icon-size)',
              height: 'var(--layout-sidebar-item-icon-size)',
            }}
          />
        ) : (
          <div
            className={`rounded-md flex items-center justify-center font-semibold text-[10px] ${
              item.isActive ? 'bg-surface-200 text-surface-800' : 'bg-surface-200 text-surface-500'
            }`}
            style={{
              width: 'var(--layout-sidebar-item-icon-size)',
              height: 'var(--layout-sidebar-item-icon-size)',
            }}
          >
            {item.fallbackInitial}
          </div>
        )}
      </div>

      <div className="ml-3 flex-1 text-left min-w-0">
        <div
          className={`truncate font-medium text-[var(--layout-sidebar-item-text-size)] ${
            item.unreadCount > 0 ? 'text-surface-900 font-semibold' : ''
          }`}
        >
          {item.displayName}
        </div>
      </div>
      <div className="flex items-center gap-1">
        {item.unreadCount > 0 ? (
          <span className="h-2 w-2 rounded-full bg-[var(--layout-sidebar-accent)]" />
        ) : item.isPinned ? (
          <PushPinOutlinedIcon
            className="!h-3 !w-3 text-surface-500"
            fontSize="inherit"
          />
        ) : null}
      </div>
    </button>
  );
};
