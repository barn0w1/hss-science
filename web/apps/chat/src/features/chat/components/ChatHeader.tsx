export const ChatHeader = () => {
	return (
    <div className="w-full flex items-center justify-between text-surface-700">
      <div className="flex items-center gap-3">
        <div className="h-9 w-9 rounded-xl bg-surface-200" />
        <div>
          <div className="text-xs uppercase tracking-wider text-surface-500">Channel</div>
          <h2 className="text-lg font-semibold text-surface-800">Design Preview</h2>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <div className="h-8 w-20 rounded-full bg-surface-100" />
        <div className="h-8 w-8 rounded-full bg-surface-100" />
      </div>
    </div>
  );
};
