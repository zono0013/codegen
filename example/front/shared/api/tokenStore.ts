/**
 * メモリ内でトークンを安全に管理するストア
 * XSS攻撃からトークンを保護するため、localStorageの代わりに使用
 * Route Handler経由でHttpOnly Cookieを発行し、WebSocket認証用に同期する
 */
class TokenStore {
  private accessToken: string | null = null;
  private _syncPromise: Promise<void> = Promise.resolve();

  /**
   * Cookie同期を直列化する
   * 前回の同期完了を待ってから次を発行し、発行順を保証する
   */
  private enqueueCookieSync(token: string | null): void {
    this._syncPromise = this._syncPromise
      .catch(() => undefined)
      .then(() => this.syncCookie(token));
  }

  /**
   * アクセストークンを設定
   * WebSocket接続時の認証用にCookieにも同期する
   */
  setAccessToken(token: string | null): void {
    this.accessToken = token;
    this.enqueueCookieSync(token);
  }

  /**
   * アクセストークンを取得
   */
  getAccessToken(): string | null {
    return this.accessToken;
  }

  /**
   * トークンをクリア
   */
  clear(): void {
    this.accessToken = null;
    this.enqueueCookieSync(null);
  }

  /**
   * Cookie同期の完了を待つ
   * WebSocket接続前にCookieがセットされていることを保証するために使用
   */
  waitForSync(): Promise<void> {
    return this._syncPromise;
  }

  /**
   * WebSocket認証用にaccess_tokenをCookieへ同期する
   * Route Handler (/api/auth/sync-token) 経由でHttpOnly Cookieを発行する
   */
  private async syncCookie(token: string | null): Promise<void> {
    if (typeof window === 'undefined') return;

    try {
      const response = await fetch('/api/auth/sync-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
    } catch (error) {
      throw new Error(
        `Failed to sync access_token cookie: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }
}

// シングルトンインスタンス
export const tokenStore = new TokenStore();
