// features/chat/api/mock.ts
import type { ChatRepository } from './index';
import type { Room, Message } from '../types';
import { mockRooms, mockMessages } from '../mock'; // 前回作った固定データ

export class MockChatRepository implements ChatRepository {
  async getRooms(): Promise<Room[]> {
    // ネットワーク遅延をシミュレート (0.5秒待つ)
    await new Promise((resolve) => setTimeout(resolve, 500));
    return mockRooms;
  }

  async getMessages(roomId: string): Promise<Message[]> {
    await new Promise((resolve) => setTimeout(resolve, 500));
    // roomIDでフィルタリングするロジックもここで再現
    return mockMessages.filter(m => m.roomId === roomId);
  }

  async sendMessage(roomId: string, content: string): Promise<Message> {
    await new Promise((resolve) => setTimeout(resolve, 300));
    const newMessage: Message = {
      id: `m-${Date.now()}`,
      roomId,
      senderId: 'me',
      content,
      attachments: [],
      createdAt: new Date().toISOString(),
    };
    return newMessage;
  }
}