export const ChatSidebar = () => {
	return (
		<div className="flex flex-col h-full gap-6">
			<div className="px-2">
				<div className="h-10 w-40 rounded-2xl bg-white/70 border border-white/60" />
			</div>

			<div className="px-2">
				<div className="h-10 w-full rounded-2xl bg-white/70 border border-white/60" />
			</div>

			<nav className="flex-1 space-y-2 overflow-y-auto px-2">
				{Array.from({ length: 6 }).map((_, index) => (
					<div
						key={index}
						className="h-12 w-full rounded-2xl bg-white/50 border border-white/50"
					/>
				))}
			</nav>

			<div className="p-4 bg-white/60 backdrop-blur-sm rounded-3xl border border-white/70 shadow-sm">
				<div className="h-4 w-28 rounded-full bg-surface-200" />
				<div className="mt-2 h-3 w-36 rounded-full bg-surface-100" />
			</div>
		</div>
	);
};
