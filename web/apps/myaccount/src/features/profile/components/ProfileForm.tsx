import { useState } from 'react';
import { Save } from 'lucide-react';
import type { Profile } from '../hooks/useProfile';
import { useUpdateProfile } from '../hooks/useUpdateProfile';

interface ProfileFormProps {
  profile: Profile;
}

export const ProfileForm = ({ profile }: ProfileFormProps) => {
  const [givenName, setGivenName] = useState(profile.given_name);
  const [familyName, setFamilyName] = useState(profile.family_name);
  const [locale, setLocale] = useState(profile.locale);
  const updateProfile = useUpdateProfile();

  const hasChanges =
    givenName !== profile.given_name ||
    familyName !== profile.family_name ||
    locale !== profile.locale;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!hasChanges) return;

    updateProfile.mutate({
      ...(givenName !== profile.given_name && { given_name: givenName }),
      ...(familyName !== profile.family_name && { family_name: familyName }),
      ...(locale !== profile.locale && { locale }),
    });
  };

  return (
    <form onSubmit={handleSubmit} className="bg-white rounded-xl border border-gray-200 p-6">
      <h3 className="text-base font-semibold text-gray-900 mb-4">Personal Information</h3>

      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label htmlFor="given_name" className="block text-sm font-medium text-gray-700 mb-1">
              First name
            </label>
            <input
              id="given_name"
              type="text"
              value={givenName}
              onChange={(e) => setGivenName(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
            />
          </div>
          <div>
            <label htmlFor="family_name" className="block text-sm font-medium text-gray-700 mb-1">
              Last name
            </label>
            <input
              id="family_name"
              type="text"
              value={familyName}
              onChange={(e) => setFamilyName(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
            />
          </div>
        </div>

        <div>
          <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
            Email
          </label>
          <input
            id="email"
            type="email"
            value={profile.email}
            disabled
            className="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500"
          />
          <p className="mt-1 text-xs text-gray-500">Email cannot be changed</p>
        </div>

        <div>
          <label htmlFor="locale" className="block text-sm font-medium text-gray-700 mb-1">
            Locale
          </label>
          <input
            id="locale"
            type="text"
            value={locale}
            onChange={(e) => setLocale(e.target.value)}
            placeholder="en-US"
            pattern="^[a-z]{2}(-[A-Z]{2})?$"
            className="w-full max-w-[200px] rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none"
          />
          <p className="mt-1 text-xs text-gray-500">Language tag (e.g. en, ja, en-US)</p>
        </div>
      </div>

      {updateProfile.isError && (
        <p className="mt-4 text-sm text-red-600">
          Failed to update profile. Please try again.
        </p>
      )}

      {updateProfile.isSuccess && (
        <p className="mt-4 text-sm text-green-600">Profile updated successfully.</p>
      )}

      <div className="mt-6 flex justify-end">
        <button
          type="submit"
          disabled={!hasChanges || updateProfile.isPending}
          className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Save size={16} />
          {updateProfile.isPending ? 'Saving...' : 'Save changes'}
        </button>
      </div>
    </form>
  );
};
