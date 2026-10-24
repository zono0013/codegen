import type { Metadata } from 'next'
import Link from 'next/link'
import { Geist, Geist_Mono } from 'next/font/google'
import './globals.css'
import { Toaster } from '@/shared/ui/sonner'
import { Providers } from './providers'

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
})

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
})

export const metadata: Metadata = {
  title: 'Blog',
  description: 'proto 駆動の Go + Next.js テンプレート',
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="ja"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col">
        <Providers>
          <header className="border-b">
            <nav className="mx-auto flex w-full max-w-5xl items-center gap-4 p-4 text-sm">
              <Link href="/" className="font-semibold">
                Blog
              </Link>
              <Link href="/articles" className="text-muted-foreground hover:text-foreground">
                記事
              </Link>
              <Link href="/users" className="text-muted-foreground hover:text-foreground">
                投稿者
              </Link>
            </nav>
          </header>
          <main className="mx-auto w-full max-w-5xl flex-1">{children}</main>
          <Toaster />
        </Providers>
      </body>
    </html>
  )
}
