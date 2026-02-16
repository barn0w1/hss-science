// features/search/components/GlobalSearchBar.tsx

export const GlobalSearchBar = () => {
  return (
    <div className="flex items-center w-full bg-gray-100 hover:bg-gray-200/80 focus-within:bg-white focus-within:shadow-md focus-within:ring-1 focus-within:ring-brand transition-all duration-200 rounded-full px-4 py-2">
      
      {/* 左側：ここにアプリアイコンを埋め込む！ */}
      {/* (Chromeでいう「G」マークや、faviconの領域) */}
      <div className="flex-shrink-0 flex items-center justify-center mr-3">
        {/* Teal(#14b8a6) のブランドカラーを使ったアイコン */}
        <div className="w-6 h-6 bg-brand rounded flex items-center justify-center text-white text-xs font-bold shadow-sm">
          A
        </div>
      </div>

      {/* 入力フォーム */}
      <input
        type="text"
        placeholder="Search workspace, messages, or files..."
        className="flex-1 bg-transparent border-none outline-none text-gray-900 placeholder-gray-500 text-sm focus:ring-0"
      />
      
      {/* 右側：ショートカットヒント */}
      <div className="flex-shrink-0 ml-3 text-[10px] text-gray-400 font-bold border border-gray-200 rounded px-1.5 py-0.5 uppercase tracking-wider">
        ⌘K
      </div>
    </div>
  );
};