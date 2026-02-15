import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Flowgenix - AI-Powered Project Flows",
  description:
    "Work smarter with AI-powered project flows. Flowgenix transforms project chaos into structured clarity with smart workflows, predictive timelines and task intelligence.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">{children}</body>
    </html>
  );
}
