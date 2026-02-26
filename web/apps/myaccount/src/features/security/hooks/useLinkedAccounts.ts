import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import type { components } from '@/shared/api/schema';

export type LinkedAccount = components['schemas']['LinkedAccount'];

export const useLinkedAccounts = () => {
  return useQuery({
    queryKey: ['linked-accounts'],
    queryFn: async () => {
      const { data, error } = await api.GET('/api/v1/linked-accounts');
      if (error) throw error;
      return data;
    },
  });
};
