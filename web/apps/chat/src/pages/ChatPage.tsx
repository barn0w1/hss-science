// pages/ChatPage.tsx
import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';
import { GlobalSearchBar } from '@/features/search/components/GlobalSearchBar';
import { ChatSidebar } from '@/features/chat/components/ChatSidebar'; // ★追加
import { Edit } from 'lucide-react'; 

export const ChatPage = () => {
  return (
    <MainLayout
      header={
        <div className="w-full h-full flex">
          
          {/* --- 左側ヘッダー --- */}
          <div className="flex-none w-80 h-full border-r border-gray-300 flex items-center px-6 justify-between bg-white">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-brand rounded-xl flex items-center justify-center text-white font-bold shadow-sm">
                A
              </div>
              <span className="font-bold text-gray-800 tracking-wide">Chat</span>
            </div>
            
            <button className="p-2 text-gray-400 hover:text-gray-700 hover:bg-gray-100 rounded-full transition-colors">
              <Edit size={18} />
            </button>
          </div>

          {/* --- 右側ヘッダー --- */}
          <div className="flex-1 h-full flex items-center px-6 justify-between bg-white">
            <div className="w-full max-w-xl">
              <GlobalSearchBar />
            </div>
            <div className="flex-none flex items-center gap-4 ml-4">
              <div className="w-8 h-8 rounded-full bg-gray-200 border border-gray-300 cursor-pointer"></div>
            </div>
          </div>
          
        </div>
      }
    >
      <MainAreaLayout
        // ★ ここで ChatSidebar を呼び出す！
        left={<ChatSidebar />} 

        right={
          <div className="w-full h-full bg-white flex flex-col items-center justify-center p-8">
            <div className="max-w-md w-full text-center space-y-6">
              <img 
                src="/relaxing-outdoors.svg" 
                alt="No conversations" 
                className="w-64 h-64 mx-auto opacity-90"
              />
              <div className="space-y-2">
                <h2 className="text-xl font-semibold text-gray-800">
                  It's quiet in here...
                </h2>
                <p className="text-gray-500 text-sm leading-relaxed">
                  There are no messages yet. <br />
                  Search for a workspace or start a new conversation to get started.
                </p>
              </div>
            </div>
          </div>
        }
      />
    </MainLayout>
  );
};