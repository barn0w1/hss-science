import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';
import { useChatStore } from '@/features/chat/state';
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import type { DirectMessage } from '@/features/chat/types';

export const DMListPage = () => {
  const navigate = useNavigate();
  const { rooms, isLoading, error, fetchRooms } = useChatStore();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  // DMのみをフィルタリング
  const dms = rooms.filter((room): room is DirectMessage => room.type === 'dm');

  // ピン留めとlastActiveAtでソート
  const sortedDms = [...dms].sort((a, b) => {
    if (a.isPinned !== b.isPinned) return a.isPinned ? -1 : 1;
    return new Date(b.lastActiveAt).getTime() - new Date(a.lastActiveAt).getTime();
  });

  const handleDMClick = (dmId: string) => {
    navigate(`/chat/dm/${dmId}`);
  };

  return (
    <MainLayout header={<ChatHeader />} sidebar={<ChatSidebar />}>
      <div className="h-full flex flex-col pr-[var(--layout-padding)] pb-[var(--layout-padding)]">
        <div className="layout-content-body h-full flex flex-col rounded-[var(--radius-panel)] overflow-hidden">
          <div className="border-b border-surface-100 px-10 py-6">
            <h1 className="text-2xl font-bold">ダイレクトメッセージ</h1>
          </div>

          <div className="flex-1 overflow-y-auto px-10 py-6">
            {isLoading && <div>読み込み中...</div>}
            {error && <div className="text-red-500">{error}</div>}

            {!isLoading && !error && (
              <div className="space-y-2">
                {sortedDms.map((dm) => (
                  <button
                    key={dm.id}
                    onClick={() => handleDMClick(dm.id)}
                    className="w-full p-4 bg-surface-50 hover:bg-surface-100 rounded-lg
                               transition-colors text-left flex items-center justify-between"
                  >
                    <div className="flex-1">
                      <div className="font-medium">
                        {dm.name || `DM: ${dm.memberIds.join(', ')}`}
                      </div>
                      <div className="text-sm text-surface-600">
                        {new Date(dm.lastActiveAt).toLocaleString('ja-JP')}
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      {dm.isPinned && <span className="text-xs">📌</span>}
                      {dm.isMuted && <span className="text-xs">🔇</span>}
                      {dm.unreadCount > 0 && (
                        <div className="bg-blue-500 text-white rounded-full px-2 py-1 text-xs">
                          {dm.unreadCount}
                        </div>
                      )}
                    </div>
                  </button>
                ))}

                {sortedDms.length === 0 && (
                  <div className="text-center py-10 text-surface-500">DMがありません</div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </MainLayout>
  );
};
