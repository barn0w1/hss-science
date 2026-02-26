import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import type { components } from '@/shared/api/schema';

export type Profile = components['schemas']['Profile'];

export const useProfile = () => {
  return useQuery({
    queryKey: ['profile'],
    queryFn: async () => {
      const { data, error } = await api.GET('/api/v1/profile');
      if (error) throw error;
      return data;
    },
  });
};
