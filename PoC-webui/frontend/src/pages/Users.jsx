import React from 'react';
import { UserPlus, Mail, Shield, MoreHorizontal, Circle, Key } from 'lucide-react';

const Users = () => {
  const users = [
    { id: 1, name: 'Alessandro Rossi', email: 'a.rossi@songbird.io', role: 'Super_Admin', status: 'Online', lastActive: 'Now' },
    { id: 2, name: 'Elena Bianchi', email: 'e.bianchi@songbird.io', role: 'Security_Analyst', status: 'Offline', lastActive: '2h ago' },
    { id: 3, name: 'Marco Verdi', email: 'm.verdi@songbird.io', role: 'ReadOnly_Viewer', status: 'Online', lastActive: '15m ago' },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Administrator Management</h1>
          <p className="text-sm text-slate-500">Configure administrative accounts and access profiles.</p>
        </div>
        <button className="bg-slate-900 hover:bg-slate-800 text-white px-4 py-2 rounded text-sm font-medium flex items-center gap-2 transition-colors">
          <UserPlus size={16} />
          <span>Create New</span>
        </button>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-50 text-slate-500 font-bold uppercase text-[10px] tracking-wider">
            <tr>
              <th className="px-6 py-3 border-b border-slate-200">User / Email</th>
              <th className="px-6 py-3 border-b border-slate-200">Access Profile</th>
              <th className="px-6 py-3 border-b border-slate-200">Status</th>
              <th className="px-6 py-3 border-b border-slate-200">Last Session</th>
              <th className="px-6 py-3 border-b border-slate-200"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {users.map((user) => (
              <tr key={user.id} className="hover:bg-slate-50 transition-colors">
                <td className="px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded bg-slate-100 flex items-center justify-center text-slate-600 font-bold text-[10px]">
                      {user.name.split(' ').map(n => n[0]).join('')}
                    </div>
                    <div>
                      <p className="font-bold text-slate-900 leading-tight">{user.name}</p>
                      <p className="text-[10px] text-slate-400">{user.email}</p>
                    </div>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2 text-slate-600">
                    <Key size={14} className="text-slate-400" />
                    <span className="font-medium">{user.role}</span>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2">
                    <div className={`w-2 h-2 rounded-full ${user.status === 'Online' ? 'bg-green-500' : 'bg-slate-300'}`}></div>
                    <span className="font-medium text-slate-700">{user.status}</span>
                  </div>
                </td>
                <td className="px-6 py-4 text-slate-500">{user.lastActive}</td>
                <td className="px-6 py-4 text-right">
                  <button className="text-slate-400 hover:text-slate-600">
                    <MoreHorizontal size={18} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Users;
