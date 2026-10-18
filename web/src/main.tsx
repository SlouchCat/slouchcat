import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
import '@/i18n';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from '@/app/App';
import { createQueryClient } from '@/app/queryClient';
import { createAppRouter } from '@/app/router';
import { sessionQuery } from '@/app/session';
import { initSpatialNavigation } from '@/ui';

const container = document.getElementById('root');
if (!container) throw new Error('Missing #root element in index.html');

initSpatialNavigation();

const queryClient = createQueryClient({
  // A 401 on any query means the session ended: forget it and go to the login.
  onUnauthenticated: () => {
    queryClient.setQueryData(sessionQuery.queryKey, null);
    void router.navigate({ to: '/login' });
  },
});
const router = createAppRouter(queryClient);

createRoot(container).render(
  <StrictMode>
    <App router={router} queryClient={queryClient} />
  </StrictMode>,
);
