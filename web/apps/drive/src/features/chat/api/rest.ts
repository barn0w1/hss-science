// src/features/chat/api/rest.ts
import type { ChatRepository } from './index';
import type { Room } from '../types';

export class RestChatRepository implements ChatRepository {
  async getRooms(): Promise<Room[]> {
    throw new Error('Not implemented yet');
    // 将来的に: return apiClient.get('/rooms').then(res => res.data);
  }
}