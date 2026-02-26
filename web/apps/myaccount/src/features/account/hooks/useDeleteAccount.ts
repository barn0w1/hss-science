import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api/client';

export const useDeleteAccount = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const { error } = await api.DELETE('/api/v1/account');
      if (error) throw error;
    },
    onSuccess: () => {
      queryClient.clear();
      window.location.href = '/auth/login';
    },
  });
};
