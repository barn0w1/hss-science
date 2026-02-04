import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/Chat';
import { useState } from 'react';

export const ChatPage = () => {
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);

  return (
    <MainLayout
      header={<ChatHeader />}
      sidebar={
        <ChatSidebar
          isSidebarOpen={isSidebarOpen}
          onToggleSidebar={() => setIsSidebarOpen((prev) => !prev)}
        />
      }
      isSidebarOpen={isSidebarOpen}
    >
      <ChatContent />
    </MainLayout>
  );
};