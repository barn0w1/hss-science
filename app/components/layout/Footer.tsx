import { Link } from "react-router";

const navItems = [
  { to: "/about", label: "About" },
  { to: "/projects", label: "Projects" },
  { to: "/news", label: "News" },
  { to: "/members", label: "Members" },
];

export function Footer() {
  return (
    <footer className="mt-auto">
      {/* Row 1: Social Icons - Slim */}
      <div className="border-t border-gray-200 py-4 bg-white">
        <div className="container mx-auto px-6 flex justify-end gap-5">
          <a href="#" className="text-gray-700 hover:text-ias-blue transition-colors" aria-label="X (Twitter)">
            <svg className="w-7 h-7" fill="currentColor" viewBox="0 0 24 24"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" /></svg>
          </a>
          <a href="#" className="text-gray-700 hover:text-ias-blue transition-colors" aria-label="Instagram">
            <svg className="w-7 h-7" fill="currentColor" viewBox="0 0 24 24"><path fillRule="evenodd" d="M12.315 2c2.43 0 2.784.013 3.808.06 1.064.049 1.791.218 2.427.465a4.902 4.902 0 0 1 1.772 1.153 4.902 4.902 0 0 1 1.153 1.772c.247.636.416 1.363.465 2.427.048 1.067.06 1.407.06 4.123v.08c0 2.643-.012 2.987-.06 4.043-.049 1.064-.218 1.791-.465 2.427a4.902 4.902 0 0 1-1.153 1.772 4.902 4.902 0 0 1-1.772 1.153c-.636.247-1.363.416-2.427.465-1.067.048-1.407.06-4.123.06h-.08c-2.643 0-2.987-.012-4.043-.06-1.064-.049-1.791-.218-2.427-.465a4.902 4.902 0 0 1-1.772-1.153 4.902 4.902 0 0 1-1.153-1.772c-.247-.636-.416-1.363-.465-2.427-.047-1.024-.06-1.379-.06-3.808v-.63c0-2.43.013-2.784.06-3.808.049-1.064.218-1.791.465-2.427a4.902 4.902 0 0 1 1.153-1.772A4.902 4.902 0 0 1 5.451 2.525c.636-.247 1.363-.416 2.427-.465C8.901 2.013 9.256 2 11.685 2h.63zm-.081 1.802h-.468c-2.456 0-2.784.011-3.807.058-.975.045-1.504.207-1.857.344-.467.182-.8.398-1.15.748-.35.35-.566.683-.748 1.15-.137.353-.3.882-.344 1.857-.047 1.023-.058 1.351-.058 3.807v.468c0 2.456.011 2.784.058 3.807.045.975.207 1.504.344 1.857.182.466.399.8.748 1.15.35.35.683.566 1.15.748.353.137.882.3 1.857.344 1.054.048 1.37.058 4.041.058h.08c2.597 0 2.917-.01 3.96-.058.976-.045 1.505-.207 1.858-.344.466-.182.8-.398 1.15-.748.35-.35.566-.683.748-1.15.137-.353.3-.882.344-1.857.048-1.055.058-1.37.058-4.041v-.08c0-2.597-.01-2.917-.058-3.96-.045-.976-.207-1.505-.344-1.858a3.097 3.097 0 0 0-.748-1.15 3.098 3.098 0 0 0-1.15-.748c-.353-.137-.882-.3-1.857-.344-1.023-.047-1.351-.058-3.807-.058zM12 6.865a5.135 5.135 0 1 1 0 10.27 5.135 5.135 0 0 1 0-10.27zm0 1.802a3.333 3.333 0 1 0 0 6.666 3.333 3.333 0 0 0 0-6.666zm5.338-3.205a1.2 1.2 0 1 1 0 2.4 1.2 1.2 0 0 1 0-2.4z" clipRule="evenodd" /></svg>
          </a>
          <a href="#" className="text-gray-700 hover:text-ias-blue transition-colors" aria-label="YouTube">
            <svg className="w-7 h-7" fill="currentColor" viewBox="0 0 24 24"><path fillRule="evenodd" d="M19.812 5.418c.861.23 1.538.907 1.768 1.768C21.998 8.746 22 12 22 12s0 3.255-.418 4.814a2.504 2.504 0 0 1-1.768 1.768c-1.56.419-7.814.419-7.814.419s-6.255 0-7.814-.419a2.505 2.505 0 0 1-1.768-1.768C2 15.255 2 12 2 12s0-3.255.418-4.814a2.507 2.507 0 0 1 1.768-1.768C5.744 5 11.998 5 11.998 5s6.255 0 7.814.418ZM15.194 12 10 15V9l5.194 3Z" clipRule="evenodd" /></svg>
          </a>
        </div>
      </div>

      {/* Row 2: Main Links & Buttons */}
      <div className="border-t border-gray-200 py-12 bg-gray-50">
        <div className="container mx-auto px-6">
          <div className="flex flex-col md:flex-row justify-between gap-10">
            {/* Links Section */}
            <div className="flex flex-wrap gap-x-16 gap-y-8">
              {/* About */}
              <div>
                <h4 className="text-sm font-bold text-ias-blue uppercase tracking-wider mb-4">About</h4>
                <ul className="space-y-2">
                  <li><Link to="/about" className="text-gray-700 hover:text-ias-blue hover:underline transition-colors text-sm">Our Club</Link></li>
                  <li><Link to="/members" className="text-gray-700 hover:text-ias-blue hover:underline transition-colors text-sm">Members</Link></li>
                </ul>
              </div>
              
              {/* Research */}
              <div>
                <h4 className="text-sm font-bold text-ias-blue uppercase tracking-wider mb-4">Research</h4>
                <ul className="space-y-2">
                  <li><Link to="/projects" className="text-gray-700 hover:text-ias-blue hover:underline transition-colors text-sm">Projects</Link></li>
                </ul>
              </div>
              
              {/* News */}
              <div>
                <h4 className="text-sm font-bold text-ias-blue uppercase tracking-wider mb-4">News</h4>
                <ul className="space-y-2">
                  <li><Link to="/news" className="text-gray-700 hover:text-ias-blue hover:underline transition-colors text-sm">Latest News</Link></li>
                </ul>
              </div>
            </div>

            {/* Buttons Section */}
            <div className="flex flex-col gap-3 min-w-[160px]">
              <Link 
                to="/join" 
                className="border border-gray-400 text-gray-700 px-6 py-2.5 text-sm text-center hover:border-gray-900 hover:text-gray-900 transition-colors"
              >
                Join
              </Link>
              <Link 
                to="/contact" 
                className="border border-gray-400 text-gray-700 px-6 py-2.5 text-sm text-center hover:border-gray-900 hover:text-gray-900 transition-colors"
              >
                Contact
              </Link>
            </div>
          </div>
        </div>
      </div>

      {/* Row 3: Bottom Bar - Slim */}
      <div className="bg-ias-blue text-white py-4">
        <div className="container mx-auto px-6">
          <div className="flex flex-col md:flex-row justify-between items-center gap-3 text-sm">
            {/* Logo */}
            <div className="font-serif text-lg">
              <span className="font-bold">HSS</span> Science
            </div>
            
            {/* Copyright */}
            <p className="text-gray-400 text-xs">
              &copy; {new Date().getFullYear()} HSS Science Club
            </p>
          </div>
        </div>
      </div>
    </footer>
  );
}
