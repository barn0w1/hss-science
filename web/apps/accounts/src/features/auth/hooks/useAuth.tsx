import { createContext, useContext, useEffect, useState, useRef } from 'react';
import type { ReactNode } from 'react';
import { useAccountsServiceRefreshToken, useAccountsServiceLogout } from '@hss-science/api';
import { setAccessToken as setGlobalAccessToken } from '@hss-science/api';

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

  // useAccountsServiceRefreshToken は mutation として定義されている (Orvalの設定によるが、添付ファイルでは mutation)
  // または Query かもしれない。添付の accounts-service.ts を見ると、 Orval はデフォルトで useQuery を生成するが、
  // POSTメソッドの場合は useMutation になる。 api.swagger.json で /refresh は POST なので useMutation。
  const refreshTokenMutation = useAccountsServiceRefreshToken();
  const logoutMutation = useAccountsServiceLogout();

  useEffect(() => {
    if (initRef.current) return;
    initRef.current = true;

    const initAuth = async () => {
      console.log("AuthProvider: Initializing auth...");
      try {
        // Refresh token を使って Access Token を取得を試みる
        // POST ボディは空でよい (Cookie利用)
        const response = await refreshTokenMutation.mutateAsync({ data: {} });
        
        if (response.access_token) {
          console.log("AuthProvider: Refresh success");
          setGlobalAccessToken(response.access_token);
          setIsAuthenticated(true);
        } else {
          console.log("AuthProvider: No access token in response");
          setGlobalAccessToken(null);
          setIsAuthenticated(false);
        }
      } catch (error) {
        console.log("AuthProvider: Refresh failed (expected if not logged in)");
        setGlobalAccessToken(null);
        setIsAuthenticated(false);
      } finally {
        setIsLoading(false);
        console.log("AuthProvider: Initialization done");
      }
    };

    initAuth();
  }, []); // Mount時のみ

  const login = (token: string) => {
    setGlobalAccessToken(token);
    setIsAuthenticated(true);
  };

  const logout = async () => {
    try {
      await logoutMutation.mutateAsync({ data: {} });
    } catch (e) {
      console.error("Logout failed", e);
    }
    setGlobalAccessToken(null);
    setIsAuthenticated(false);
    // 必要ならリダイレクトなどは Router 側、あるいはここで window.location.href 等で行う
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, logout, login }}>
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
