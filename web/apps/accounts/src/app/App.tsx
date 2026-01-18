import { QueryClientProvider } from '@tanstack/react-query';
import { queryClient } from '../lib/react-query';
import { AuthProvider } from '../features/auth/hooks/useAuth';
import { AppRouter } from './router';

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <AppRouter />
      </AuthProvider>
    </QueryClientProvider>
  );
}

export default App;
