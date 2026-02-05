import { useEffect } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { MainLayout } from '@/app/layouts/MainLayout';
import { ChatContent, ChatHeader, ChatSidebar } from '@/features/chat/components/ChatComponents';
import { useChatStore } from '@/features/chat/state';

export const ChatPage = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const setActiveRoom = useChatStore((state) => state.setActiveRoom);

  // Sync URL params with chat store
  useEffect(() => {
    // If we're on /chat/dm/:id or /chat/space/:id, set the active room
    if (id && (location.pathname.startsWith('/chat/dm/') || location.pathname.startsWith('/chat/space/'))) {
      setActiveRoom(id);
    } else if (location.pathname === '/chat/home') {
      // On home page, clear active room (or set to null)
      setActiveRoom(null);
    }
  }, [id, location.pathname, setActiveRoom]);

  return (
    <MainLayout
      header={<ChatHeader />}
      sidebar={<ChatSidebar />}
    >
      <ChatContent />
    </MainLayout>
  );
};