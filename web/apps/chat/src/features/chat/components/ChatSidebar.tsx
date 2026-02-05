import type { ReactNode } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import ChatBubbleOutlineIcon from '@mui/icons-material/ChatBubbleOutline';
import GroupsOutlinedIcon from '@mui/icons-material/GroupsOutlined';
import AddOutlinedIcon from '@mui/icons-material/AddOutlined';

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
  const navigate = useNavigate();
  const location = useLocation();

  const railButtonBaseClass =
    'relative flex items-center justify-center outline-none transition-all duration-200 ease-out active:scale-90 active:duration-75';

  const menuItems = [
    { id: 'home', label: 'Home', icon: HomeOutlinedIcon, path: '/chat/home' },
    { id: 'dm', label: 'DM', icon: ChatBubbleOutlineIcon, path: '/chat/dm' },
    { id: 'spaces', label: 'Spaces', icon: GroupsOutlinedIcon, path: '/chat/space' },
  ];

  // Determine active item based on current path
  const getActiveId = () => {
    if (location.pathname.startsWith('/chat/dm')) return 'dm';
    if (location.pathname.startsWith('/chat/space')) return 'spaces';
    if (location.pathname === '/chat/home') return 'home';
    return null;
  };

  const activeId = getActiveId();

  const handleItemClick = (item: typeof menuItems[number]) => {
    navigate(item.path);
  };

  return (
    <ChatSidebarLayout>
      <div className="flex flex-col items-center h-full w-full py-4">
        <nav className="flex flex-col items-center w-full gap-[var(--sidebar-item-gap)]">
          {menuItems.map((item) => (
            <SidebarRailItem
              key={item.id}
              item={item}
              isActive={activeId === item.id}
              onClick={() => handleItemClick(item)}
              baseClassName={railButtonBaseClass}
            />
          ))}
        </nav>

        {/* Bottom Area (Settings / User) */}
        <div className="mt-auto pb-4 flex flex-col items-center">
          <button
            aria-label="New"
            className={`
              ${railButtonBaseClass}
              w-[var(--sidebar-item-size)] h-[var(--sidebar-item-size)]
              rounded-[var(--sidebar-item-radius)]
              bg-[var(--sidebar-bg-active)] text-[var(--sidebar-text-active)]
              hover:bg-[var(--sidebar-bg-hover)] hover:text-[var(--sidebar-text-hover)]
              hover:shadow-[var(--sidebar-shadow-active)] hover:scale-105
              active:scale-95
            `}
          >
            <AddOutlinedIcon style={{ fontSize: '24px' }} />
          </button>
        </div>
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
  baseClassName: string;
}

const SidebarRailItem = ({ item, isActive, onClick, baseClassName }: SidebarRailItemProps) => {
  const Icon = item.icon;
  
  return (
    <div className="relative group flex justify-center">
      <button
        onClick={onClick}
        className={`
          ${baseClassName}
          w-[var(--sidebar-item-size)] h-[var(--sidebar-item-size)]
          rounded-[var(--sidebar-item-radius)]
          ${
            isActive
              ? 'bg-[var(--sidebar-bg-active)] text-[var(--sidebar-text-active)] shadow-[var(--sidebar-shadow-active)] scale-100'
              : 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-bg-hover)] hover:text-[var(--sidebar-text-hover)] hover:scale-105'
          }
        `}
      >
        <Icon 
          className="transition-transform duration-300"
          style={{ 
            fontSize: 'var(--sidebar-icon-size)',
            // アクティブ時はアイコンを少し大きく見せても良い
            // transform: isActive ? 'scale(1.05)' : 'scale(1)'
          }} 
        />
      </button>

      {/* 
         3. Modern Tooltip 
         アイコンのみのUIには必須。ホバー時のみ右側にフワッと出す。
      */}
      <div className="
        absolute left-full top-1/2 -translate-y-1/2 ml-3 px-3 py-1.5 
        bg-[var(--color-surface-800)] text-white text-xs font-medium rounded-lg 
        opacity-0 group-hover:opacity-100 translate-x-[-8px] group-hover:translate-x-0 
        transition-all duration-200 pointer-events-none whitespace-nowrap z-50 shadow-xl
      ">
        {item.label}
      </div>
    </div>
  );
};
