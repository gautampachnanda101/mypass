import type { Metadata } from "next";
import "./globals.css";
import { VaultProvider } from "./components/VaultProvider";

export const metadata: Metadata = {
  title: "MyPass",
  description: "Self-hosted password manager",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <VaultProvider>{children}</VaultProvider>
      </body>
    </html>
  );
}
