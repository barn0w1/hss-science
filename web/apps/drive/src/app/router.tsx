import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ChatPage } from '@/pages/ChatPage';

export const AppRouter = () => {
  return (
    <BrowserRouter>
      <Routes>
        {/* routes */}
        <Route path="/" element={<ChatPage />} />

        {/* Catch all - redirect to home */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
};
