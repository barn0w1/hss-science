export const ChatHeader = () => {
	return (
    <div className="w-full flex items-center justify-between text-surface-700">
      <div className="flex items-center gap-3">
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
