import { createContext, useContext, useEffect, useState, useRef } from 'react';
import type { ReactNode } from 'react';
import { useMutation } from '@tanstack/react-query';
import { refreshAccessToken, logout as apiLogout, setAccessToken } from '../../../lib/accounts-api';

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean; // 初期化中かどうか
  logout: () => void;
  login: (token: string) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  // 初期値は true。初期化チェックが終わるまで true のままにする。
  const [isLoading, setIsLoading] = useState(true);
  
  const initRef = useRef(false);

  const refreshTokenMutation = useMutation({
    mutationFn: refreshAccessToken,
  });
  const logoutMutation = useMutation({
    mutationFn: apiLogout,
  });

  useEffect(() => {
    if (initRef.current) return;
    initRef.current = true;

    const initAuth = async () => {
      console.log("AuthProvider: Initializing auth...");
      try {
        // Refresh token を使って Access Token を取得を試みる
        // POST ボディは空でよい (Cookie利用)
        const response = await refreshTokenMutation.mutateAsync();
        
        if (response.access_token) {
          console.log("AuthProvider: Refresh success");
          setAccessToken(response.access_token);
          setIsAuthenticated(true);
        } else {
          console.log("AuthProvider: No access token in response");
          setAccessToken(null);
          setIsAuthenticated(false);
        }
      } catch {
        console.log("AuthProvider: Refresh failed (expected if not logged in)");
        setAccessToken(null);
        setIsAuthenticated(false);
      } finally {
        setIsLoading(false);
        console.log("AuthProvider: Initialization done");
      }
    };

    initAuth();
  }, [refreshTokenMutation]); // Mount時のみ

  const login = (token: string) => {
    setAccessToken(token);
    setIsAuthenticated(true);
  };

  const logout = async () => {
    try {
      await logoutMutation.mutateAsync();
    } catch (error) {
      console.error("Logout failed", error);
    }
    setAccessToken(null);
    setIsAuthenticated(false);
    // 必要ならリダイレクトなどは Router 側、あるいはここで window.location.href 等で行う
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, logout, login }}>
      {children}
    </AuthContext.Provider>
  );
};

// eslint-disable-next-line react-refresh/only-export-components
export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
