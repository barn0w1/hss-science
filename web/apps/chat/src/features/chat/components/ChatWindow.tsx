export const ChatHeader = () => {
	return <div className="h-full" />;
};

export const ChatSidebar = () => {
	return (
    // サイドバーの中身のイメージ
    <div className="flex flex-col h-full gap-6"> {/* gap-6で要素間の余白を大きく取る */}
      
      {/* 1. アプリ名 / ブランドロゴエリア */}
      <div className="px-4 py-2">
        <h1 className="text-xl font-bold bg-gradient-to-br from-primary-600 to-primary-400 bg-clip-text text-transparent">
          HSS Community
        </h1>
      </div>

      {/* 2. ナビゲーションリスト */}
      <nav className="flex-1 space-y-1 overflow-y-auto">
        {/* 
          【重要】リストアイテムのデザイン
          透明なサイドバーの上に乗るボタンは、
          「角丸のピル（カプセル）型」にすると、メイン画面の角丸と調和します。
        */}
        {['Lounge', 'General', 'Tech Talk'].map((room) => (
          <button 
            key={room}
            className="w-full flex items-center px-4 py-3 text-left 
                      text-surface-600 font-medium rounded-xl 
                      transition-all duration-200
                      hover:bg-white/60 hover:text-primary-600 hover:shadow-sm"
          >
            <span className="w-2 h-2 rounded-full bg-surface-300 mr-3" /> {/* 状態アイコン */}
            {room}
          </button>
        ))}
      </nav>

      {/* 3. ユーザー情報 (下部固定) */}
      <div className="p-4 bg-white/40 backdrop-blur-sm rounded-2xl border border-white/50 shadow-sm">
        <div className="text-sm font-bold text-surface-700">User Profile</div>
      </div>
    </div>
  )
};

export const ChatContent = () => {
	return <div className="h-full" />;
};
