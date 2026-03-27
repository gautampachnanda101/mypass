import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  /* Uncomment and set NEXT_PUBLIC_VAULT_API to override the API base URL */
  // env: {
  //   NEXT_PUBLIC_VAULT_API: process.env.NEXT_PUBLIC_VAULT_API ?? "https://vault.local/api",
  // },
};

export default nextConfig;
