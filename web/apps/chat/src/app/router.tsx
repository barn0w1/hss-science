import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ChatPage } from '@/pages/ChatPage';
import { HomePage } from '@/pages/HomePage';
import { DMListPage } from '@/pages/DMListPage';
import { SpaceListPage } from '@/pages/SpaceListPage';

export const AppRouter = () => {
  return (
    <BrowserRouter>
      <Routes>
        {/* Redirect root to /chat/home */}
        <Route path="/" element={<Navigate to="/chat/home" replace />} />

        {/* Chat routes */}
        <Route path="/chat/home" element={<HomePage />} />
        <Route path="/chat/dm" element={<DMListPage />} />
        <Route path="/chat/dm/:id" element={<ChatPage />} />
        <Route path="/chat/space" element={<SpaceListPage />} />
        <Route path="/chat/space/:id" element={<ChatPage />} />

        {/* Catch all - redirect to home */}
        <Route path="*" element={<Navigate to="/chat/home" replace />} />
      </Routes>
    </BrowserRouter>
  );
};
