// features/search/components/GlobalSearchBar.tsx
import { Search } from 'lucide-react';

export const GlobalSearchBar = () => {
  return (
    <div className="flex items-center w-full bg-gray-100 hover:bg-gray-200/70 focus-within:bg-gray-200/70 transition-colors duration-200 rounded-lg px-3 py-2">
      {/* シンプルな虫眼鏡アイコン */}
      <Search size={16} className="text-gray-400 mr-2 flex-shrink-0" />
      
      {/* 入力フォーム */}
      <input
        type="text"
        placeholder="Search..."
        className="flex-1 bg-transparent border-none outline-none text-gray-900 placeholder-gray-500 text-sm focus:ring-0"
      />
    </div>
  );
};