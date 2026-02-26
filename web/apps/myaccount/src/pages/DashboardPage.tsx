import { NavLink } from 'react-router-dom';
import { User, Shield, Settings, ArrowRight } from 'lucide-react';
import { useSession } from '@/features/auth/hooks/useSession';

export const DashboardPage = () => {
  const { data: session } = useSession();

  const greeting = session?.given_name ? `Welcome, ${session.given_name}` : 'Welcome';

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">{greeting}</h1>
        <p className="text-sm text-gray-600 mt-1">
          Manage your account settings and preferences.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <QuickLink
          to="/profile"
          icon={<User size={24} />}
          title="Profile"
          description="View and edit your personal information"
        />
        <QuickLink
          to="/security"
          icon={<Shield size={24} />}
          title="Security"
          description="Manage linked accounts and sessions"
        />
        <QuickLink
          to="/account"
          icon={<Settings size={24} />}
          title="Account"
          description="Account settings and data management"
        />
      </div>
    </div>
  );
};

const QuickLink = ({
  to,
  icon,
  title,
  description,
}: {
  to: string;
  icon: React.ReactNode;
  title: string;
  description: string;
}) => (
  <NavLink
    to={to}
    className="group bg-white rounded-xl border border-gray-200 p-5 hover:border-blue-300 hover:shadow-sm transition-all"
  >
    <div className="flex items-start justify-between">
      <div className="text-gray-400 group-hover:text-blue-600 transition-colors">{icon}</div>
      <ArrowRight size={16} className="text-gray-300 group-hover:text-blue-500 transition-colors" />
    </div>
    <h3 className="mt-3 text-sm font-semibold text-gray-900">{title}</h3>
    <p className="mt-1 text-xs text-gray-500">{description}</p>
  </NavLink>
);
