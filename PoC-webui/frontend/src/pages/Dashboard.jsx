import React from 'react';
import {
  Activity,
  Cpu,
  Database,
  Clock,
  Terminal,
  Wifi,
  Power,
  AlertCircle,
  BarChart3
} from 'lucide-react';
import { Line } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
);

const Dashboard = () => {
  const throughputData = {
    labels: Array.from({ length: 20 }, (_, i) => i),
    datasets: [
      {
        label: 'WAN In',
        data: Array.from({ length: 20 }, () => Math.floor(Math.random() * 500) + 100),
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
      },
      {
        label: 'WAN Out',
        data: Array.from({ length: 20 }, () => Math.floor(Math.random() * 200) + 50),
        borderColor: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
      },
    ],
  };

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { display: false },
      y: {
        beginAtZero: true,
        grid: { color: '#f1f5f9' },
        ticks: { font: { size: 10 }, color: '#94a3b8' }
      }
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">System Dashboard</h1>
        <div className="flex gap-2">
          <span className="flex items-center gap-1.5 px-3 py-1 bg-green-50 text-green-700 text-xs font-bold rounded border border-green-100">
            <div className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse"></div>
            SYSTEM READY
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* System Information Widget */}
        <div className="lg:col-span-4 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
              <Cpu size={14} /> System Information
            </h3>
          </div>
          <div className="p-4 space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <p className="text-[10px] text-slate-400 font-bold uppercase">Model</p>
                <p className="text-sm font-medium text-slate-700">Songbird FW-100D</p>
              </div>
              <div className="space-y-1">
                <p className="text-[10px] text-slate-400 font-bold uppercase">Uptime</p>
                <p className="text-sm font-medium text-slate-700">12d 4h 33m</p>
              </div>
              <div className="space-y-1">
                <p className="text-[10px] text-slate-400 font-bold uppercase">CPU Usage</p>
                <div className="flex items-center gap-2">
                  <div className="flex-1 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                    <div className="h-full bg-blue-500 rounded-full" style={{ width: '24%' }}></div>
                  </div>
                  <span className="text-xs font-bold text-slate-600">24%</span>
                </div>
              </div>
              <div className="space-y-1">
                <p className="text-[10px] text-slate-400 font-bold uppercase">Memory</p>
                <div className="flex items-center gap-2">
                  <div className="flex-1 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                    <div className="h-full bg-blue-500 rounded-full" style={{ width: '42%' }}></div>
                  </div>
                  <span className="text-xs font-bold text-slate-600">42%</span>
                </div>
              </div>
            </div>
            <div className="pt-4 border-t border-slate-50">
              <div className="flex items-center justify-between text-xs text-slate-500">
                <span>Firmware Version</span>
                <span className="font-mono font-bold text-slate-700">v0.1.1-stable</span>
              </div>
            </div>
          </div>
        </div>

        {/* Interface Throughput Widget */}
        <div className="lg:col-span-8 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
              <Wifi size={14} /> Interface Throughput (Kbps)
            </h3>
            <div className="flex gap-4">
               <div className="flex items-center gap-1.5 text-[10px] text-slate-500 font-bold uppercase">
                 <div className="w-2 h-2 rounded-full bg-blue-500"></div> WAN In
               </div>
               <div className="flex items-center gap-1.5 text-[10px] text-slate-500 font-bold uppercase">
                 <div className="w-2 h-2 rounded-full bg-green-500"></div> WAN Out
               </div>
            </div>
          </div>
          <div className="p-4 h-[180px]">
            <Line data={throughputData} options={chartOptions} />
          </div>
        </div>

        {/* Services Status Widget */}
        <div className="lg:col-span-5 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-100 bg-slate-50/50">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
              <Database size={14} /> Services Status
            </h3>
          </div>
          <div className="p-0">
            {[
              { name: 'Firewall Engine', status: 'Running', uptime: '12d 4h', color: 'text-green-500' },
              { name: 'Intrusion Detection', status: 'Running', uptime: '5d 22h', color: 'text-green-500' },
              { name: 'VPN Gateway', status: 'Standby', uptime: '-', color: 'text-slate-400' },
              { name: 'DHCP Server', status: 'Running', uptime: '12d 4h', color: 'text-green-500' },
              { name: 'DNS Forwarder', status: 'Error', uptime: '0m', color: 'text-red-500' },
            ].map((svc) => (
              <div key={svc.name} className="flex items-center justify-between px-4 py-3 border-b border-slate-50 last:border-0 hover:bg-slate-50 transition-colors">
                <span className="text-sm font-medium text-slate-700">{svc.name}</span>
                <div className="flex items-center gap-4">
                  <span className="text-[10px] text-slate-400 font-bold uppercase">{svc.uptime}</span>
                  <span className={`text-[10px] font-black uppercase tracking-tighter ${svc.color}`}>{svc.status}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Live Logs Widget */}
        <div className="lg:col-span-7 bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden flex flex-col">
          <div className="px-4 py-3 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
              <Terminal size={14} /> Live Firewall Logs
            </h3>
            <button className="text-[10px] font-bold text-blue-600 uppercase hover:underline">View All</button>
          </div>
          <div className="flex-1 bg-slate-900 font-mono text-[11px] p-4 text-slate-300 space-y-1 overflow-y-auto max-h-[300px]">
            <p><span className="text-slate-500">[14:45:01]</span> <span className="text-green-400">PASS</span> 192.168.1.10:54223 {"->"} 8.8.8.8:53 (UDP)</p>
            <p><span className="text-slate-500">[14:45:02]</span> <span className="text-red-400">DROP</span> 45.12.33.19:1232 {"->"} 192.168.1.1:22 (TCP) [GEO-BLOCK]</p>
            <p><span className="text-slate-500">[14:45:04]</span> <span className="text-green-400">PASS</span> 192.168.1.15:80 {"->"} 203.0.113.10:443 (TCP)</p>
            <p><span className="text-slate-500">[14:45:07]</span> <span className="text-yellow-400">WARN</span> 10.0.4.55:3321 {"->"} 1.1.1.1:53 (UDP) [ICMP RATE LIMIT]</p>
            <p><span className="text-slate-500">[14:45:10]</span> <span className="text-red-400">DROP</span> 185.122.2.1:54332 {"->"} 192.168.1.100:3389 (TCP) [MALICIOUS]</p>
            <p><span className="text-slate-500">[14:45:12]</span> <span className="text-green-400">PASS</span> 192.168.1.10:44321 {"->"} 142.250.1.1:443 (TCP)</p>
            <p><span className="text-slate-500">[14:45:15]</span> <span className="text-green-400">PASS</span> 192.168.1.20:51123 {"->"} 10.0.1.55:80 (TCP)</p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
