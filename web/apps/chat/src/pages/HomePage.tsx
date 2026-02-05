import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';
import { useChatStore } from '@/features/chat/state';
import { useEffect } from 'react';

export const HomePage = () => {
  const { rooms, isLoading, error, fetchRooms } = useChatStore();

  useEffect(() => {
    fetchRooms();
  }, [fetchRooms]);

  // 最近のアクティビティを計算
  const recentRooms = rooms
    .sort((a, b) => new Date(b.lastActiveAt).getTime() - new Date(a.lastActiveAt).getTime())
    .slice(0, 5);

  const totalUnread = rooms.reduce((sum, room) => sum + room.unreadCount, 0);

  return (
    <MainLayout header={<ChatHeader />} sidebar={<ChatSidebar />}>
      <div className="h-full flex flex-col pr-[var(--layout-padding)] pb-[var(--layout-padding)]">
        <div className="layout-content-body h-full flex flex-col rounded-[var(--radius-panel)] overflow-hidden p-10">
          <h1 className="text-3xl font-bold mb-6">ホーム</h1>

          {isLoading && <div>読み込み中...</div>}
          {error && <div className="text-red-500">{error}</div>}

          {!isLoading && !error && (
            <>
              <section className="mb-8">
                <h2 className="text-xl font-semibold mb-4">サマリー</h2>
                <div className="grid grid-cols-3 gap-4">
                  <div className="p-4 bg-surface-50 rounded-lg">
                    <div className="text-sm text-surface-600">未読メッセージ</div>
                    <div className="text-2xl font-bold">{totalUnread}</div>
                  </div>
                  <div className="p-4 bg-surface-50 rounded-lg">
                    <div className="text-sm text-surface-600">DM</div>
                    <div className="text-2xl font-bold">
                      {rooms.filter((r) => r.type === 'dm').length}
                    </div>
                  </div>
                  <div className="p-4 bg-surface-50 rounded-lg">
                    <div className="text-sm text-surface-600">Space</div>
                    <div className="text-2xl font-bold">
                      {rooms.filter((r) => r.type === 'space').length}
                    </div>
                  </div>
                </div>
              </section>

              <section>
                <h2 className="text-xl font-semibold mb-4">最近のアクティビティ</h2>
                <div className="space-y-2">
                  {recentRooms.map((room) => (
                    <div
                      key={room.id}
                      className="p-3 bg-surface-50 rounded-lg flex justify-between"
                    >
                      <div>
                        <div className="font-medium">
                          {room.type === 'space' ? room.name : `DM: ${room.id}`}
                        </div>
                        <div className="text-sm text-surface-600">
                          {new Date(room.lastActiveAt).toLocaleString('ja-JP')}
                        </div>
                      </div>
                      {room.unreadCount > 0 && (
                        <div className="bg-blue-500 text-white rounded-full px-2 py-1 text-xs">
                          {room.unreadCount}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </section>
            </>
          )}
        </div>
      </div>
    </MainLayout>
  );
};
