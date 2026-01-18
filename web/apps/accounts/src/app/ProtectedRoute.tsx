import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../features/auth/providers/AuthProvider';

export const ProtectedRoute = () => {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return <div className="flex h-screen items-center justify-center text-gray-500">Loading auth state...</div>;
  }

  return isAuthenticated ? <Outlet /> : <Navigate to="/login" replace />;
};
