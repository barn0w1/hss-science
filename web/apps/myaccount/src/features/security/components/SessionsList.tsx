import { useState } from 'react';
import { Monitor, Trash2 } from 'lucide-react';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { useSessions, type Session } from '../hooks/useSessions';
import { useRevokeSession } from '../hooks/useRevokeSession';
import { LoadingSpinner } from '@/shared/ui/LoadingSpinner';

export const SessionsList = () => {
  const { data, isLoading, isError } = useSessions();
  const revokeSession = useRevokeSession();
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null);

  if (isLoading) return <LoadingSpinner />;
  if (isError) return <p className="text-sm text-red-600">Failed to load sessions.</p>;

  const sessions = data?.sessions ?? [];

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <h3 className="text-base font-semibold text-gray-900 mb-4">Active Sessions</h3>
      <p className="text-sm text-gray-600 mb-4">
        Devices and applications currently signed in to your account.
      </p>

      {sessions.length === 0 ? (
        <p className="text-sm text-gray-500">No active sessions.</p>
      ) : (
        <ul className="divide-y divide-gray-100">
          {sessions.map((session: Session) => (
            <li key={session.session_id} className="flex items-center justify-between py-3">
              <div className="flex items-center gap-3">
                <Monitor size={18} className="text-gray-400" />
                <div>
                  <p className="text-sm font-medium text-gray-900">{session.client_id}</p>
                  <p className="text-xs text-gray-500">
                    Created {new Date(session.created_at).toLocaleDateString()}
                    {' · '}
                    Expires {new Date(session.expires_at).toLocaleDateString()}
                  </p>
                  <p className="text-xs text-gray-400">{session.scopes.join(', ')}</p>
                </div>
              </div>
              <button
                onClick={() => setRevokeTarget(session.session_id)}
                className="flex items-center gap-1 text-xs text-gray-500 hover:text-red-600 transition-colors"
              >
                <Trash2 size={14} />
                Revoke
              </button>
            </li>
          ))}
        </ul>
      )}

      {revokeSession.isError && (
        <p className="mt-3 text-sm text-red-600">Failed to revoke session.</p>
      )}

      <ConfirmDialog
        open={revokeTarget !== null}
        title="Revoke session"
        description="This will sign out the selected session. The device will need to sign in again."
        confirmLabel="Revoke"
        variant="danger"
        isPending={revokeSession.isPending}
        onConfirm={() => {
          if (revokeTarget) {
            revokeSession.mutate(revokeTarget, {
              onSuccess: () => setRevokeTarget(null),
            });
          }
        }}
        onCancel={() => setRevokeTarget(null)}
      />
    </div>
  );
};
