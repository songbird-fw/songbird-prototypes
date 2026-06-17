import React, { useState } from 'react';
import {
  Server,
  Globe,
  Settings,
  Plus,
  Search,
  MoreVertical,
  Tag,
  ExternalLink,
  Code2
} from 'lucide-react';

const Objects = () => {
  const [activeTab, setActiveTab] = useState('addresses');

  const addresses = [
    { name: 'LAN_Network', type: 'Subnet', details: '192.168.1.0/24', interface: 'Internal', color: 'bg-blue-500' },
    { name: 'Google_DNS', type: 'FQDN', details: 'dns.google.com', interface: 'WAN', color: 'bg-green-500' },
    { name: 'Web_Server_IP', type: 'IP Range', details: '10.0.1.50 - 10.0.1.60', interface: 'DMZ', color: 'bg-purple-500' },
  ];

  const vips = [
    { name: 'HTTPS_Forward', external: '203.0.113.5', mapped: '10.0.1.55', port: '443', service: 'HTTPS' },
    { name: 'SSH_External', external: '203.0.113.6', mapped: '10.0.1.22', port: '22', service: 'SSH' },
  ];

  const services = [
    { name: 'HTTP', protocol: 'TCP', port: '80', category: 'Web' },
    { name: 'HTTPS', protocol: 'TCP', port: '443', category: 'Web' },
    { name: 'DNS', protocol: 'UDP/TCP', port: '53', category: 'Network' },
    { name: 'SMTP', protocol: 'TCP', port: '25', category: 'Email' },
  ];

  const tabs = [
    { id: 'addresses', name: 'Addresses', icon: <Globe size={18} /> },
    { id: 'vips', name: 'Virtual IPs', icon: <Server size={18} /> },
    { id: 'services', name: 'Services', icon: <Settings size={18} /> },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Policy Objects</h1>
          <p className="text-sm text-slate-500">Define and manage reusable network entities.</p>
        </div>
        <button className="bg-slate-900 hover:bg-slate-800 text-white px-4 py-2 rounded text-sm font-medium flex items-center gap-2 transition-colors">
          <Plus size={16} />
          <span>Create New</span>
        </button>
      </div>

      <div className="bg-white rounded-lg border border-slate-200 shadow-sm">
        <div className="flex border-b border-slate-200">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-6 py-4 text-sm font-medium border-b-2 transition-all ${
                activeTab === tab.id
                  ? 'border-slate-900 text-slate-900 bg-slate-50/50'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:bg-slate-50'
              }`}
            >
              {tab.icon}
              {tab.name}
            </button>
          ))}
        </div>

        <div className="p-0">
          {activeTab === 'addresses' && (
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50 text-slate-500 font-medium">
                <tr>
                  <th className="px-6 py-3 border-b border-slate-200">Name</th>
                  <th className="px-6 py-3 border-b border-slate-200">Type</th>
                  <th className="px-6 py-3 border-b border-slate-200">Details</th>
                  <th className="px-6 py-3 border-b border-slate-200">Interface</th>
                  <th className="px-6 py-3 border-b border-slate-200"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {addresses.map((addr) => (
                  <tr key={addr.name} className="hover:bg-slate-50 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className={`w-2 h-2 rounded-full ${addr.color}`}></div>
                        <span className="font-medium text-slate-900">{addr.name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-slate-600">{addr.type}</td>
                    <td className="px-6 py-4 font-mono text-xs text-slate-500">{addr.details}</td>
                    <td className="px-6 py-4">
                      <span className="px-2 py-1 bg-slate-100 rounded text-[10px] font-bold text-slate-600 uppercase">
                        {addr.interface}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button className="text-slate-400 hover:text-slate-600"><MoreVertical size={16} /></button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {activeTab === 'vips' && (
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50 text-slate-500 font-medium">
                <tr>
                  <th className="px-6 py-3 border-b border-slate-200">Name</th>
                  <th className="px-6 py-3 border-b border-slate-200">External IP</th>
                  <th className="px-6 py-3 border-b border-slate-200">Mapped IP</th>
                  <th className="px-6 py-3 border-b border-slate-200">Port / Service</th>
                  <th className="px-6 py-3 border-b border-slate-200"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {vips.map((vip) => (
                  <tr key={vip.name} className="hover:bg-slate-50 transition-colors">
                    <td className="px-6 py-4 font-medium text-slate-900">{vip.name}</td>
                    <td className="px-6 py-4 text-slate-600 font-mono text-xs">{vip.external}</td>
                    <td className="px-6 py-4 text-slate-600 font-mono text-xs">{vip.mapped}</td>
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <span className="text-slate-900 font-medium">{vip.service}</span>
                        <span className="text-slate-400">({vip.port})</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button className="text-slate-400 hover:text-slate-600"><MoreVertical size={16} /></button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {activeTab === 'services' && (
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50 text-slate-500 font-medium">
                <tr>
                  <th className="px-6 py-3 border-b border-slate-200">Name</th>
                  <th className="px-6 py-3 border-b border-slate-200">Protocol</th>
                  <th className="px-6 py-3 border-b border-slate-200">Port</th>
                  <th className="px-6 py-3 border-b border-slate-200">Category</th>
                  <th className="px-6 py-3 border-b border-slate-200"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {services.map((svc) => (
                  <tr key={svc.name} className="hover:bg-slate-50 transition-colors">
                    <td className="px-6 py-4 font-medium text-slate-900">{svc.name}</td>
                    <td className="px-6 py-4">
                      <span className="px-2 py-0.5 bg-blue-50 text-blue-700 rounded text-[10px] font-bold">
                        {svc.protocol}
                      </span>
                    </td>
                    <td className="px-6 py-4 font-mono text-xs text-slate-500">{svc.port}</td>
                    <td className="px-6 py-4 text-slate-600">{svc.category}</td>
                    <td className="px-6 py-4 text-right">
                      <button className="text-slate-400 hover:text-slate-600"><MoreVertical size={16} /></button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
};

export default Objects;
