interface ChatHeaderProps {
  isSidebarOpen: boolean;
  onToggleSidebar: () => void;
}

export const ChatHeader = ({ isSidebarOpen, onToggleSidebar }: ChatHeaderProps) => {
	return (
    <div className="relative w-full flex items-center text-surface-700">
      <button
        className="absolute -translate-x-1/2 left-[var(--layout-sidebar-toggle-x)] h-11 w-11 flex items-center justify-center text-surface-500 hover:text-[var(--layout-header-hover-fg)] hover:bg-[var(--layout-header-hover-bg)]"
        onClick={onToggleSidebar}
        aria-label={isSidebarOpen ? 'Close sidebar' : 'Open sidebar'}
        type="button"
      >
        <span className="sr-only">Toggle sidebar</span>
        <span className="mx-auto flex h-4 w-5 flex-col justify-between">
          <span className="block h-[2px] w-full rounded-full bg-current" />
          <span className="block h-[2px] w-full rounded-full bg-current" />
          <span className="block h-[2px] w-full rounded-full bg-current" />
        </span>
      </button>
    </div>
  );
};
