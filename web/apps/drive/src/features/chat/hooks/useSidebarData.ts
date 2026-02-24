import { useMemo } from 'react';
import { useChatStore } from '../state';
import type { Room } from '../types';
import { mockUsers } from '../mock'; // 本来は UserStore などから取得

// UIが表示するためだけの「使いやすい」型定義 (ViewModel)
export interface SidebarItemData {
  id: string;
  type: 'space' | 'dm';
  displayName: string;   // UIが決める必要のない、確定した表示名
  iconUrl?: string;      // 確定したアイコンURL
  fallbackInitial: string; // アイコンがない時の文字 (例: "H")
  unreadCount: number;
  isPinned: boolean;
  isActive: boolean;
}

export const useSidebarData = () => {
  const rooms = useChatStore((state) => state.rooms);
  const activeRoomId = useChatStore((state) => state.activeRoomId);
  const isLoading = useChatStore((state) => state.isLoading);

  // データ変換ロジック (Presenter)
  const formatRoom = (room: Room): SidebarItemData => {
    let displayName = 'Unknown';
    let iconUrl: string | undefined = undefined;

    if (room.type === 'space') {
      // Spaceの場合
      displayName = room.name;
      iconUrl = room.iconUrl;
    } else {
      // DMの場合: 相手の名前を解決するロジック
      // (本来はここも UserStore.users.find(...) などで行う)
      if (room.name) {
        displayName = room.name; // グループDMなどで名前がある場合
      } else {
        // 自分(me)以外のメンバーを探して名前を表示
        const partnerId = room.memberIds.find((id) => id !== 'me');
        const partner = partnerId ? mockUsers[partnerId] : null;
        displayName = partner ? partner.name : 'Unknown User';
        iconUrl = partner?.avatarUrl;
      }
    }

    return {
      id: room.id,
      type: room.type,
      displayName,
      iconUrl,
      fallbackInitial: displayName.slice(0, 1).toUpperCase(),
      unreadCount: room.unreadCount,
      isPinned: room.isPinned,
      isActive: room.id === activeRoomId,
    };
  };

  const { spaces, dms } = useMemo(() => {
    // 1. ソート
    const sortedRooms = [...rooms].sort((a, b) => {
      if (a.isPinned !== b.isPinned) return a.isPinned ? -1 : 1;
      return new Date(b.lastActiveAt).getTime() - new Date(a.lastActiveAt).getTime();
    });

    // 2. ViewModelへの変換 (Mapping)
    const formattedRooms = sortedRooms.map(formatRoom);

    // 3. 分割
    return {
      spaces: formattedRooms.filter((r) => r.type === 'space'),
      dms: formattedRooms.filter((r) => r.type === 'dm'),
    };
  }, [rooms, activeRoomId]);

  return { spaces, dms, isLoading };
};