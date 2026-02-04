interface ChatHeaderProps {
  isSidebarOpen: boolean;
  onToggleSidebar: () => void;
}

export const ChatHeader = ({ isSidebarOpen, onToggleSidebar }: ChatHeaderProps) => {
	return (
    <div className="w-full flex items-center justify-between text-surface-700">
      <div className="flex items-center gap-3">
        <button
          className="h-9 w-9 rounded-full bg-white/80 border border-surface-200 flex items-center justify-center text-surface-500 hover:text-surface-700 hover:bg-white"
          onClick={onToggleSidebar}
          aria-label={isSidebarOpen ? 'Close sidebar' : 'Open sidebar'}
          type="button"
        >
          <span className="sr-only">Toggle sidebar</span>
          <span className="text-xs font-medium">{isSidebarOpen ? '◀' : '▶'}</span>
        </button>
        <div className="h-10 w-64 rounded-[var(--radius-pill)] bg-white border border-surface-200" />
        <div className="h-10 w-24 rounded-[var(--radius-pill)] bg-surface-100" />
      </div>
      <div className="flex items-center gap-3">
        <div className="h-10 w-10 rounded-[var(--radius-pill)] bg-surface-100" />
        <div className="h-10 w-10 rounded-[var(--radius-pill)] bg-surface-100" />
        <div className="h-10 w-10 rounded-[var(--radius-pill)] bg-surface-100" />
      </div>
    </div>
  );
};
