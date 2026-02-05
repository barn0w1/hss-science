import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';
import { useChatStore } from '@/features/chat/state';
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Space } from '@/features/chat/types';

export const SpaceListPage = () => {
  const navigate = useNavigate();
  const { rooms, isLoading, error, fetchRooms } = useChatStore();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  // Spaceのみをフィルタリング
  const spaces = rooms.filter((room): room is Space => room.type === 'space');

  // ピン留めとlastActiveAtでソート
  const sortedSpaces = [...spaces].sort((a, b) => {
    if (a.isPinned !== b.isPinned) return a.isPinned ? -1 : 1;
    return new Date(b.lastActiveAt).getTime() - new Date(a.lastActiveAt).getTime();
  });

  const handleSpaceClick = (spaceId: string) => {
    navigate(`/chat/space/${spaceId}`);
  };

  return (
    <MainLayout header={<ChatHeader />} sidebar={<ChatSidebar />}>
      <div className="h-full flex flex-col pr-[var(--layout-padding)] pb-[var(--layout-padding)]">
        <div className="layout-content-body h-full flex flex-col rounded-[var(--radius-panel)] overflow-hidden">
          <div className="border-b border-surface-100 px-10 py-6">
            <h1 className="text-2xl font-bold">スペース</h1>
          </div>

          <div className="flex-1 overflow-y-auto px-10 py-6">
            {isLoading && <div>読み込み中...</div>}
            {error && <div className="text-red-500">{error}</div>}

            {!isLoading && !error && (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {sortedSpaces.map((space) => (
                  <button
                    key={space.id}
                    onClick={() => handleSpaceClick(space.id)}
                    className="p-6 bg-surface-50 hover:bg-surface-100 rounded-lg
                               transition-colors text-left relative"
                  >
                    <div className="flex items-start gap-3 mb-3">
                      {space.iconUrl && (
                        <img
                          src={space.iconUrl}
                          alt={space.name}
                          className="w-12 h-12 rounded-lg"
                        />
                      )}
                      <div className="flex-1">
                        <div className="font-medium text-lg">{space.name}</div>
                        {!space.isPublic && <span className="text-xs">🔒 Private</span>}
                      </div>
                    </div>

                    {space.description && (
                      <div className="text-sm text-surface-600 mb-3 line-clamp-2">
                        {space.description}
                      </div>
                    )}

                    <div className="flex items-center justify-between text-xs text-surface-500">
                      <div>{new Date(space.lastActiveAt).toLocaleDateString('ja-JP')}</div>
                      <div className="flex items-center gap-2">
                        {space.isPinned && <span>📌</span>}
                        {space.isMuted && <span>🔇</span>}
                        {space.unreadCount > 0 && (
                          <div className="bg-blue-500 text-white rounded-full px-2 py-1">
                            {space.unreadCount}
                          </div>
                        )}
                      </div>
                    </div>
                  </button>
                ))}

                {sortedSpaces.length === 0 && (
                  <div className="col-span-full text-center py-10 text-surface-500">
                    スペースがありません
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </MainLayout>
  );
};
