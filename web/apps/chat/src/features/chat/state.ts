import { create } from 'zustand';
import type { Room } from './types';
import { chatRepository } from './api'; // Interface

interface ChatState {
  // Data State
  rooms: Room[];
  activeRoomId: string | null;

  // UI Status
  isLoading: boolean;
  error: string | null;

  // Actions
  // 部屋一覧を取得する
  fetchRooms: () => Promise<void>;
  
  // 選択中の部屋を変更する
  setActiveRoom: (roomId: string) => void;

  // 部屋の状態を更新する (ピン留めやミュートなどのOptimistic UI用)
  updateRoom: (roomId: string, updates: Partial<Room>) => void;
}

export const useChatStore = create<ChatState>((set, get) => ({
  rooms: [],
  activeRoomId: null,
  isLoading: false,
  error: null,

  fetchRooms: async () => {
    // 1. ローディング開始
    set({ isLoading: true, error: null });

    try {
      // 2. Repositoryからデータ取得 (ここで0.8秒待たされる)
      const rooms = await chatRepository.getRooms();

      // 3. データセット
      set({ rooms, isLoading: false });

      // (オプション) もしアクティブな部屋が未選択なら、最初の部屋を選択しておく
      const currentActive = get().activeRoomId;
      if (!currentActive && rooms.length > 0) {
        set({ activeRoomId: rooms[0].id });
      }

    } catch (err) {
      console.error('Failed to fetch rooms:', err);
      set({ 
        error: 'Failed to load rooms.', 
        isLoading: false 
      });
    }
  },

  setActiveRoom: (roomId) => {
    set({ activeRoomId: roomId });
  },

  updateRoom: (roomId, updates) => {
    set((state) => ({
      rooms: state.rooms.map((room) =>
        room.id === roomId ? { ...room, ...updates } as Room : room
      ),
    }));
  },
}));