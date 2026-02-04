import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Login } from '../features/auth/ui/Login';
import { Callback } from '../features/auth/ui/Callback';
import { ProtectedRoute } from '../features/auth/ui/ProtectedRoute';
import { Profile } from '../features/profile/ui/Profile';

export const AppRouter = () => {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public Routes */}
        <Route path="/login" element={<Login />} />
        <Route path="/callback" element={<Callback />} />
        
        {/* Protected Routes */}
        <Route element={<ProtectedRoute />}>
          <Route path="/" element={<Profile />} />
        </Route>
        
        {/* Catch all */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
};
