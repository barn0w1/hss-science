import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import type { components } from '@/shared/api/schema';

export type UpdateProfileRequest = components['schemas']['UpdateProfileRequest'];

export const useUpdateProfile = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (body: UpdateProfileRequest) => {
      const { data, error } = await api.PATCH('/api/v1/profile', { body });
      if (error) throw error;
      return data;
    },
    onSuccess: (data) => {
      queryClient.setQueryData(['profile'], data);
      queryClient.invalidateQueries({ queryKey: ['session'] });
    },
  });
};
