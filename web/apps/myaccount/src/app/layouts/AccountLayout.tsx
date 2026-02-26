import { Outlet, NavLink } from 'react-router-dom';
import { User, Shield, Settings, LogOut } from 'lucide-react';
import { useSession } from '@/features/auth/hooks/useSession';
import { useLogout } from '@/features/auth/hooks/useLogout';
import { LoadingSpinner } from '@/shared/ui/LoadingSpinner';

export const AccountLayout = () => {
  const { data: session, isLoading, isError } = useSession();
  const logout = useLogout();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <LoadingSpinner />
      </div>
    );
  }

  if (isError || !session?.authenticated) {
    window.location.href = '/auth/login?return_to=' + encodeURIComponent(window.location.pathname);
    return null;
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-6 h-16 flex items-center justify-between">
        <NavLink to="/" className="text-xl font-semibold text-gray-900 hover:text-gray-700">
          My Account
        </NavLink>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-600">{session.email}</span>
          {session.picture ? (
            <img
              src={session.picture}
              alt=""
              className="w-8 h-8 rounded-full"
              referrerPolicy="no-referrer"
            />
          ) : (
            <div className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-medium">
              {(session.given_name?.[0] ?? session.email[0]).toUpperCase()}
            </div>
          )}
          <button
            onClick={() => logout.mutate()}
            className="text-sm text-gray-500 hover:text-gray-700 flex items-center gap-1"
            disabled={logout.isPending}
          >
            <LogOut size={16} />
            Sign out
          </button>
        </div>
      </header>

      <div className="max-w-5xl mx-auto px-6 py-8 flex gap-8">
        {/* Sidebar Navigation */}
        <nav className="w-56 flex-shrink-0 space-y-1">
          <SidebarLink to="/profile" icon={<User size={18} />} label="Profile" />
          <SidebarLink to="/security" icon={<Shield size={18} />} label="Security" />
          <SidebarLink to="/account" icon={<Settings size={18} />} label="Account" />
        </nav>

        {/* Content Area */}
        <main className="flex-1 min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  );
};

const SidebarLink = ({ to, icon, label }: { to: string; icon: React.ReactNode; label: string }) => (
  <NavLink
    to={to}
    className={({ isActive }) =>
      `flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
        isActive
          ? 'bg-blue-50 text-blue-700'
          : 'text-gray-700 hover:bg-gray-100'
      }`
    }
  >
    {icon}
    {label}
  </NavLink>
);
