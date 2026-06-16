import React from 'react';
import { Shield, Plus, MoreVertical, CheckCircle2, XCircle } from 'lucide-react';

const Policies = () => {
  const policies = [
    { id: 1, name: 'Allow SSH', source: '0.0.0.0/0', destination: 'Internal', port: '22', protocol: 'TCP', action: 'ALLOW', status: 'Active' },
    { id: 2, name: 'Block Malicious IP', source: '45.12.33.0/24', destination: 'Any', port: 'Any', protocol: 'Any', action: 'DROP', status: 'Active' },
    { id: 3, name: 'Web Traffic', source: 'Any', destination: 'DMZ', port: '80, 443', protocol: 'TCP', action: 'ALLOW', status: 'Active' },
    { id: 4, name: 'DB Access', source: '10.0.1.0/24', destination: '10.0.2.15', port: '5432', protocol: 'TCP', action: 'ALLOW', status: 'Disabled' },
    { id: 5, name: 'ICMP Rate Limit', source: 'Any', destination: 'Any', port: 'N/A', protocol: 'ICMP', action: 'LIMIT', status: 'Active' },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Firewall Policies</h1>
          <p className="text-slate-500">Manage your network security rules and traffic flow.</p>
        </div>
        <button className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg flex items-center gap-2 transition-colors">
          <Plus size={20} />
          <span>Add Rule</span>
        </button>
      </div>

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead className="bg-slate-50 border-b border-slate-200">
              <tr>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Rule Name</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Source</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Destination</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Service/Port</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Action</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-4 text-xs font-semibold text-slate-500 uppercase tracking-wider"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {policies.map((policy) => (
                <tr key={policy.id} className="hover:bg-slate-50 transition-colors">
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <Shield size={18} className="text-slate-400" />
                      <span className="font-medium text-slate-900">{policy.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-slate-600">{policy.source}</td>
                  <td className="px-6 py-4 text-sm text-slate-600">{policy.destination}</td>
                  <td className="px-6 py-4 text-sm text-slate-600">
                    <span className="px-2 py-1 bg-slate-100 rounded text-xs font-mono">{policy.protocol}:{policy.port}</span>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`text-xs font-bold px-2.5 py-1 rounded-full ${
                      policy.action === 'ALLOW' ? 'bg-green-100 text-green-700' :
                      policy.action === 'DROP' ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'
                    }`}>
                      {policy.action}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      {policy.status === 'Active' ? (
                        <CheckCircle2 size={16} className="text-green-500" />
                      ) : (
                        <XCircle size={16} className="text-slate-400" />
                      )}
                      <span className={`text-sm ${policy.status === 'Active' ? 'text-slate-900' : 'text-slate-400'}`}>
                        {policy.status}
                      </span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button className="text-slate-400 hover:text-slate-600">
                      <MoreVertical size={20} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default Policies;
