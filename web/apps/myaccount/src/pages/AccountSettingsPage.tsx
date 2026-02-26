import { DangerZone } from '@/features/account/components/DangerZone';

export const AccountSettingsPage = () => {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Account</h1>
        <p className="text-sm text-gray-600 mt-1">
          Account-level settings and data management.
        </p>
      </div>
      <DangerZone />
    </div>
  );
};
