// Types of trakt-server.mjs for the specs that read its entries.
export declare const TRAKT_CLIENT_ID: string;
export declare const TRAKT_ENTRIES: {
  movies: { tmdb: number; title: string }[];
  shows: { tmdb: number; title: string }[];
};
export declare function startTraktServer(options?: {
  port?: number;
}): Promise<{ port: number; close: () => Promise<void> }>;
