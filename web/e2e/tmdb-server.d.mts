// Types of tmdb-server.mjs for the specs that store its key and read its titles.
export declare const TMDB_API_KEY: string;
interface TmdbFixture {
  id: number;
  de: string;
  en: string;
  date: string;
}
export declare const MOVIES: TmdbFixture[];
export declare const SHOWS: (TmdbFixture & {
  tvdb: number;
  seasons: { n: number; episodes: number; date: string }[];
})[];
export declare function startTmdbServer(options?: {
  port?: number;
}): Promise<{ port: number; close: () => Promise<void> }>;
