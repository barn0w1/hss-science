import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api/client';

export const useLogout = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const { error } = await api.POST('/auth/logout');
      if (error) throw error;
    },
    onSuccess: () => {
      queryClient.clear();
      window.location.href = '/auth/login';
    },
  });
};
