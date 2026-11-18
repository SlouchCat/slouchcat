// Types of seerr-server.mjs for the specs that read its key and users.
export declare const SEERR_API_KEY: string;
export declare const SEERR_VERSION: string;
export declare const USERS: {
  id: number;
  displayName: string;
  email: string;
  userType: number;
  permissions: number;
}[];
export declare function startSeerrServer(options?: {
  port?: number;
  radarrPort?: number;
  sonarrPort?: number;
}): Promise<{ port: number; close: () => Promise<void> }>;
