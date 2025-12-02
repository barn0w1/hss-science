import { Link, NavLink } from "react-router";

const navItems = [
  { to: "/about", label: "About" },
  { to: "/projects", label: "Projects" },
  { to: "/news", label: "News" },
  { to: "/members", label: "Members" },
];

export function Header() {
  return (
    <header className="w-full bg-white sticky top-0 z-50 shadow-sm">
      {/* Top Accent Line */}
      <div className="h-1 bg-gradient-to-r from-ias-blue via-[#004d99] to-ias-blue" />
      
      {/* Main Navigation */}
      <div className="container mx-auto px-6 py-3">
        <div className="flex items-center justify-between">
          {/* Logo Area */}
          <Link to="/" className="group">
            <div className="text-3xl font-serif font-bold text-gray-900 group-hover:text-ias-blue transition-colors tracking-tight">
              HSS Science
            </div>
            <div className="text-sm text-gray-500 tracking-wide mt-0.5 hidden sm:block">
              Hiratsuka Secondary School Science Club
            </div>
          </Link>

          {/* Nav Links */}
          <nav className="hidden md:flex items-center">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `relative text-sm font-semibold tracking-wide px-5 py-2 transition-colors duration-200 group ${
                    isActive
                      ? "text-ias-blue"
                      : "text-gray-600 hover:text-ias-blue"
                  }`
                }
              >
                {({ isActive }) => (
                  <>
                    {item.label}
                    <span 
                      className={`absolute bottom-0 left-1/2 h-0.5 bg-ias-blue rounded-full transition-all duration-200 ease-out ${
                        isActive ? "w-[calc(100%-40px)] -translate-x-1/2" : "w-0 group-hover:w-[calc(100%-40px)] -translate-x-1/2"
                      }`} 
                    />
                  </>
                )}
              </NavLink>
            ))}
          </nav>

          {/* Mobile Menu Button */}
          <button className="md:hidden p-2 text-gray-600 hover:text-ias-blue">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
        </div>
      </div>
    </header>
  );
}
