import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  ShieldCheck,
  Users,
  Box,
  LogOut,
  ChevronRight
} from 'lucide-react';
import logo from '../assets/logo.png';

const Sidebar = () => {
  const menuItems = [
    { name: 'Dashboard', path: '/', icon: <LayoutDashboard size={18} /> },
    { name: 'IPv4 Policy', path: '/policies', icon: <ShieldCheck size={18} /> },
    { name: 'Policy Objects', path: '/objects', icon: <Box size={18} /> },
    { name: 'User Management', path: '/users', icon: <Users size={18} /> },
  ];

  return (
    <div className="w-60 h-screen bg-white text-slate-600 flex flex-col fixed left-0 top-0 border-r border-slate-200 shadow-sm z-50">
      <div className="p-6 flex items-center gap-3 border-b border-slate-50">
        <div className="w-10 h-10 flex items-center justify-center">
          <img src={logo} alt="Songbird Logo" className="w-full h-full object-contain" />
        </div>
        <span className="text-lg font-bold text-slate-900 tracking-tight">Songbird</span>
      </div>

      <nav className="flex-1 mt-6 px-3">
        <div className="px-3 mb-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">Main Menu</div>
        <ul className="space-y-1">
          {menuItems.map((item) => (
            <li key={item.path}>
              <NavLink
                to={item.path}
                className={({ isActive }) =>
                  `flex items-center justify-between px-3 py-2 rounded transition-all group ${
                    isActive
                      ? 'bg-slate-900 text-white shadow-md'
                      : 'text-slate-500 hover:bg-slate-50 hover:text-slate-900'
                  }`
                }
              >
                {({ isActive }) => (
                  <>
                    <div className="flex items-center gap-3">
                      {item.icon}
                      <span className="text-sm font-medium">{item.name}</span>
                    </div>
                    <ChevronRight size={14} className={`transition-opacity ${isActive ? 'opacity-40' : 'opacity-0 group-hover:opacity-40'}`} />
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="p-4 border-t border-slate-50">
        <div className="bg-slate-50 rounded-lg p-3">
          <div className="flex items-center gap-3 mb-3">
            <div className="w-8 h-8 rounded bg-slate-900 flex items-center justify-center font-bold text-white text-xs">
              AD
            </div>
            <div className="overflow-hidden">
              <p className="text-xs font-bold text-slate-900 truncate">Admin</p>
              <p className="text-[10px] text-slate-400 truncate">admin@songbird.io</p>
            </div>
          </div>
          <button className="w-full flex items-center justify-center gap-2 py-1.5 rounded border border-slate-200 text-[10px] font-bold uppercase tracking-wider hover:bg-white transition-colors">
            <LogOut size={12} /> Sign Out
          </button>
        </div>
      </div>
    </div>
  );
};

const Layout = ({ children }) => {
  return (
    <div className="flex bg-slate-50/50 min-h-screen font-sans antialiased text-slate-900">
      <Sidebar />
      <main className="flex-1 ml-60 min-h-screen">
        <div className="p-8 max-w-[1400px] mx-auto">
          {children}
        </div>
      </main>
    </div>
  );
};

export default Layout;
