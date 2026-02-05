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
      <div className="flex flex-col items-center h-full w-full py-4">
        <nav className="flex flex-col items-center w-full" style={{ rowGap: 'var(--sidebar-item-gap)' }}>
          {menuItems.map((item) => (
            <SidebarRailItem
              key={item.id}
              item={item}
              isActive={activeId === item.id}
              onClick={() => setActiveId(item.id)}
            />
          ))}
        </nav>
        
        {/* Bottom Area (Settings / User) */}
        <div className="mt-auto pb-4">
           {/* 必要に応じて追加 */}
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
}

const SidebarRailItem = ({ item, isActive, onClick }: SidebarRailItemProps) => {
  const Icon = item.icon;
  
  return (
    <div className="relative group flex justify-center">
      <button
        onClick={onClick}
        className={`
          relative flex items-center justify-center outline-none 
          transition-all duration-200 ease-out
          /* 2. Tactile Feedback: クリック時に少し縮む */
          active:scale-90 active:duration-75
          ${
            isActive
              ? 'bg-[var(--sidebar-bg-active)] text-[var(--sidebar-text-active)] shadow-[var(--sidebar-shadow-active)] scale-100'
              : 'text-[var(--sidebar-text)] hover:bg-[var(--sidebar-bg-hover)] hover:text-[var(--sidebar-text-hover)] hover:scale-105'
          }
        `}
        style={{ 
          width: 'var(--sidebar-item-size)', 
          height: 'var(--sidebar-item-size)',
          borderRadius: 'var(--sidebar-item-radius)'
        }}
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
        {/* 三角形の矢印 */}
        <div className="absolute left-0 top-1/2 -translate-y-1/2 -translate-x-[4px] border-[4px] border-transparent border-r-[var(--color-surface-800)]" />
      </div>
    </div>
  );
};