import { MainLayout } from '../app/layouts/MainLayout';

export const ChatPage = () => {
  return (
    <MainLayout
      // サイドバーの中身（仮）
      sidebar={
        <div className="flex flex-col h-full">
          {/* ヘッダー部分は固定 */}
          <div className="h-16 flex items-center px-4 border-b border-surface-200 font-bold text-primary-600">
            HSS Chat
          </div>
          
          {/* リスト部分はスクロール可能に */}
          <div className="flex-1 overflow-y-auto p-2 space-y-2">
            {/* スクロールテスト用にダミーを大量生成 */}
            {Array.from({ length: 20 }).map((_, i) => (
              <div key={i} className="p-3 rounded-lg hover:bg-surface-200 cursor-pointer text-sm">
                # チャンネル {i + 1}
              </div>
            ))}
          </div>
        </div>
      }
    >
      {/* メインエリアの中身（仮）*/}
      <div className="flex flex-col h-full">
        {/* チャットヘッダー */}
        <div className="h-16 flex items-center px-6 border-b border-surface-200 bg-white">
          <h2 className="font-semibold text-lg"># HSS General</h2>
        </div>

        {/* チャットログ（ここがスクロールする） */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4 bg-surface-50">
          {Array.from({ length: 15 }).map((_, i) => (
            <div key={i} className={`flex ${i % 2 === 0 ? 'justify-start' : 'justify-end'}`}>
              <div className={`max-w-[70%] p-4 rounded-2xl ${
                i % 2 === 0 
                  ? 'bg-white border border-surface-200' 
                  : 'bg-primary-500 text-white'
              }`}>
                テストメッセージ {i + 1}
                <br />
                レイアウトのスクロール確認用です。
              </div>
            </div>
          ))}
        </div>

        {/* 入力エリア */}
        <div className="p-4 border-t border-surface-200 bg-white">
          <div className="h-12 border border-surface-300 rounded-full bg-surface-50 flex items-center px-4 text-surface-500">
            メッセージを送信...
          </div>
        </div>
      </div>
    </MainLayout>
  );
};