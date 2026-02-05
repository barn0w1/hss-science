import { useState } from 'react';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import InboxOutlinedIcon from '@mui/icons-material/InboxOutlined';
import GroupsOutlinedIcon from '@mui/icons-material/GroupsOutlined';
import CloudQueueOutlinedIcon from '@mui/icons-material/CloudQueueOutlined';

export const ChatSidebar = () => {
  const [activeId, setActiveId] = useState('home');

  const menuItems = [
    { id: 'home', label: 'Home', icon: HomeOutlinedIcon },
    { id: 'inbox', label: 'Inbox', icon: InboxOutlinedIcon },
    { id: 'spaces', label: 'Spaces', icon: GroupsOutlinedIcon },
    { id: 'drive', label: 'Drive', icon: CloudQueueOutlinedIcon },
  ];

  return (
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
      
      {/* Active Indicator: A subtle dot or small bar to the left */}
      {isActive && (
        <div className="absolute -left-3 top-1/2 -translate-y-1/2 h-8 w-1 bg-primary-500 rounded-full" />
      )}
    </button>
  );
};
