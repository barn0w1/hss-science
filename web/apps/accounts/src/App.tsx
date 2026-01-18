import { Routes, Route, Navigate } from 'react-router-dom';
import { LoginPage } from './features/auth/pages/LoginPage';
import { CallbackPage } from './features/auth/pages/CallbackPage';

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<CallbackPage />} />
      
      {/* 認証後のページ (仮) */}
      <Route path="/" element={
        <div className="p-8">
          <h1>Dashboard</h1>
          <p>Welcome! Token is in localStorage.</p>
        </div>
      } />

      {/* その他はLoginへ */}
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  );
}

export default App;