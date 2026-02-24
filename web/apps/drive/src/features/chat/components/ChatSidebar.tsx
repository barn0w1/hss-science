// features/chat/components/ChatSidebar.tsx
import { useState } from 'react';
import { Hash, Pin, UserPlus } from 'lucide-react';

// ダミーデータ（メッセージ用）
const mockChats = [
  {
    id: '1',
    type: 'dm',
    name: 'Alice Smith',
    preview: 'You sent a post',
    time: '3m',
    avatar: 'https://content.webtarget.dev/icons/00.webp',
    unread: false,
    isActive: false,
    isPinned: true,
    isOnline: true,
  },
  {
    id: '2',
    type: 'dm',
    name: 'Home! Atlanta',
    preview: 'It\'s a blast',
    time: '4h',
    avatar: 'https://content.webtarget.dev/icons/01.webp',
    unread: true,
    isActive: false,
    isPinned: false,
    isOnline: true,
  },
  {
    id: '3',
    type: 'space',
    name: 'AlphaZero',
    preview: 'Alice: 8/10 is impressive!',
    time: '2d',
    avatar: null,
    unread: false,
    isActive: false,
    isPinned: true,
    isOnline: false,
  },
  {
    id: '4',
    type: 'space',
    name: 'London',
    preview: 'Bob: I\'m loving the new features',
    time: '50m',
    avatar: null,
    unread: false,
    isActive: false,
    isPinned: true,
    isOnline: false,
  },
  {
    id: '5',
    type: 'dm',
    name: 'Charlie Davis',
    preview: 'Which led me here',
    time: '5d',
    avatar: 'https://content.webtarget.dev/icons/02.webp',
    unread: true,
    isActive: false, // 現在選択中
    isPinned: false,
    isOnline: false,
  },
  {
    id: '6',
    type: 'dm',
    name: 'David Wilson',
    preview: 'Thanks for the help!',
    time: '1w',
    avatar: 'https://content.webtarget.dev/icons/03.png',
    unread: true,
    isActive: false, // 現在選択中
    isPinned: false,
    isOnline: false,
  },
  {
    id: '7',
    type: 'dm',
    name: 'HSS Science Club',
    preview: 'Let\'s catch up soon',
    time: '30m',
    avatar: 'https://content.webtarget.dev/icons/04.png',
    unread: false,
    isActive: false, // 現在選択中
    isPinned: false,
    isOnline: true,
  },
];

// ダミーデータ（リクエスト用）
const mockRequests = [
  {
    id: 'r1',
    name: 'Elena Rostova',
    preview: 'Wants to send you a message',
    time: '1h',
    avatar: 'https://content.webtarget.dev/icons/01.webp',
  }
];

export const ChatSidebar = () => {
  const [activeTab, setActiveTab] = useState<'messages' | 'requests'>('messages');

  const sortedChats = [...mockChats].sort((a, b) => 
    b.isPinned === a.isPinned ? 0 : b.isPinned ? 1 : -1
  );

  const renderChatItem = (chat: typeof mockChats[0]) => (
    <div
      key={chat.id}
      className={`group flex items-center gap-3 px-4 py-2.5 cursor-pointer transition-colors duration-150 ${
        chat.isActive 
          ? 'bg-gray-50' 
          : 'hover:bg-gray-50/50'
      }`}
    >
      <div className="relative flex-shrink-0">
        {chat.type === 'dm' ? (
          <img 
            src={chat.avatar!} 
            alt={chat.name} 
            className="w-10 h-10 rounded-full object-cover border border-gray-100"
          />
        ) : (
          <div className="w-10 h-10 rounded-[14px] bg-gray-50 text-gray-400 flex items-center justify-center border border-gray-100">
            <Hash size={18} strokeWidth={2} />
          </div>
        )}

        {chat.type === 'dm' && chat.isOnline && (
          <div className="absolute -bottom-0.5 -right-0.5 w-3 h-3 bg-emerald-500 border-2 border-white rounded-full z-10"></div>
        )}
      </div>

      <div className="flex-1 min-w-0 flex flex-col justify-center">
        <div className="flex justify-between items-center mb-0.5">
          <div className="flex items-center gap-1.5 min-w-0">
            <span className={`truncate text-[13.5px] ${chat.unread ? 'font-semibold text-gray-900' : 'font-medium text-gray-500'}`}>
              {chat.name}
            </span>
            {chat.isPinned && (
              <Pin size={11} className="text-gray-300 flex-shrink-0" strokeWidth={2.5} />
            )}
          </div>
          
          <div className="flex items-center gap-2 flex-shrink-0 pl-2">
            {chat.unread && (
              <div className="w-1.5 h-1.5 bg-blue-500 rounded-full"></div>
            )}
            <span className={`text-[11px] ${chat.unread ? 'font-medium text-blue-500' : 'text-gray-400'}`}>
              {chat.time}
            </span>
          </div>
        </div>
        
        <span className={`text-[12px] truncate ${chat.unread ? 'font-medium text-gray-800' : 'text-gray-400'}`}>
          {chat.preview}
        </span>
      </div>
    </div>
  );

  return (
    <div className="w-full h-full flex flex-col bg-white">
      
      {/* タブ領域 */}
      <div className="flex w-full border-b border-gray-100 flex-shrink-0">
        <button
          onClick={() => setActiveTab('messages')}
          className={`relative flex-1 py-3.5 text-[11px] font-bold uppercase tracking-[0.08em] transition-colors duration-200 ${
            activeTab === 'messages' 
              ? 'text-gray-900' 
              : 'text-gray-400 hover:text-gray-600'
          }`}
        >
          Messages
          {activeTab === 'messages' && (
            <div className="absolute bottom-0 left-0 w-full h-[1.5px] bg-gray-900"></div>
          )}
        </button>
        
        <button
          onClick={() => setActiveTab('requests')}
          className={`relative flex-1 flex items-center justify-center py-3.5 text-[11px] font-bold uppercase tracking-[0.08em] transition-colors duration-200 ${
            activeTab === 'requests' 
              ? 'text-gray-900' 
              : 'text-gray-400 hover:text-gray-600'
          }`}
        >
          <span>Requests</span>
          {/* ソフトで上品なカラーバッジ */}
          {mockRequests.length > 0 && (
            <span className={`ml-1.5 flex items-center justify-center px-1.5 min-w-[18px] h-[18px] rounded-full text-[10px] transition-colors ${
              activeTab === 'requests' 
                ? 'bg-blue-100 text-blue-700' 
                : 'bg-blue-50 text-blue-600'
            }`}>
              {mockRequests.length}
            </span>
          )}
          {activeTab === 'requests' && (
            <div className="absolute bottom-0 left-0 w-full h-[1.5px] bg-gray-900"></div>
          )}
        </button>
      </div>

      {/* リスト領域 */}
      <div className="flex-1 overflow-y-auto pb-4 pt-1">
        {activeTab === 'messages' ? (
          sortedChats.map(renderChatItem)
        ) : (
          <div>
            <div className="px-5 py-3 text-[11px] font-semibold uppercase tracking-wider text-gray-400">
              Pending requests
            </div>
            {mockRequests.map(req => (
              <div key={req.id} className="group flex items-center gap-3 px-4 py-2.5 cursor-pointer hover:bg-gray-50 transition-colors duration-150">
                <img src={req.avatar} alt={req.name} className="w-10 h-10 rounded-full object-cover border border-gray-100" />
                <div className="flex-1 min-w-0 flex flex-col justify-center">
                  <div className="flex justify-between items-center mb-0.5">
                    <span className="truncate text-[13.5px] font-semibold text-gray-900">{req.name}</span>
                    <span className="text-[11px] text-gray-400">{req.time}</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-[12px] font-medium text-blue-500">
                    <UserPlus size={12} strokeWidth={2.5} />
                    <span className="truncate">{req.preview}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

    </div>
  );
};