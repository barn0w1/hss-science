import { useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { DeleteAccountDialog } from './DeleteAccountDialog';

export const DangerZone = () => {
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  return (
    <div className="bg-white rounded-xl border border-red-200 p-6">
      <div className="flex items-center gap-2 mb-4">
        <AlertTriangle size={18} className="text-red-600" />
        <h3 className="text-base font-semibold text-red-900">Danger Zone</h3>
      </div>

      <p className="text-sm text-gray-600 mb-4">
        Permanently delete your account and all associated data. This action cannot be undone.
      </p>

      <button
        onClick={() => setShowDeleteDialog(true)}
        className="rounded-lg border border-red-300 bg-white px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2"
      >
        Delete account
      </button>

      <DeleteAccountDialog
        open={showDeleteDialog}
        onClose={() => setShowDeleteDialog(false)}
      />
    </div>
  );
};
