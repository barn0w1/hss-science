import { User } from '../auth/types';
import { Room, Message } from './types';

// ログイン中のユーザー（自分）
export const currentUser: User = {
  id: 'me',
  name: 'Kiuchi Developer',
  email: 'dev@hss-science.org',
  avatarUrl: 'https://ui-avatars.com/api/?name=Kiuchi+Dev&background=0ea5e9&color=fff',
  status: 'online',
};

// 他のユーザーたち
export const mockUsers: Record<string, User> = {
  'u1': {
    id: 'u1',
    name: '佐藤 博士',
    email: 'sato@hss-science.org',
    status: 'dnd', // dnd
    avatarUrl: 'https://ui-avatars.com/api/?name=Sato+Dr&background=random',
  },
  'u2': {
    id: 'u2',
    name: '田中 研究員',
    email: 'tanaka@hss-science.org',
    status: 'offline',
    avatarUrl: 'https://ui-avatars.com/api/?name=Tanaka+Res&background=random',
  },
  'u3': {
    id: 'u3',
    name: '鈴木 学生',
    email: 'suzuki@hss-science.org',
    status: 'online',
    // avatarUrlなしの場合のテスト用
  },
};

// チャットルーム一覧
export const mockRooms: Room[] = [
  {
    id: 'room-1',
    name: 'HSS General',
    type: 'channel',
    unreadCount: 0,
    memberIds: ['me', 'u1', 'u2', 'u3'],
    lastMessage: {
      id: 'm100',
      content: '明日のミーティングは10時からです。',
      senderId: 'u1',
      createdAt: new Date(Date.now() - 1000 * 60 * 30).toISOString(), // 30分前
    },
  },
  {
    id: 'room-2',
    name: 'Drive開発プロジェクト',
    type: 'group',
    unreadCount: 3,
    memberIds: ['me', 'u2'],
    lastMessage: {
      id: 'm200',
      content: 'APIの仕様書更新しました確認お願いします！',
      senderId: 'u2',
      createdAt: new Date(Date.now() - 1000 * 60 * 5).toISOString(), // 5分前
    },
  },
  {
    id: 'room-3',
    name: '佐藤 博士',
    type: 'dm',
    unreadCount: 0,
    memberIds: ['me', 'u1'],
    avatarUrl: mockUsers['u1'].avatarUrl,
    lastMessage: {
      id: 'm300',
      content: '承知いたしました。',
      senderId: 'me',
      createdAt: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString(), // 1日前
    },
  },
];

// 特定のルーム内のメッセージ履歴例 (room-1用)
export const mockMessages: Message[] = [
  {
    id: 'msg-1',
    content: 'おはようございます。本日の予定を確認させてください。',
    senderId: 'u3',
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(),
  },
  {
    id: 'msg-2',
    content: 'おはよう。今日はDriveのUI実装を進める予定だよ。',
    senderId: 'me',
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 1.9).toISOString(),
  },
  {
    id: 'msg-3',
    content: '了解です！私もお手伝いできることがあれば言ってください。',
    senderId: 'u3',
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 1.8).toISOString(),
  },
  {
    id: 'msg-4',
    content: 'ありがとう、後でプルリク投げるのでレビュー頼むかも。',
    senderId: 'me',
    createdAt: new Date(Date.now() - 1000 * 60 * 60 * 1.7).toISOString(),
  },
];