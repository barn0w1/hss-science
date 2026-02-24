// src/features/chat/mock.ts
import type { User, Room, Space, DirectMessage } from './types';

// ------------------------------------------------------------------
// Users
// ------------------------------------------------------------------
export const currentUser: User = {
  id: 'me',
  name: 'Kiuchi',
  avatarUrl: 'https://ui-avatars.com/api/?name=Kiuchi&background=0ea5e9&color=fff',
  status: 'online',
  bio: 'Frontend Engineer',
};

export const mockUsers: Record<string, User> = {
  'u1': {
    id: 'u1',
    name: 'Finland',
    avatarUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    status: 'dnd', // DND (Do Not Disturb)
  },
  'u2': {
    id: 'u2',
    name: 'Kawasaki',
    avatarUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    status: 'offline',
  },
  'u3': {
    id: 'u3',
    name: 'Yamamoto',
    status: 'online',
    // avatarUrlなしのテスト用
  },
};

// ------------------------------------------------------------------
// Spaces Data
// ------------------------------------------------------------------
const spaces: Space[] = [
  {
    type: 'space',
    id: 's1',
    name: 'London.jp',
    description: 'General announcements and casual chat',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    // UI State
    unreadCount: 0,
    isPinned: true, // ピン留めテスト
    isMuted: false,
    lastActiveAt: new Date().toISOString(),
  },
  {
    type: 'space',
    id: 's2',
    name: 'Drive Development',
    description: '開発進捗の共有',
    isPublic: false, // 鍵付きアイコンテスト用
    unreadCount: 5,  // 未読バッジテスト用
    isPinned: false,
    isMuted: false,
    lastActiveAt: new Date(Date.now() - 3600000).toISOString(), // 1時間前
  },
  {
    type: 'space',
    id: 's3',
    name: 'Music Lounge',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    unreadCount: 0,
    isPinned: false,
    isMuted: true, // ミュートアイコンテスト用
    lastActiveAt: new Date(Date.now() - 86400000).toISOString(), // 1日前
  },
  {
    type: 'space',
    id: 's4',
    name: 'London.jp',
    description: 'General announcements and casual chat',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    // UI State
    unreadCount: 0,
    isPinned: true, // ピン留めテスト
    isMuted: false,
    lastActiveAt: new Date().toISOString(),
  },
  {
    type: 'space',
    id: 's5',
    name: 'Drive Development',
    description: '開発進捗の共有',
    isPublic: false, // 鍵付きアイコンテスト用
    unreadCount: 5,  // 未読バッジテスト用
    isPinned: false,
    isMuted: false,
    lastActiveAt: new Date(Date.now() - 3600000).toISOString(), // 1時間前
  },
  {
    type: 'space',
    id: 's6',
    name: 'Music Lounge',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    unreadCount: 0,
    isPinned: false,
    isMuted: true, // ミュートアイコンテスト用
    lastActiveAt: new Date(Date.now() - 86400000).toISOString(), // 1日前
  },
  {
    type: 'space',
    id: 's7',
    name: 'Microsoft',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    unreadCount: 0,
    isPinned: false,
    isMuted: true, // ミュートアイコンテスト用
    lastActiveAt: new Date(Date.now() - 86400000).toISOString(), // 1日前
  },
  {
    type: 'space',
    id: 's8',
    name: 'Google',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    unreadCount: 0,
    isPinned: false,
    isMuted: true, // ミュートアイコンテスト用
    lastActiveAt: new Date(Date.now() - 86400000).toISOString(), // 1日前
  },
  {
    type: 'space',
    id: 's9',
    name: 'Apple',
    iconUrl: 'https://content.webtarget.dev/minecraft/server-icon.png',
    isPublic: true,
    unreadCount: 0,
    isPinned: false,
    isMuted: true, // ミュートアイコンテスト用
    lastActiveAt: new Date(Date.now() - 86400000).toISOString(), // 1日前
  },
];

// ------------------------------------------------------------------
// DMs Data
// ------------------------------------------------------------------
const dms: DirectMessage[] = [
  {
    type: 'dm',
    id: 'dm1',
    memberIds: ['me', 'u1'],
    unreadCount: 2,
    isPinned: true,
    isMuted: false,
    lastActiveAt: new Date().toISOString(),
  },
  {
    type: 'dm',
    id: 'dm2',
    memberIds: ['me', 'u2'],
    unreadCount: 0,
    isPinned: false,
    isMuted: false,
    lastActiveAt: new Date(Date.now() - 7200000).toISOString(),
  },
  {
    type: 'dm',
    id: 'dm3',
    name: 'Frontend Team',
    memberIds: ['me', 'u1', 'u3'],
    unreadCount: 0,
    isPinned: false,
    isMuted: false,
    lastActiveAt: new Date(Date.now() - 86400000 * 2).toISOString(),
  },
  {
    type: 'dm',
    id: 'dm4',
    memberIds: ['me', 'u1'],
    unreadCount: 2,
    isPinned: true,
    isMuted: false,
    lastActiveAt: new Date().toISOString(),
  },
];

// export all rooms combined
export const mockRooms: Room[] = [...spaces, ...dms];