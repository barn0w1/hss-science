import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';
import { ChatContent, ChatHeader } from '@/features/chat/components/ChatComponents';

export const ChatPage = () => {
  return (
    <MainLayout header={<ChatHeader />}>
      <MainAreaLayout
        left={
          <div className="h-full bg-white rounded-xl">
            Sidebar
          </div>
        }
        right={
          <div className="h-full bg-white rounded-xl">
            <ChatContent />
          </div>
        }
      />
    </MainLayout>
  );
};
