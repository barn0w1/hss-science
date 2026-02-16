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
          {/* --- 左側ヘッダー --- */}
          <div className="flex-none w-80 h-full border-r border-gray-200 flex items-center px-5 justify-between bg-white">
            
            <div className="flex items-center gap-3.5 min-w-0 pr-2">
              <img
                src="/icon.svg"
                alt="HSSSC Icon"
                className="w-8 h-8 flex-shrink-0 object-contain drop-shadow-sm" 
              />

              <div className="min-w-0 flex flex-col justify-center">
                <div className="flex items-baseline gap-2">
                  <span className="text-base font-extrabold text-gray-900 tracking-tight truncate leading-none">
                    Chat
                  </span>
                </div>
                {/* 文字間隔(tracking)と色味で洗練された印象に */}
                <div className="mt-1">
                  <span className="text-[10px] font-bold text-gray-400/80 uppercase tracking-[0.2em] truncate block leading-none">
                    HSS Science
                  </span>
                </div>
              </div>
            </div>

            <button className="flex-shrink-0 p-1.5 text-gray-400 hover:text-gray-900 hover:bg-gray-50 rounded-md transition-all duration-200">
              <Edit size={16} />
            </button>
          </div>

          {/* --- 右側ヘッダー --- */}
          <div className="flex-1 h-full flex items-center px-6 justify-between bg-white">
            <div className="w-full max-w-sm">
              <GlobalSearchBar />
            </div>
            <div className="flex-none flex items-center gap-4 ml-4">
              {/* アバタープレースホルダーを少し上品に */}
              <div className="w-7 h-7 rounded-full bg-gradient-to-tr from-gray-100 to-gray-200 border border-gray-200 shadow-sm cursor-pointer"></div>
            </div>
          </div>
        </div>
      }
    >
      <MainAreaLayout
        left={<ChatSidebar />}
        right={
          /* 背景をほんのり冷たいグレーに寄せることで、キャンバス感を出す（好みでbg-whiteに戻してもOK） */
          <div className="w-full h-full bg-[#fcfcfc] flex flex-col items-center justify-center p-8">
            <div className="max-w-sm w-full text-center flex flex-col items-center">
              
              {/* イラストを少し小さくし、透明度を下げて主張を抑える */}
              <img
                src="/relaxing-outdoors.svg"
                alt="Empty state"
                className="w-48 h-48 mb-8 opacity-60 mix-blend-multiply transition-opacity duration-500 hover:opacity-80"
              />
              
              {/* 詩的でミニマルなテキスト */}
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