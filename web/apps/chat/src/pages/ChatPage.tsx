import { useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';
import { useChatStore } from '@/features/chat/state';

export const ChatPage = () => {
  const { id } = useParams<{ id: string }>();
  const setActiveRoom = useChatStore((state) => state.setActiveRoom);

  // Sync URL params with chat store
  useEffect(() => {
    if (id) {
      setActiveRoom(id);
    }

    // Cleanup: clear active room when leaving the page
    return () => {
      setActiveRoom(null);
    };
  }, [id, setActiveRoom]);

  return (
    <MainLayout
      header={<ChatHeader />}
      sidebar={<ChatSidebar />}
    >
      <ChatContent />
    </MainLayout>
  );
};