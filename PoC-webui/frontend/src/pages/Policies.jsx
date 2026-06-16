import React, { useState } from 'react';
import {
  Shield,
  Plus,
  MoreVertical,
  CheckCircle2,
  XCircle,
  ChevronDown,
  ChevronRight,
  Clock,
  ArrowRight
} from 'lucide-react';

const Policies = () => {
  const [expandedGroups, setExpandedGroups] = useState(['lan_wan', 'wan_lan']);

  const groups = [
    {
      id: 'lan_wan',
      name: 'LAN -> WAN',
      policies: [
        { id: 1, name: 'Web Access', src: 'LAN_Network', dst: 'Any', service: 'HTTPS', action: 'ACCEPT', schedule: 'always', status: 'Active' },
        { id: 3, name: 'DNS Queries', src: 'LAN_Network', dst: 'Google_DNS', service: 'DNS', action: 'ACCEPT', schedule: 'always', status: 'Active' },
      ]
    },
    {
      id: 'wan_lan',
      name: 'WAN -> LAN',
      policies: [
        { id: 2, name: 'Web Server VIP', src: 'Any', dst: 'HTTPS_Forward', service: 'HTTPS', action: 'ACCEPT', schedule: 'always', status: 'Active' },
        { id: 5, name: 'SSH Remote', src: 'Trusted_IPs', dst: 'SSH_External', service: 'SSH', action: 'DENY', schedule: 'work_hours', status: 'Disabled' },
      ]
    }
  ];

  const toggleGroup = (id) => {
    setExpandedGroups(prev =>
      prev.includes(id) ? prev.filter(g => g !== id) : [...prev, id]
    );
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">IPv4 Policy</h1>
          <p className="text-sm text-slate-500">Manage security policies and traffic inspection rules.</p>
        </div>
        <button className="bg-slate-900 hover:bg-slate-800 text-white px-4 py-2 rounded text-sm font-medium flex items-center gap-2 transition-colors">
          <Plus size={16} />
          <span>Create New</span>
        </button>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden">
        <table className="w-full text-left text-sm border-collapse">
          <thead className="bg-slate-50 text-slate-500 font-bold uppercase text-[10px] tracking-wider">
            <tr>
              <th className="px-4 py-3 border-b border-slate-200 w-12">ID</th>
              <th className="px-4 py-3 border-b border-slate-200">Name</th>
              <th className="px-4 py-3 border-b border-slate-200">Source</th>
              <th className="px-4 py-3 border-b border-slate-200">Destination</th>
              <th className="px-4 py-3 border-b border-slate-200">Service</th>
              <th className="px-4 py-3 border-b border-slate-200">Schedule</th>
              <th className="px-4 py-3 border-b border-slate-200">Action</th>
              <th className="px-4 py-3 border-b border-slate-200 text-center">Status</th>
              <th className="px-4 py-3 border-b border-slate-200"></th>
            </tr>
          </thead>
          <tbody>
            {groups.map((group) => (
              <React.Fragment key={group.id}>
                {/* Group Header */}
                <tr
                  className="bg-slate-100/50 cursor-pointer hover:bg-slate-100 transition-colors"
                  onClick={() => toggleGroup(group.id)}
                >
                  <td colSpan={9} className="px-4 py-2 border-y border-slate-200">
                    <div className="flex items-center gap-2 text-slate-900 font-bold">
                      {expandedGroups.includes(group.id) ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                      <span className="text-xs uppercase">{group.name}</span>
                      <span className="text-[10px] text-slate-400 font-medium">({group.policies.length} Policies)</span>
                    </div>
                  </td>
                </tr>
                {/* Policy Rows */}
                {expandedGroups.includes(group.id) && group.policies.map((policy) => (
                  <tr key={policy.id} className="hover:bg-slate-50 transition-colors border-b border-slate-100 last:border-b-0">
                    <td className="px-4 py-3 text-slate-400 font-mono text-xs">{policy.id}</td>
                    <td className="px-4 py-3 font-medium text-slate-700">{policy.name}</td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-1 bg-slate-100 rounded text-[10px] font-bold text-slate-600">{policy.src}</span>
                    </td>
                    <td className="px-4 py-3">
                      <span className="px-2 py-1 bg-slate-100 rounded text-[10px] font-bold text-slate-600">{policy.dst}</span>
                    </td>
                    <td className="px-4 py-3 text-slate-600">
                      <div className="flex items-center gap-1">
                         <span className="px-1.5 py-0.5 bg-blue-50 text-blue-700 rounded text-[10px] font-bold uppercase">{policy.service}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-slate-500">
                      <div className="flex items-center gap-1.5 text-[10px]">
                        <Clock size={12} /> {policy.schedule}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-[10px] font-black px-2 py-1 rounded tracking-tighter ${
                        policy.action === 'ACCEPT' ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600'
                      }`}>
                        {policy.action}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center">
                      <div className="flex justify-center">
                        {policy.status === 'Active' ? (
                          <div className="w-2 h-2 rounded-full bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.5)]"></div>
                        ) : (
                          <div className="w-2 h-2 rounded-full bg-slate-300"></div>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button className="text-slate-400 hover:text-slate-600 transition-colors">
                        <MoreVertical size={16} />
                      </button>
                    </td>
                  </tr>
                ))}
              </React.Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
};

export default Policies;
