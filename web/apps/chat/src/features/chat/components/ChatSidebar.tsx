// features/chat/components/ChatSidebar.tsx
import { Hash, Pin } from 'lucide-react';

// ダミーデータ
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
  },
  {
    id: '3',
    type: 'space',
    name: 'AlphaZero',
    preview: 'Alice: 8/10 is impressive!',
    time: '2d',
    avatar: null,
    unread: false,
    isActive: true,
    isPinned: true,
  },
  {
    id: '4',
    type: 'dm',
    name: 'Charlie Davis',
    preview: 'Which led me here',
    time: '5d',
    avatar: 'https://content.webtarget.dev/icons/02.webp',
    unread: false,
    isActive: false,
    isPinned: false,
  },
];

export const ChatSidebar = () => {
  const pinnedChats = mockChats.filter(chat => chat.isPinned);
  const recentChats = mockChats.filter(chat => !chat.isPinned);

  const renderChatItem = (chat: typeof mockChats[0]) => (
    <div
      key={chat.id}
      className={`group flex items-center gap-3 px-4 py-2.5 cursor-pointer transition-all duration-200 ${
        chat.isActive 
          ? 'bg-gray-100/70' 
          : 'hover:bg-gray-50'
      }`}
    >
      {/* アバター領域：w-10 h-10 に縮小し、シャープな印象に */}
      <div className="relative flex-shrink-0">
        {chat.type === 'dm' ? (
          <img 
            src={chat.avatar!} 
            alt={chat.name} 
            className="w-10 h-10 rounded-full object-cover border border-gray-200/50 shadow-sm"
          />
        ) : (
          <div className="w-10 h-10 rounded-[14px] bg-gray-100 text-gray-500 flex items-center justify-center border border-gray-200/60 shadow-sm">
            <Hash size={18} strokeWidth={2.5} />
          </div>
        )}
        
        {/* 未読バッジ：少し小さく上品に */}
        {chat.unread && (
          <div className="absolute -top-0.5 -right-0.5 w-3 h-3 bg-blue-500 border-2 border-white rounded-full"></div>
        )}
      </div>

      {/* テキスト領域：フォントサイズを絞り、コントラストを強調 */}
      <div className="flex-1 min-w-0 flex flex-col justify-center">
        <div className="flex justify-between items-center mb-0.5">
          <span className={`truncate text-sm tracking-tight ${chat.unread ? 'font-semibold text-gray-900' : 'font-medium text-gray-700'}`}>
            {chat.name}
          </span>
          <div className="flex items-center gap-1.5 flex-shrink-0 ml-2">
            {chat.isPinned && <Pin size={10} className="text-gray-300 fill-gray-300" />}
            <span className="text-[11px] font-medium text-gray-400">{chat.time}</span>
          </div>
        </div>
        <span className={`text-xs truncate ${chat.unread ? 'font-medium text-gray-800' : 'font-light text-gray-500'}`}>
          {chat.preview}
        </span>
      </div>
    </div>
  );

  return (
    <div className="w-full h-full flex flex-col bg-white">
      {/* pt-2 で少しだけ上のゆとりを減らし、ヘッダーとの一体感を出す */}
      <div className="flex-1 overflow-y-auto pt-2 pb-4">
        
        {pinnedChats.length > 0 && (
          <div className="mb-1">
            {/* セクション見出し：極小サイズにして、ノイズにならないように */}
            <div className="px-5 py-2 text-[10px] font-bold text-gray-400/80 uppercase tracking-widest">
              Pinned
            </div>
            {pinnedChats.map(renderChatItem)}
          </div>
        )}

        {recentChats.length > 0 && (
          <div className="mt-2">
            {/* ボーダーを極細の透過グレーにし、主張を和らげる */}
            <div className="px-5 py-2 text-[10px] font-bold text-gray-400/80 uppercase tracking-widest border-t border-gray-100/80 pt-3">
              Recent
            </div>
            {recentChats.map(renderChatItem)}
          </div>
        )}
        
      </div>
    </div>
  );
};