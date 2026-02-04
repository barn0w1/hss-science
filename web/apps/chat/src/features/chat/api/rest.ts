// features/chat/api/rest.ts
/*
import type { ChatRepository } from './index';
import type { Room, Message } from '../types';
import { apiClient } from '@/shared/api/client'; 

export class RestChatRepository implements ChatRepository {
  async getRooms(): Promise<Room[]> {
    const { data } = await apiClient.get<Room[]>('/rooms');
    return data;
  }

  async getMessages(roomId: string): Promise<Message[]> {
    const { data } = await apiClient.get<Message[]>(`/rooms/${roomId}/messages`);
    return data;
  }

  async sendMessage(roomId: string, content: string): Promise<Message> {
    const { data } = await apiClient.post<Message>(`/rooms/${roomId}/messages`, { content });
    return data;
  }
}
*/