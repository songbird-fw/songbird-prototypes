import React from 'react';
import { Network, Activity, ArrowUpRight, ArrowDownLeft } from 'lucide-react';
import ThroughputWidget from '../components/widgets/ThroughputWidget';
import Widget from '../components/widgets/Widget';

const NetworkOverview = () => {
  const throughputData = {
    labels: Array.from({ length: 20 }, (_, i) => i),
    datasets: [
      {
        label: 'WAN In',
        data: [120, 150, 180, 450, 410, 380, 420, 480, 510, 550, 520, 490, 450, 420, 410, 450, 480, 510, 550, 580],
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true, tension: 0.4, pointRadius: 0,
      },
      {
        label: 'WAN Out',
        data: [50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 130, 120, 110, 100, 90, 80, 70, 60, 50, 60],
        borderColor: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: true, tension: 0.4, pointRadius: 0,
      },
    ],
  };

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { display: false },
      y: { beginAtZero: true, grid: { color: '#f1f5f9' }, ticks: { font: { size: 10 }, color: '#94a3b8' } }
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">Network Overview</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-white p-4 rounded-lg border border-slate-200 shadow-sm">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[10px] font-bold text-slate-400 uppercase">Active Sessions</span>
            <Activity size={14} className="text-blue-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900">1,284</div>
          <div className="text-[10px] text-green-500 font-bold mt-1 flex items-center gap-1">
            <ArrowUpRight size={10} /> +12% from last hour
          </div>
        </div>
        {/* Add more metric cards if needed */}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        <div className="lg:col-span-12">
          <ThroughputWidget data={throughputData} options={chartOptions} />
        </div>

        <div className="lg:col-span-6">
          <Widget title="Interfaces Summary" icon={Network}>
             <table className="w-full text-left text-sm">
               <thead className="bg-slate-50 text-[10px] font-bold text-slate-400 uppercase">
                 <tr>
                   <th className="px-4 py-2">Interface</th>
                   <th className="px-4 py-2">Status</th>
                   <th className="px-4 py-2">IP Address</th>
                   <th className="px-4 py-2">Throughput</th>
                 </tr>
               </thead>
               <tbody className="divide-y divide-slate-100">
                 <tr>
                   <td className="px-4 py-3 font-medium text-slate-700">WAN1</td>
                   <td className="px-4 py-3"><span className="px-1.5 py-0.5 bg-green-100 text-green-700 rounded text-[10px] font-bold">UP</span></td>
                   <td className="px-4 py-3 text-slate-500 font-mono text-xs">203.0.113.42</td>
                   <td className="px-4 py-3 text-slate-500 text-xs">580 Kbps</td>
                 </tr>
                 <tr>
                   <td className="px-4 py-3 font-medium text-slate-700">LAN</td>
                   <td className="px-4 py-3"><span className="px-1.5 py-0.5 bg-green-100 text-green-700 rounded text-[10px] font-bold">UP</span></td>
                   <td className="px-4 py-3 text-slate-500 font-mono text-xs">192.168.1.1</td>
                   <td className="px-4 py-3 text-slate-500 text-xs">420 Kbps</td>
                 </tr>
               </tbody>
             </table>
          </Widget>
        </div>
      </div>
    </div>
  );
};

export default NetworkOverview;
