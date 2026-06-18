import React from 'react';
import {
  Cpu,
  Database,
  Terminal,
  Network,
  Shield,
  ShieldAlert,
  Globe,
  HardDrive,
  Activity,
  ArrowUpRight,
  Clock,
  Server
} from 'lucide-react';
import { Line, Pie } from 'react-chartjs-2';
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
  ArcElement
} from 'chart.js';
import Widget from '../components/widgets/Widget';
import ThroughputWidget from '../components/widgets/ThroughputWidget';
import DistributionWidget from '../components/widgets/DistributionWidget';
import WorldMapWidget from '../components/widgets/WorldMapWidget';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
  ArcElement
);

const SectionHeader = ({ title }) => (
  <div className="flex items-center gap-2 mb-3 mt-6 first:mt-0">
    <div className="h-px bg-slate-200 flex-1"></div>
    <span className="text-[10px] font-black text-slate-400 uppercase tracking-[0.2em]">{title}</span>
    <div className="h-px bg-slate-200 flex-1"></div>
  </div>
);

const StatCard = ({ label, value, icon: Icon, trend }) => (
  <div className="bg-white p-3 rounded-lg border border-slate-200 shadow-sm">
    <div className="flex items-center justify-between mb-1">
      <span className="text-[8px] font-bold text-slate-400 uppercase tracking-wider">{label}</span>
      {Icon && <Icon size={12} className="text-slate-400" />}
    </div>
    <div className="text-lg font-black text-slate-700 leading-none">{value}</div>
    {trend && (
      <div className={`text-[8px] font-bold mt-1 ${trend > 0 ? 'text-green-500' : 'text-red-500'}`}>
        {trend > 0 ? '↑' : '↓'} {Math.abs(trend)}% from last hour
      </div>
    )}
  </div>
);

const Dashboard = () => {
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

  const protocolData = {
    labels: ['TCP', 'UDP', 'ICMP'],
    datasets: [{
      data: [618, 112, 45],
      backgroundColor: ['#3b82f6', '#10b981', '#f59e0b'],
      borderWidth: 0,
    }],
  };

  const portData = {
    labels: ['22', '443', '3389', '80'],
    datasets: [{
      data: [298, 412, 122, 95],
      backgroundColor: ['#ef4444', '#3b82f6', '#f59e0b', '#10b981'],
      borderWidth: 0,
    }],
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
    <div className="space-y-4 max-w-[1600px] mx-auto pb-8">
      <div className="flex items-center justify-between mb-2">
        <h1 className="text-xl font-bold text-slate-900 tracking-tight">Security Dashboard</h1>
        <div className="flex gap-2">
          <span className="flex items-center gap-1 px-2 py-0.5 bg-green-50 text-green-700 text-[10px] font-bold rounded border border-green-100 uppercase">
            System Online
          </span>
        </div>
      </div>

      {/* Hardware Section */}
      <SectionHeader title="Hardware" />
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <div className="lg:col-span-3">
          <StatCard label="Model" value="Songbird FW-100D" icon={Server} />
        </div>
        <div className="lg:col-span-3">
          <StatCard label="Uptime" value="12d 4h 33m" icon={Clock} />
        </div>
        <div className="lg:col-span-6">
          <Widget title="Resource Utilization" icon={Activity}>
             <div className="p-3 grid grid-cols-2 gap-6">
                <div className="space-y-2">
                   <div className="flex justify-between text-[8px] font-bold uppercase">
                      <span className="text-slate-400">CPU Usage</span>
                      <span className="text-slate-700">24%</span>
                   </div>
                   <div className="h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full bg-blue-500" style={{ width: '24%' }}></div>
                   </div>
                </div>
                <div className="space-y-2">
                   <div className="flex justify-between text-[8px] font-bold uppercase">
                      <span className="text-slate-400">Memory Usage</span>
                      <span className="text-slate-700">42%</span>
                   </div>
                   <div className="h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full bg-green-500" style={{ width: '42%' }}></div>
                   </div>
                </div>
             </div>
          </Widget>
        </div>
      </div>

      {/* Firewall Section */}
      <SectionHeader title="Firewall" />
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <div className="lg:col-span-2 space-y-4">
           <StatCard label="Blocked Events" value="1,018" icon={ShieldAlert} trend={12} />
           <StatCard label="Top Attacker" value="185.122.2.1" icon={Globe} />
        </div>
        <div className="lg:col-span-6">
           <WorldMapWidget />
        </div>
        <div className="lg:col-span-4 grid grid-cols-2 gap-4">
           <DistributionWidget title="Protocols" icon={Shield} data={protocolData} />
           <DistributionWidget title="Ports" icon={ShieldAlert} data={portData} />
        </div>
      </div>

      {/* Network Stats Section */}
      <SectionHeader title="Network Stats" />
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        <div className="lg:col-span-8">
           <ThroughputWidget data={throughputData} options={chartOptions} />
        </div>
        <div className="lg:col-span-4">
           <Widget title="Interface Summary" icon={Network}>
              <table className="w-full text-left text-[10px]">
                 <thead className="bg-slate-50 font-bold text-slate-400 uppercase border-b border-slate-100">
                    <tr>
                       <th className="px-3 py-1.5">Iface</th>
                       <th className="px-3 py-1.5">IP</th>
                       <th className="px-3 py-1.5">Status</th>
                    </tr>
                 </thead>
                 <tbody className="divide-y divide-slate-50">
                    <tr>
                       <td className="px-3 py-1.5 font-bold">WAN</td>
                       <td className="px-3 py-1.5 text-slate-500">203.0.113.42</td>
                       <td className="px-3 py-1.5 text-green-500 font-bold">UP</td>
                    </tr>
                    <tr>
                       <td className="px-3 py-1.5 font-bold">LAN</td>
                       <td className="px-3 py-1.5 text-slate-500">192.168.1.1</td>
                       <td className="px-3 py-1.5 text-green-500 font-bold">UP</td>
                    </tr>
                    <tr>
                       <td className="px-3 py-1.5 font-bold">DMZ</td>
                       <td className="px-3 py-1.5 text-slate-500">10.0.40.1</td>
                       <td className="px-3 py-1.5 text-slate-400 font-bold">DOWN</td>
                    </tr>
                 </tbody>
              </table>
           </Widget>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
