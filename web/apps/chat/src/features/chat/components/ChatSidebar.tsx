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
          ? 'bg-[var(--layout-sidebar-bg-active)] text-[var(--layout-sidebar-text-active)] shadow-[0_4px_12px_rgba(0,0,0,0.05)]'
          : 'text-[var(--layout-sidebar-text)] hover:bg-[var(--layout-sidebar-bg-hover)] hover:text-surface-900'
      }`}
      style={{ 
        width: 'var(--layout-sidebar-item-width)', 
        height: 'var(--layout-sidebar-item-height)',
        borderRadius: 'var(--layout-sidebar-item-radius)'
      }}
    >
      <Icon 
        style={{ 
          fontSize: 'var(--layout-sidebar-item-icon-size)' 
        }} 
      />
      
      {/* Tooltip */}
      <div className="absolute left-full ml-4 px-3 py-1.5 bg-surface-900 text-white text-xs font-semibold rounded-lg opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-200 pointer-events-none whitespace-nowrap z-50 shadow-xl top-1/2 -translate-y-1/2">
        {item.label}
        {/* Arrow */}
        <div className="absolute right-full top-1/2 -translate-y-1/2 border-[6px] border-transparent border-r-surface-900" />
      </div>
      
      {/* Active Indicator Pillage (optional, nice for docks) */}
      {isActive && (
        <div className="absolute -left-3 top-1/2 -translate-y-1/2 w-1 h-5 bg-primary-500 rounded-r-full" />
      )}
    </button>
  );
};
