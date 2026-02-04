// src/features/chat/api/mock.ts
import type { ChatRepository } from './index';
import type { Room } from '../types';
import { mockRooms } from '../mock';

export class MockChatRepository implements ChatRepository {
  async getRooms(): Promise<Room[]> {
    // ネットワーク遅延をシミュレート (0.8秒待つ)
    // これがないと、一瞬で表示されてLoadingステートの確認ができない
    await new Promise((resolve) => setTimeout(resolve, 800));
    
    // データを返す (実際のAPIならここでaxios.getなどが走る)
    return [...mockRooms];
  }
}