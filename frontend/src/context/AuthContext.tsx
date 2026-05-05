import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';

const TOKEN_KEY = 'netgazer-token';
const USER_KEY = 'netgazer-user';

interface AuthState {
  token: string | null;
  username: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthState>({
  token: null,
  username: null,
  login: async () => {},
  logout: () => {},
  isAuthenticated: false,
});

export function AuthContextProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => {
    try { return localStorage.getItem(TOKEN_KEY) || null; } catch { return null; }
  });
  const [username, setUsername] = useState<string | null>(() => {
    try { return localStorage.getItem(USER_KEY) || null; } catch { return null; }
  });

  const login = useCallback(async (uname: string, password: string) => {
    const BASE = `${import.meta.env.BASE_URL}api`;
    const res = await fetch(`${BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: uname, password }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Invalid credentials' }));
      throw new Error(err.error || 'Invalid credentials');
    }
    const data = await res.json();
    setToken(data.token);
    setUsername(data.username || uname);
    try {
      localStorage.setItem(TOKEN_KEY, data.token);
      localStorage.setItem(USER_KEY, data.username || uname);
    } catch { /* ignore */ }
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setUsername(null);
    try {
      localStorage.removeItem(TOKEN_KEY);
      localStorage.removeItem(USER_KEY);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    if (!token) return;
    try {
      const payload = JSON.parse(atob(token.split('.')[1]));
      if (payload.exp && payload.exp * 1000 < Date.now()) {
        logout();
      }
    } catch { logout(); }
  }, [token, logout]);

  return (
    <AuthContext.Provider value={{ token, username, login, logout, isAuthenticated: !!token }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
