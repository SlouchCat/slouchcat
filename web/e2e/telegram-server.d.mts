// Types of telegram-server.mjs for the specs and the log scan.
export declare const TELEGRAM_BOT_TOKEN: string;
export declare const BOT_USERNAME: string;
export declare const CHAT_ID: number;
export declare function startTelegramServer(options?: {
  port?: number;
}): Promise<{ port: number; close: () => Promise<void> }>;
