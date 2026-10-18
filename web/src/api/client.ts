import { createApiClient } from '@slouchcat/api-client';

/** Shared same-origin API client; the dev server proxies /api to the Go server. */
export const api = createApiClient();
