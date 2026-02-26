import { useState } from 'react';
import { Unlink } from 'lucide-react';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { useLinkedAccounts } from '../hooks/useLinkedAccounts';
import { useUnlinkAccount } from '../hooks/useUnlinkAccount';
import { LoadingSpinner } from '@/shared/ui/LoadingSpinner';

export const LinkedAccountsList = () => {
  const { data, isLoading, isError } = useLinkedAccounts();
  const unlinkAccount = useUnlinkAccount();
  const [unlinkTarget, setUnlinkTarget] = useState<string | null>(null);

  if (isLoading) return <LoadingSpinner />;
  if (isError) return <p className="text-sm text-red-600">Failed to load linked accounts.</p>;

  const accounts = data?.linked_accounts ?? [];
  const canUnlink = accounts.length > 1;

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <h3 className="text-base font-semibold text-gray-900 mb-4">Linked Accounts</h3>
      <p className="text-sm text-gray-600 mb-4">
        Identity providers linked to your account for sign-in.
      </p>

      {accounts.length === 0 ? (
        <p className="text-sm text-gray-500">No linked accounts found.</p>
      ) : (
        <ul className="divide-y divide-gray-100">
          {accounts.map((account) => (
            <li key={account.id} className="flex items-center justify-between py-3">
              <div>
                <p className="text-sm font-medium text-gray-900 capitalize">{account.provider}</p>
                <p className="text-xs text-gray-500">
                  Linked {new Date(account.linked_at).toLocaleDateString()}
                </p>
              </div>
              {canUnlink && (
                <button
                  onClick={() => setUnlinkTarget(account.id)}
                  className="flex items-center gap-1 text-xs text-gray-500 hover:text-red-600 transition-colors"
                >
                  <Unlink size={14} />
                  Unlink
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {unlinkAccount.isError && (
        <p className="mt-3 text-sm text-red-600">
          Failed to unlink account. You must have at least one linked account.
        </p>
      )}

      <ConfirmDialog
        open={unlinkTarget !== null}
        title="Unlink account"
        description="Are you sure you want to unlink this identity provider? You will no longer be able to sign in with it."
        confirmLabel="Unlink"
        variant="danger"
        isPending={unlinkAccount.isPending}
        onConfirm={() => {
          if (unlinkTarget) {
            unlinkAccount.mutate(unlinkTarget, {
              onSuccess: () => setUnlinkTarget(null),
            });
          }
        }}
        onCancel={() => setUnlinkTarget(null)}
      />
    </div>
  );
};
