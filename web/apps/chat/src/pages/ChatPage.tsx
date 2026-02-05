import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/Chat';
import { useState } from 'react';

export const ChatPage = () => {
  return (
    <MainLayout
      header={<ChatHeader />}
      sidebar={<ChatSidebar />}
    >
      <ChatContent />
    </MainLayout>
  );
};