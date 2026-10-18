import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
import '@/i18n';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from '@/app/App';
import { createQueryClient } from '@/app/queryClient';
import { createAppRouter } from '@/app/router';

const container = document.getElementById('root');
if (!container) throw new Error('Missing #root element in index.html');

createRoot(container).render(
  <StrictMode>
    <App router={createAppRouter()} queryClient={createQueryClient()} />
  </StrictMode>,
);
