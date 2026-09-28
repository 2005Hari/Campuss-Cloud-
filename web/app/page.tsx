"use client";

import { ActionsSection } from "@/components/ActionsSection";
import { BackupsSection } from "@/components/BackupsSection";
import { DashboardHeader } from "@/components/DashboardHeader";
import { HealthSection } from "@/components/HealthSection";
import { LoginScreen } from "@/components/LoginScreen";
import { LogsSection } from "@/components/LogsSection";
import { OverviewSection } from "@/components/OverviewSection";
import { useAuth } from "@/lib/auth";

export default function Home() {
  const { credentials, isAuthenticated, ready } = useAuth();

  if (!ready) return null;
  if (!isAuthenticated || !credentials) return <LoginScreen />;

  return (
    <div className="flex min-h-screen flex-col">
      <DashboardHeader />
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-6 px-4 py-6">
        <ActionsSection credentials={credentials} />
        <OverviewSection credentials={credentials} />
        <HealthSection credentials={credentials} />
        <BackupsSection credentials={credentials} />
        <LogsSection credentials={credentials} />
      </main>
    </div>
  );
}
