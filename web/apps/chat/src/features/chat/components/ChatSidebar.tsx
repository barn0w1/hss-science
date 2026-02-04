interface ChatSidebarProps {
	isSidebarOpen: boolean;
	onToggleSidebar: () => void;
}

export const ChatSidebar = ({ isSidebarOpen, onToggleSidebar }: ChatSidebarProps) => {
	return (
		<div className="relative flex flex-col h-full gap-6 pt-14">
			<button
				className="absolute left-0 top-0 h-9 w-9 rounded-full bg-white/80 border border-surface-200 flex items-center justify-center text-surface-500 hover:text-surface-700 hover:bg-white transition"
				onClick={onToggleSidebar}
				aria-label={isSidebarOpen ? 'Close sidebar' : 'Open sidebar'}
				type="button"
			>
				<span className="sr-only">Toggle sidebar</span>
				<span className="text-xs font-medium">{isSidebarOpen ? '◀' : '▶'}</span>
			</button>

			{isSidebarOpen && (
				<div className="px-2">
					<div className="h-10 w-40 rounded-2xl bg-white/70 border border-white/60" />
				</div>
			)}

			{isSidebarOpen ? (
				<div className="px-2">
					<div className="h-10 w-full rounded-2xl bg-white/70 border border-white/60" />
				</div>
			) : (
				<div className="flex flex-col items-center gap-3">
					<div className="h-10 w-10 rounded-2xl bg-white/70 border border-white/60" />
					<div className="h-10 w-10 rounded-2xl bg-white/70 border border-white/60" />
				</div>
			)}

			<nav className={`flex-1 overflow-y-auto ${isSidebarOpen ? 'space-y-2 px-2' : 'space-y-3 px-1 flex flex-col items-center'}`}>
				{Array.from({ length: 6 }).map((_, index) => (
					<div
						key={index}
						className={
							isSidebarOpen
								? 'h-12 w-full rounded-2xl bg-white/50 border border-white/50'
								: 'h-10 w-10 rounded-2xl bg-white/50 border border-white/50'
						}
					/>
				))}
			</nav>
		</div>
	);
};
