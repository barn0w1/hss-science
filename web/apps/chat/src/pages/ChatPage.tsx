import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';

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