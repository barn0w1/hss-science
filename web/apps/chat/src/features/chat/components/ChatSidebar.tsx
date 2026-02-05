import type { ReactNode } from 'react';
import { useState } from 'react';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import InboxOutlinedIcon from '@mui/icons-material/InboxOutlined';
import GroupsOutlinedIcon from '@mui/icons-material/GroupsOutlined';
import CloudQueueOutlinedIcon from '@mui/icons-material/CloudQueueOutlined';
import { DebugPlaceholder } from '@/shared/ui/DebugPlaceholder';

const IS_LAYOUT_DEBUG = false;

const ChatSidebarLayout = ({ children }: { children: ReactNode }) => (
  <div className="h-full w-full">
    {IS_LAYOUT_DEBUG ? (
      <DebugPlaceholder
        label="Chat Sidebar"
        color="bg-red-500/20 border-red-500/50 text-red-700"
      />
    ) : (
      children
    )}
  </div>
);

export const ChatSidebar = () => {
  const [activeId, setActiveId] = useState('home');

  const menuItems = [
    { id: 'home', label: 'Home', icon: HomeOutlinedIcon },
    { id: 'inbox', label: 'Inbox', icon: InboxOutlinedIcon },
    { id: 'spaces', label: 'Spaces', icon: GroupsOutlinedIcon },
    { id: 'drive', label: 'Drive', icon: CloudQueueOutlinedIcon },
  ];

  return (
    <ChatSidebarLayout>
      <div className="flex flex-col items-center h-full w-full py-2">
        <nav className="flex flex-col items-center w-full" style={{ rowGap: 'var(--layout-sidebar-item-gap)' }}>
          {menuItems.map((item) => (
            <SidebarRailItem
              key={item.id}
              item={item}
              isActive={activeId === item.id}
              onClick={() => setActiveId(item.id)}
            />
          ))}
        </nav>
      </div>
    </ChatSidebarLayout>
  );
};

interface SidebarRailItemProps {
  item: {
    id: string;
    label: string;
    icon: React.ElementType;
  };
  isActive: boolean;
  onClick: () => void;
}

const SidebarRailItem = ({ item, isActive, onClick }: SidebarRailItemProps) => {
  const Icon = item.icon;
  
  return (
    <button
      onClick={onClick}
      className={`relative group flex items-center justify-center outline-none transition-all duration-300 ease-out ${
        isActive
          ? 'bg-[var(--sidebar-bg-active)] text-[var(--sidebar-text-active)] shadow-sm'
          : 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-bg-hover)] hover:text-surface-900'
      }`}
      style={{ 
        width: 'var(--sidebar-item-size)', 
        height: 'var(--sidebar-item-size)',
        borderRadius: 'var(--sidebar-item-radius)'
      }}
    >
      <Icon 
        style={{ 
          fontSize: 'var(--sidebar-icon-size)' 
        }} 
      />
    </button>
  );
};
