import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Don't let `next dev` (re)generate AGENTS.md/CLAUDE.md in this
  // directory on every dev-server start.
  agentRules: false,
};

export default nextConfig;
