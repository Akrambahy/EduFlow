import {
  createContext,
  useContext,
  useState,
  useCallback,
  useMemo,
} from "react";
import { ROLES } from "../utils/constants";
import { demoUsers } from "../data/users";

const AuthContext = createContext(null);

/**
 * AuthProvider wraps the app and provides authentication state.
 *
 * MOCK BEHAVIOUR (demo mode):
 *   When no backend is connected the provider uses demoUsers from
 *   src/data/users.js so every role can be tested in the browser.
 *   Replace the login/logout bodies with real API calls once the
 *   backend auth endpoints are confirmed.
 *
 * The backend is authoritative for authentication — this context
 * only stores client-side session state for UI routing.
 */
export function AuthProvider({ children }) {
  const [user, setUser] = useState(() => {
    const saved = localStorage.getItem("eduflow_user");
    return saved ? JSON.parse(saved) : null;
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const login = useCallback(async (email, password) => {
    setLoading(true);
    setError(null);
    try {
      /* ── DEMO MODE ─────────────────────────────────────────────
       * Matches email against demo users.  Replace with:
       *   const data = await api.post('/auth/login', { email, password });
       *   const user = data.user;
       *   localStorage.setItem('eduflow_token', data.token);
       * ──────────────────────────────────────────────────────── */
      const found = demoUsers.find(
        (u) => u.email.toLowerCase() === email.toLowerCase(),
      );
      if (!found) throw new Error("Invalid email or password");

      // Simulate network delay
      await new Promise((r) => setTimeout(r, 600));

      localStorage.setItem("eduflow_user", JSON.stringify(found));
      setUser(found);
      return found;
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem("eduflow_user");
    localStorage.removeItem("eduflow_token");
    setUser(null);
  }, []);

  const switchRole = useCallback(
    (role) => {
      if (!user) return;
      const target = demoUsers.find((u) => u.role === role);
      if (target) {
        localStorage.setItem("eduflow_user", JSON.stringify(target));
        setUser(target);
      }
    },
    [user],
  );

  const value = useMemo(
    () => ({
      user,
      loading,
      error,
      isAuthenticated: !!user,
      login,
      logout,
      switchRole,
    }),
    [user, loading, error, login, logout, switchRole],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export default AuthContext;
