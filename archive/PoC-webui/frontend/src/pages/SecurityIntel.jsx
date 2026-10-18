import React from 'react';
import { ShieldAlert, Globe, Shield } from 'lucide-react';
import DistributionWidget from '../components/widgets/DistributionWidget';
import WorldMapWidget from '../components/widgets/WorldMapWidget';
import Widget from '../components/widgets/Widget';

const SecurityIntel = () => {
  const protocolData = {
    labels: ['TCP', 'UDP', 'ICMP', 'Other'],
    datasets: [{
      data: [618, 112, 45, 12],
      backgroundColor: ['#3b82f6', '#10b981', '#f59e0b', '#64748b'],
      borderWidth: 0,
    }],
  };

  const portData = {
    labels: ['22', '443', '3389', '80', 'Other'],
    datasets: [{
      data: [298, 412, 122, 95, 34],
      backgroundColor: ['#ef4444', '#3b82f6', '#f59e0b', '#10b981', '#64748b'],
      borderWidth: 0,
    }],
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">Security Intelligence</h1>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        <div className="lg:col-span-8">
           <WorldMapWidget />
        </div>
        <div className="lg:col-span-4 space-y-6">
           <DistributionWidget title="Blocked Protocols" icon={Shield} data={protocolData} />
           <DistributionWidget title="Target Ports" icon={ShieldAlert} data={portData} />
        </div>

        <div className="lg:col-span-12">
          <Widget title="Recent Blocked Events" icon={ShieldAlert}>
            <div className="p-0">
               {[
                 { time: '14:45:10', source: '185.122.2.1', dest: '192.168.1.100', port: '3389', proto: 'TCP', reason: 'MALICIOUS' },
                 { time: '14:45:02', source: '45.12.33.19', dest: '192.168.1.1', port: '22', proto: 'TCP', reason: 'GEO-BLOCK' },
                 { time: '14:44:55', source: '103.44.12.1', dest: '192.168.1.5', port: '23', proto: 'TCP', reason: 'PORT-SCAN' },
               ].map((log, i) => (
                 <div key={i} className="flex items-center justify-between px-4 py-2 border-b border-slate-50 last:border-0 text-xs font-mono">
                    <div className="flex items-center gap-4">
                      <span className="text-slate-400">{log.time}</span>
                      <span className="text-red-500 font-bold">DROP</span>
                      <span className="text-slate-700">{log.source} {"->"} {log.dest}:{log.port}</span>
                    </div>
                    <span className="px-1.5 py-0.5 bg-red-50 text-red-600 rounded text-[10px] font-bold">{log.reason}</span>
                 </div>
               ))}
            </div>
          </Widget>
        </div>
      </div>
    </div>
  );
};

export default SecurityIntel;
