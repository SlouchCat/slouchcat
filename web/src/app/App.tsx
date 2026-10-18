import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import type { createAppRouter } from './router';

interface AppProps {
  router: ReturnType<typeof createAppRouter>;
  queryClient: QueryClient;
}

export function App({ router, queryClient }: AppProps) {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
