// Types of smtp-server.mjs for the specs and the log scan.
export declare const SMTP_USER: string;
export declare const SMTP_PASSWORD: string;
export declare const SMTP_FROM: string;
export declare function startSmtpServer(options?: {
  port?: number;
  httpPort?: number;
}): Promise<{ port: number; httpPort: number; close: () => Promise<void> }>;
