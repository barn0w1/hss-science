// features/search/components/GlobalSearchBar.tsx
import { Search } from 'lucide-react';

export const GlobalSearchBar = () => {
  return (
    <div className="group flex items-center w-full h-10 bg-gray-100 border border-transparent hover:bg-gray-200/70 focus-within:!bg-white focus-within:border-blue-500 focus-within:ring-1 focus-within:ring-blue-500 transition-all duration-200 rounded-full px-4 cursor-text">
      <Search 
        size={18} 
        className="text-gray-500 group-focus-within:text-blue-500 mr-3 flex-shrink-0 transition-colors duration-200" 
      />
      <input
        type="text"
        placeholder="Search everything..."
        className="flex-1 bg-transparent border-none outline-none text-gray-900 placeholder-gray-500 text-[14px] w-full"
      />
    </div>
  );
};