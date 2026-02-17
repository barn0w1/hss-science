// pages/ChatPage.tsx
import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';
import { GlobalSearchBar } from '@/features/search/components/GlobalSearchBar';
import { ChatSidebar } from '@/features/chat/components/ChatSidebar';
import { Edit } from 'lucide-react';

export const ChatPage = () => {
  return (
    <MainLayout
      header={
        <div className="w-full h-full flex">
          {/* --- 左側ヘッダー (変更なし) --- */}
          <div className="flex-none w-80 h-full border-r border-gray-200 flex items-center px-5 justify-between bg-white">
            <div className="flex items-center gap-3.5 min-w-0 pr-2">
              
            </div>
            <button className="flex-shrink-0 p-1.5 text-gray-400 hover:text-gray-900 hover:bg-gray-50 rounded-md transition-all duration-200">
              <Edit size={16} />
            </button>
          </div>

          {/* --- 右側ヘッダー --- */}
          <div className="flex-1 h-full flex items-center px-6 justify-between bg-white">
            {/* 検索バー：max-w-xlで幅を広げ、h-10で高さをアバターと合わせる */}
            <div className="w-full max-w-xl h-10">
              <GlobalSearchBar />
            </div>
            
            <div className="flex-none flex items-center gap-4 ml-4">
              {/* アバター：h-10で検索バーと高さを統一 */}
              <div className="w-10 h-10 rounded-full bg-gradient-to-tr from-gray-100 to-gray-200 border border-gray-200 shadow-sm cursor-pointer"></div>
            </div>
          </div>
        </div>
      }
    >
      {/* MainAreaLayout は変更なし */}
      <MainAreaLayout
        left={<ChatSidebar />}
        right={
          <div className="w-full h-full bg-[#fcfcfc] flex flex-col items-center justify-center p-8">
            <div className="max-w-sm w-full text-center flex flex-col items-center">
              <img
                src="/relaxing-outdoors.svg"
                alt="Empty state"
                className="w-48 h-48 mb-8 opacity-60 mix-blend-multiply transition-opacity duration-500 hover:opacity-80"
              />
              <h2 className="text-lg font-medium text-gray-800 tracking-tight">
                A quiet space.
              </h2>
              <p className="mt-1.5 text-sm text-gray-400 font-light">
                Select a conversation to start.
              </p>
            </div>
          </div>
        }
      />
    </MainLayout>
  );
};