// features/search/components/GlobalSearchBar.tsx
import { Search } from 'lucide-react';

export const GlobalSearchBar = () => {
  return (
    <div className="group flex items-center w-full bg-gray-100 border border-transparent hover:bg-gray-100/80 hover:border-gray-200/50 focus-within:bg-white focus-within:border-gray-200/80 focus-within:shadow-[0_2px_12px_-3px_rgba(0,0,0,0.08)] transition-all duration-300 rounded-full px-4 py-2 cursor-text">
      
      <Search 
        size={15} 
        className="text-gray-400 group-focus-within:text-gray-600 mr-3 flex-shrink-0 transition-colors duration-300" 
      />
      
      <input
        type="text"
        placeholder="Search messages or spaces..."
        className="flex-1 bg-transparent border-none outline-none text-gray-800 placeholder-gray-400 text-[13px] font-medium focus:ring-0 w-full leading-normal"
      />
      
      <div className="flex-shrink-0 ml-3 hidden sm:flex items-center gap-0.5 opacity-0 group-hover:opacity-100 focus-within:!opacity-0 transition-opacity duration-300">
        <kbd className="inline-flex items-center justify-center text-[10px] font-sans font-semibold text-gray-400/70 border border-gray-200/60 rounded-full px-2 py-0.5 bg-white shadow-sm">
          ⌘
        </kbd>
        <kbd className="inline-flex items-center justify-center text-[10px] font-sans font-semibold text-gray-400/70 border border-gray-200/60 rounded-full px-2 py-0.5 bg-white shadow-sm">
          K
        </kbd>
      </div>

    </div>
  );
};