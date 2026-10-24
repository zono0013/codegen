import type { NextConfig } from "next";

// ブラウザからの /api/* を Next.js のサーバーがバックエンドに転送する。ブラウザから見て同一オリジンになるので、
// バックエンドに CORS の設定は要らない（Cookie を使う認証を足すときも withCredentials が要らない）。
// BACKEND_URL はサーバー側だけで使う。転送先は next build のときに決まる
const backendUrl = process.env.BACKEND_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${backendUrl}/api/:path*` }];
  },
};

export default nextConfig;
