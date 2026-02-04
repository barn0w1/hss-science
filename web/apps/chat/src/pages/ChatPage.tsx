import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/ChatWindow';

export const ChatPage = () => {
  return (
    <MainLayout header={<ChatHeader />} sidebar={<ChatSidebar />}>
      <ChatContent />
    </MainLayout>
  );
};