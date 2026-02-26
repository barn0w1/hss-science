import { LinkedAccountsList } from '@/features/security/components/LinkedAccountsList';
import { SessionsList } from '@/features/security/components/SessionsList';

export const SecurityPage = () => {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Security</h1>
        <p className="text-sm text-gray-600 mt-1">
          Manage your linked accounts and active sessions.
        </p>
      </div>
      <LinkedAccountsList />
      <SessionsList />
    </div>
  );
};
