import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "MyPass – Free Password Manager",
  description: "Free password manager with client-side AES-256 encryption. Your passwords never leave your device.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col font-sans">{children}</body>
    </html>
  );
}

