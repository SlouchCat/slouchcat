import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
import '@/i18n';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from '@/app/App';
import { createQueryClient } from '@/app/queryClient';
import { createAppRouter } from '@/app/router';
import { registerServiceWorker } from '@/app/serviceWorker';
import { sessionQuery } from '@/app/session';
import { dismissHeroPrepaint, dismissPosterPrepaint, loadSpatialNavigation } from '@/ui';

const container = document.getElementById('root');
if (!container) throw new Error('Missing #root element in index.html');

// Requested now, awaited by the routes with the app header (router.tsx).
void loadSpatialNavigation();

const queryClient = createQueryClient({
  // A 401 on any query means the session ended: forget it and go to the login.
  onUnauthenticated: () => {
    queryClient.setQueryData(sessionQuery.queryKey, null);
    void router.navigate({ to: '/login' });
  },
  // The active profile is gone (deleted elsewhere): read the session again and pick another.
  onProfileRequired: () => {
    void queryClient.invalidateQueries({ queryKey: sessionQuery.queryKey });
    void router.navigate({ to: '/profiles' });
  },
});
const router = createAppRouter(queryClient);
// The images the server painted into the start page's shell never outlive the start page.
router.subscribe('onResolved', ({ toLocation }) => {
  if (toLocation.pathname !== '/') {
    dismissHeroPrepaint();
    dismissPosterPrepaint();
  }
});

createRoot(container).render(
  <StrictMode>
    <App router={router} queryClient={queryClient} />
  </StrictMode>,
);

registerServiceWorker();
