import { useProfile } from '@/features/profile/hooks/useProfile';
import { ProfileCard } from '@/features/profile/components/ProfileCard';
import { ProfileForm } from '@/features/profile/components/ProfileForm';
import { LoadingSpinner } from '@/shared/ui/LoadingSpinner';

export const ProfilePage = () => {
  const { data: profile, isLoading, isError } = useProfile();

  if (isLoading) return <LoadingSpinner />;

  if (isError || !profile) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 p-6">
        <p className="text-sm text-red-600">Failed to load profile. Please try again.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Profile</h1>
        <p className="text-sm text-gray-600 mt-1">Your personal information and account details.</p>
      </div>
      <ProfileCard profile={profile} />
      <ProfileForm profile={profile} />
    </div>
  );
};
