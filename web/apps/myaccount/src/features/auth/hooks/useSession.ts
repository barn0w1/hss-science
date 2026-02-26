import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import type { components } from '@/shared/api/schema';

export type SessionInfo = components['schemas']['SessionInfo'];

export const useSession = () => {
  return useQuery({
    queryKey: ['session'],
    queryFn: async () => {
      const { data, error } = await api.GET('/auth/session');
      if (error) throw error;
      return data;
    },
    retry: false,
  });
};
