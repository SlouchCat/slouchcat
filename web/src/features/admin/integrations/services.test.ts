import type { IntegrationStatus } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { integrationHref } from './services';

describe('integrationHref', () => {
  it('leads each dashboard integration to the view of the service that holds it', () => {
    const expected: Record<IntegrationStatus['key'], string> = {
      tmdb: '/admin/integrations?item=keys',
      tvdb: '/admin/integrations?item=keys',
      youtube: '/admin/integrations?item=keys',
      trakt: '/admin/integrations?item=keys',
      email: '/admin/integrations?item=email',
      telegram: '/admin/integrations?item=telegram',
      push: '/admin/integrations?item=push',
      updateCheck: '/admin/integrations?item=server',
      seerr: '/admin/integrations?item=seerr',
    };
    for (const [key, href] of Object.entries(expected)) {
      expect(integrationHref(key as IntegrationStatus['key']), key).toBe(href);
    }
  });
});
