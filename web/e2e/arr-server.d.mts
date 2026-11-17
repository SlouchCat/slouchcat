// Types of arr-server.mjs for the specs that steer the Radarr and Sonarr mocks.
export declare const ARR_API_KEY: string;
export declare function startArrServer(options: {
  kind: 'radarr' | 'sonarr';
  port: number;
}): Promise<{ port: number; close: () => Promise<void> }>;
