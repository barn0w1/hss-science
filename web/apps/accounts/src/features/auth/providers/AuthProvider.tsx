import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useAccountsServiceRefreshToken, useAccountsServiceLogout } from '@hss-science/api';
import { setAccessToken } from '../../../lib/axios';

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (token: string) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  const refreshTokenMutation = useAccountsServiceRefreshToken();
  const logoutMutation = useAccountsServiceLogout();

  useEffect(() => {
    const initAuth = async () => {
      try {
        // Attempt silent refresh
        const response = await refreshTokenMutation.mutateAsync({ data: {} });
        if (response.access_token) {
          setAccessToken(response.access_token);
          setIsAuthenticated(true);
        }
      } catch (error) {
        // Silent refresh failed - user is not logged in
        // We don't log error to console to keep it clean, as this is expected for new users
        setAccessToken(null);
        setIsAuthenticated(false);
      } finally {
        setIsLoading(false);
      }
    };

    initAuth();
  }, []);

  const login = (token: string) => {
    setAccessToken(token);
    setIsAuthenticated(true);
  };

  const logout = async () => {
    try {
        await logoutMutation.mutateAsync({ data: {} });
    } catch (e) {
        console.error("Logout failed", e);
    }
    setAccessToken(null);
    setIsAuthenticated(false);
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
