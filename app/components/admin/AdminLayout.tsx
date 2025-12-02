import { useState } from "react";
import { Link, useLocation } from "react-router";

// Icons as simple SVG components
const Icons = {
  Dashboard: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M4 5a1 1 0 011-1h4a1 1 0 011 1v5a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM14 5a1 1 0 011-1h4a1 1 0 011 1v3a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 16a1 1 0 011-1h4a1 1 0 011 1v3a1 1 0 01-1 1H5a1 1 0 01-1-1v-3zM14 13a1 1 0 011-1h4a1 1 0 011 1v6a1 1 0 01-1 1h-4a1 1 0 01-1-1v-6z" />
    </svg>
  ),
  Article: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  ),
  Users: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  Media: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  ),
  Settings: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  ),
  Menu: () => (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M4 6h16M4 12h16M4 18h16" />
    </svg>
  ),
  Close: () => (
    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M6 18L18 6M6 6l12 12" />
    </svg>
  ),
  ChevronDown: () => (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  ),
  Logout: () => (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
    </svg>
  ),
  External: () => (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth={1.5} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
    </svg>
  ),
};

interface NavItem {
  name: string;
  href: string;
  icon: keyof typeof Icons;
}

const navigation: NavItem[] = [
  { name: "ダッシュボード", href: "/admin", icon: "Dashboard" },
  { name: "記事管理", href: "/admin/articles", icon: "Article" },
  { name: "メンバー", href: "/admin/members", icon: "Users" },
  { name: "メディア", href: "/admin/media", icon: "Media" },
];

const secondaryNav: NavItem[] = [
  { name: "設定", href: "/admin/settings", icon: "Settings" },
];

interface AdminLayoutProps {
  children: React.ReactNode;
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const location = useLocation();

  const isActive = (href: string) => {
    if (href === "/admin") {
      return location.pathname === "/admin";
    }
    return location.pathname.startsWith(href);
  };

  return (
    <div className="min-h-screen bg-neutral-100">
      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <div 
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed inset-y-0 left-0 z-50 w-64 bg-neutral-900 text-white
          transform transition-transform duration-200 ease-out
          lg:translate-x-0
          ${sidebarOpen ? "translate-x-0" : "-translate-x-full"}
        `}
      >
        {/* Logo */}
        <div className="h-16 flex items-center justify-between px-4 border-b border-neutral-800">
          <Link to="/admin" className="flex items-center gap-3">
            <div className="w-8 h-8 bg-white flex items-center justify-center">
              <span className="text-neutral-900 font-bold text-sm">HS</span>
            </div>
            <div>
              <span className="font-semibold text-sm tracking-wide">HSS Science</span>
              <span className="block text-[10px] text-neutral-500 uppercase tracking-widest">Admin</span>
            </div>
          </Link>
          <button
            onClick={() => setSidebarOpen(false)}
            className="lg:hidden p-1 hover:bg-neutral-800"
          >
            <Icons.Close />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navigation.map((item) => {
            const Icon = Icons[item.icon];
            const active = isActive(item.href);
            return (
              <Link
                key={item.name}
                to={item.href}
                className={`
                  flex items-center gap-3 px-3 py-2.5 text-sm font-medium
                  transition-colors duration-150
                  ${active 
                    ? "bg-white text-neutral-900" 
                    : "text-neutral-400 hover:bg-neutral-800 hover:text-white"
                  }
                `}
              >
                <Icon />
                {item.name}
              </Link>
            );
          })}
        </nav>

        {/* Secondary navigation */}
        <div className="px-3 py-4 border-t border-neutral-800">
          {secondaryNav.map((item) => {
            const Icon = Icons[item.icon];
            const active = isActive(item.href);
            return (
              <Link
                key={item.name}
                to={item.href}
                className={`
                  flex items-center gap-3 px-3 py-2.5 text-sm font-medium
                  transition-colors duration-150
                  ${active 
                    ? "bg-white text-neutral-900" 
                    : "text-neutral-400 hover:bg-neutral-800 hover:text-white"
                  }
                `}
              >
                <Icon />
                {item.name}
              </Link>
            );
          })}
          
          {/* View site link */}
          <a
            href="/"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-3 px-3 py-2.5 text-sm font-medium text-neutral-400 hover:bg-neutral-800 hover:text-white transition-colors duration-150 mt-1"
          >
            <Icons.External />
            サイトを見る
          </a>
        </div>

        {/* User section */}
        <div className="p-3 border-t border-neutral-800">
          <div className="flex items-center gap-3 px-3 py-2">
            <div className="w-8 h-8 bg-neutral-700 flex items-center justify-center text-xs font-medium">
              US
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate">ユーザー名</p>
              <p className="text-xs text-neutral-500 truncate">user@example.com</p>
            </div>
            <button className="p-1 text-neutral-400 hover:text-white hover:bg-neutral-800 transition-colors">
              <Icons.Logout />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <div className="lg:pl-64">
        {/* Top bar */}
        <header className="h-16 bg-white border-b border-neutral-200 sticky top-0 z-30">
          <div className="h-full px-4 flex items-center justify-between">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 -ml-2 text-neutral-600 hover:bg-neutral-100"
            >
              <Icons.Menu />
            </button>

            {/* Breadcrumb area - can be customized per page */}
            <div className="hidden lg:block">
              <nav className="text-sm text-neutral-500">
                <span>Admin</span>
              </nav>
            </div>

            {/* Right side actions */}
            <div className="flex items-center gap-2">
              <button className="hidden sm:flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-neutral-600 hover:bg-neutral-100 transition-colors">
                <span>ユーザー名</span>
                <Icons.ChevronDown />
              </button>
            </div>
          </div>
        </header>

        {/* Page content */}
        <main className="p-6">
          {children}
        </main>
      </div>
    </div>
  );
}

// Reusable admin components
export function AdminPageHeader({ 
  title, 
  description,
  actions 
}: { 
  title: string; 
  description?: string;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 className="text-2xl font-semibold text-neutral-900">{title}</h1>
        {description && (
          <p className="mt-1 text-sm text-neutral-500">{description}</p>
        )}
      </div>
      {actions && <div className="flex gap-2">{actions}</div>}
    </div>
  );
}

export function AdminCard({ 
  children, 
  className = "" 
}: { 
  children: React.ReactNode; 
  className?: string;
}) {
  return (
    <div className={`bg-white border border-neutral-200 ${className}`}>
      {children}
    </div>
  );
}

export function AdminButton({
  children,
  variant = "primary",
  size = "md",
  ...props
}: {
  children: React.ReactNode;
  variant?: "primary" | "secondary" | "ghost";
  size?: "sm" | "md" | "lg";
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const baseStyles = "inline-flex items-center justify-center font-medium transition-colors";
  
  const variants = {
    primary: "bg-neutral-900 text-white hover:bg-neutral-800",
    secondary: "bg-white text-neutral-900 border border-neutral-300 hover:bg-neutral-50",
    ghost: "text-neutral-600 hover:bg-neutral-100",
  };
  
  const sizes = {
    sm: "px-3 py-1.5 text-sm",
    md: "px-4 py-2 text-sm",
    lg: "px-6 py-3 text-base",
  };

  return (
    <button 
      className={`${baseStyles} ${variants[variant]} ${sizes[size]}`}
      {...props}
    >
      {children}
    </button>
  );
}
