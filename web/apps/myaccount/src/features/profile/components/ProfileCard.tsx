import { User } from 'lucide-react';
import type { Profile } from '../hooks/useProfile';

interface ProfileCardProps {
  profile: Profile;
}

export const ProfileCard = ({ profile }: ProfileCardProps) => {
  const fullName = [profile.given_name, profile.family_name].filter(Boolean).join(' ');

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <div className="flex items-center gap-5">
        {profile.picture ? (
          <img
            src={profile.picture}
            alt=""
            className="w-20 h-20 rounded-full"
            referrerPolicy="no-referrer"
          />
        ) : (
          <div className="w-20 h-20 rounded-full bg-blue-500 flex items-center justify-center text-white text-2xl font-medium">
            {(profile.given_name?.[0] ?? profile.email[0]).toUpperCase()}
          </div>
        )}
        <div>
          {fullName && <h2 className="text-xl font-semibold text-gray-900">{fullName}</h2>}
          <p className="text-sm text-gray-600">{profile.email}</p>
          <div className="mt-1 flex items-center gap-2">
            <User size={14} className="text-gray-400" />
            <span className="text-xs text-gray-500">
              Member since {new Date(profile.created_at).toLocaleDateString()}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};
