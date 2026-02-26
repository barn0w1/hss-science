import { useState } from 'react';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { useDeleteAccount } from '../hooks/useDeleteAccount';

interface DeleteAccountDialogProps {
  open: boolean;
  onClose: () => void;
}

export const DeleteAccountDialog = ({ open, onClose }: DeleteAccountDialogProps) => {
  const [confirmText, setConfirmText] = useState('');
  const deleteAccount = useDeleteAccount();

  const isConfirmed = confirmText === 'DELETE';

  const handleConfirm = () => {
    if (!isConfirmed) return;
    deleteAccount.mutate();
  };

  const handleCancel = () => {
    setConfirmText('');
    onClose();
  };

  return (
    <ConfirmDialog
      open={open}
      title="Delete your account"
      description="This will permanently delete your account, including all profile data, linked accounts, and active sessions. This cannot be undone."
      confirmLabel="Delete my account"
      variant="danger"
      isPending={deleteAccount.isPending}
      onConfirm={handleConfirm}
      onCancel={handleCancel}
    >
      <div className="mt-4">
        <label htmlFor="confirm-delete" className="block text-sm font-medium text-gray-700 mb-1">
          Type <span className="font-mono font-semibold">DELETE</span> to confirm
        </label>
        <input
          id="confirm-delete"
          type="text"
          value={confirmText}
          onChange={(e) => setConfirmText(e.target.value)}
          className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:ring-1 focus:ring-red-500 outline-none"
          autoComplete="off"
        />
      </div>

      {deleteAccount.isError && (
        <p className="mt-3 text-sm text-red-600">
          Failed to delete account. Please try again.
        </p>
      )}
    </ConfirmDialog>
  );
};
