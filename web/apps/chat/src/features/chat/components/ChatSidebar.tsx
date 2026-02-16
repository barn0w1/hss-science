// features/chat/components/ChatSidebar.tsx
import { useState } from 'react';
import { Hash, Inbox, User } from 'lucide-react';

type FilterType = 'all' | 'space' | 'dm';

// ダミーデータを配列で定義（実運用ではAPIから取得するイメージ）
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
  },
  {
    id: '2',
    type: 'dm',
    name: 'Home! Atlanta',
    preview: 'It\'s a blast',
    time: '4h',
    avatar: 'https://content.webtarget.dev/icons/01.webp',
    unread: true, // 未読
    isActive: false,
  },
  {
    id: '3',
    type: 'space',
    name: 'AlphaZero',
    preview: 'Alice: 8/10 is impressive!',
    time: '2d',
    avatar: null, // Spaceは画像なし
    unread: false,
    isActive: true, // 現在選択中
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
  },
];

export const ChatSidebar = () => {
  const [activeFilter, setActiveFilter] = useState<FilterType>('all');

  // フィルター処理
  const filteredChats = mockChats.filter((chat) => {
    if (activeFilter === 'all') return true;
    return chat.type === activeFilter;
  });

  return (
    <div className="w-full h-full flex flex-col bg-white">
      
      {/* --- リストヘッダー & フィルター --- */}
      <div className="px-4 pt-6 pb-4 flex items-center justify-between border-b border-gray-100">
        <h2 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
          Messages
        </h2>
        
        <div className="flex bg-gray-100 p-0.5 rounded-md">
          <button
            onClick={() => setActiveFilter('all')}
            className={`p-1.5 rounded-sm transition-all ${activeFilter === 'all' ? 'bg-white shadow-sm text-gray-900' : 'text-gray-400 hover:text-gray-600'}`}
          >
            <Inbox size={14} />
          </button>
          <button
            onClick={() => setActiveFilter('space')}
            className={`p-1.5 rounded-sm transition-all ${activeFilter === 'space' ? 'bg-white shadow-sm text-gray-900' : 'text-gray-400 hover:text-gray-600'}`}
          >
            <Hash size={14} />
          </button>
          <button
            onClick={() => setActiveFilter('dm')}
            className={`p-1.5 rounded-sm transition-all ${activeFilter === 'dm' ? 'bg-white shadow-sm text-gray-900' : 'text-gray-400 hover:text-gray-600'}`}
          >
            <User size={14} />
          </button>
        </div>
      </div>

      {/* --- リスト部分 (大きく、贅沢な余白で) --- */}
      <div className="flex-1 overflow-y-auto">
        {filteredChats.map((chat) => (
          <div
            key={chat.id}
            className={`flex items-center gap-4 px-4 py-3 cursor-pointer transition-colors ${
              chat.isActive 
                ? 'bg-gray-100/80' // Instagramの選択中ハイライトのような色
                : 'hover:bg-gray-50'
            }`}
          >
            {/* アイコン画像エリア (w-14 h-14 とかなり大きめに設定) */}
            <div className="relative flex-shrink-0">
              {chat.type === 'dm' ? (
                <img 
                  src={chat.avatar!} 
                  alt={chat.name} 
                  className="w-14 h-14 rounded-full object-cover border border-gray-200"
                />
              ) : (
                // Space用のダミーアイコン (角丸四角形にしてDMの丸と区別)
                <div className="w-14 h-14 rounded-2xl bg-brand/10 text-brand flex items-center justify-center border border-brand/20">
                  <Hash size={24} />
                </div>
              )}
              
              {/* 未読のブルードット */}
              {chat.unread && (
                <div className="absolute top-0 right-0 w-3.5 h-3.5 bg-blue-500 border-2 border-white rounded-full"></div>
              )}
            </div>

            {/* テキスト情報エリア */}
            <div className="flex-1 min-w-0 flex flex-col justify-center">
              <div className="flex justify-between items-baseline mb-1">
                <span className={`truncate text-base ${chat.unread ? 'font-bold text-gray-900' : 'font-medium text-gray-800'}`}>
                  {chat.name}
                </span>
                <span className="text-xs text-gray-400 flex-shrink-0 ml-2">
                  {chat.time}
                </span>
              </div>
              <span className={`text-sm truncate ${chat.unread ? 'font-semibold text-gray-800' : 'text-gray-500'}`}>
                {chat.preview}
              </span>
            </div>
            
          </div>
        ))}
      </div>
    </div>
  );
};