"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { Credentials } from "./api";

const STORAGE_KEY = "campuscloud.credentials";

interface AuthContextValue {
  credentials: Credentials | null;
  isAuthenticated: boolean;
  login: (creds: Credentials) => void;
  logout: () => void;
  ready: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// Storing the API token in localStorage is a deliberate tradeoff for this
// admin dashboard: it's a client-only Next.js app with no server component
// of its own (that's what makes it trivially deployable to Vercel with
// zero required secrets), so there's nowhere else to keep it. The token
// only grants access to your own campuscloud API instance — treat it like
// any other admin credential, and only open this dashboard on trusted
// devices.
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [credentials, setCredentials] = useState<Credentials | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) setCredentials(JSON.parse(raw));
    } catch {
      // localStorage unavailable (private browsing, etc.) — just start logged out.
    } finally {
      setReady(true);
    }
  }, []);

  const login = useCallback((creds: Credentials) => {
    setCredentials(creds);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(creds));
    } catch {
      // ignore storage failures; session still works for this page load
    }
  }, []);

  const logout = useCallback(() => {
    setCredentials(null);
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch {
      // ignore
    }
  }, []);

  return (
    <AuthContext.Provider
      value={{ credentials, isAuthenticated: credentials !== null, login, logout, ready }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
