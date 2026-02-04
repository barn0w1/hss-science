// features/chat/api/index.ts
import type { Room, Message } from '../types';
import { MockChatRepository } from './mock';
import { RestChatRepository } from './rest';

// Repository
// バックエンドがあろうがなかろうが、このメソッドがあることは絶対保証するということ
export interface ChatRepository {
  getRooms(): Promise<Room[]>;
  getMessages(roomId: string): Promise<Message[]>;
  sendMessage(roomId: string, content: string): Promise<Message>;
}

// ■ 2. 環境変数による切り替え (Dependency Injection)
const USE_MOCK = import.meta.env.VITE_USE_MOCK === 'true';

// アプリ全体で使うインスタンスをここで決定してexportする
export const chatRepository: ChatRepository = USE_MOCK 
  ? new MockChatRepository() 
  : new RestChatRepository();