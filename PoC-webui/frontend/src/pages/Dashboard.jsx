import React from 'react';
import {
  Cpu,
  Database,
  Terminal,
  Network,
  Shield,
  ShieldAlert,
  Globe,
  HardDrive
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
import GaugeWidget from '../components/widgets/GaugeWidget';
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
    <div className="space-y-4 max-h-[calc(100vh-4rem)]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-slate-900 tracking-tight">Security Overview</h1>
        <div className="flex gap-2">
          <span className="flex items-center gap-1 px-2 py-0.5 bg-green-50 text-green-700 text-[10px] font-bold rounded border border-green-100">
            <div className="w-1 h-1 bg-green-500 rounded-full animate-pulse"></div>
            FW-ENGINE: RUNNING
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Row 1: System and Throughput */}
        <div className="lg:col-span-3">
          <Widget title="System Info" icon={Cpu}>
            <div className="p-3 space-y-3">
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <p className="text-[8px] text-slate-400 font-bold uppercase">Uptime</p>
                  <p className="text-xs font-bold text-slate-700">12d 4h 33m</p>
                </div>
                <div>
                  <p className="text-[8px] text-slate-400 font-bold uppercase">Firmware</p>
                  <p className="text-xs font-bold text-slate-700">v0.1.1</p>
                </div>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between text-[8px] font-bold uppercase">
                  <span className="text-slate-400">Memory</span>
                  <span className="text-slate-600">42%</span>
                </div>
                <div className="h-1 bg-slate-100 rounded-full overflow-hidden">
                  <div className="h-full bg-blue-500" style={{ width: '42%' }}></div>
                </div>
              </div>
            </div>
          </Widget>
        </div>

        <div className="lg:col-span-6">
          <ThroughputWidget data={throughputData} options={chartOptions} />
        </div>

        <div className="lg:col-span-3">
          <GaugeWidget title="CPU Load" value={24} color="bg-blue-500" />
        </div>

        {/* Row 2: Security & Map */}
        <div className="lg:col-span-3">
          <DistributionWidget title="Blocked Protocols" icon={Shield} data={protocolData} />
        </div>

        <div className="lg:col-span-6">
          <WorldMapWidget />
        </div>

        <div className="lg:col-span-3">
           <Widget title="Service Status" icon={Database}>
             <div className="p-0">
               {[
                 { name: 'Firewall Engine', status: 'Running', color: 'text-green-500' },
                 { name: 'IDS/IPS', status: 'Running', color: 'text-green-500' },
                 { name: 'DHCP Server', status: 'Running', color: 'text-green-500' },
                 { name: 'DNS Forwarder', status: 'Error', color: 'text-red-500' },
               ].map((svc) => (
                 <div key={svc.name} className="flex items-center justify-between px-3 py-2 border-b border-slate-50 last:border-0">
                   <span className="text-[11px] font-medium text-slate-600">{svc.name}</span>
                   <span className={`text-[9px] font-black uppercase tracking-tighter ${svc.color}`}>{svc.status}</span>
                 </div>
               ))}
             </div>
           </Widget>
        </div>

        {/* Row 3: Logs */}
        <div className="lg:col-span-12">
          <Widget
            title="Live Security Events"
            icon={Terminal}
            headerAction={<button className="text-[9px] font-bold text-blue-600 uppercase hover:underline">View All</button>}
          >
            <div className="bg-slate-900 font-mono text-[10px] p-3 text-slate-300 space-y-1 h-[120px] overflow-y-auto">
              <p><span className="text-slate-500">[14:45:12]</span> <span className="text-green-400">PASS</span> 192.168.1.10:44321 {"->"} 142.250.1.1:443 (TCP)</p>
              <p><span className="text-slate-500">[14:45:10]</span> <span className="text-red-400">DROP</span> 185.122.2.1:54332 {"->"} 192.168.1.100:3389 (TCP) [MALICIOUS]</p>
              <p><span className="text-slate-500">[14:45:07]</span> <span className="text-yellow-400">WARN</span> 10.0.4.55:3321 {"->"} 1.1.1.1:53 (UDP) [RATE-LIMIT]</p>
              <p><span className="text-slate-500">[14:45:02]</span> <span className="text-red-400">DROP</span> 45.12.33.19:1232 {"->"} 192.168.1.1:22 (TCP) [GEO-BLOCK]</p>
              <p><span className="text-slate-500">[14:44:58]</span> <span className="text-green-400">PASS</span> 192.168.1.15:80 {"->"} 203.0.113.10:443 (TCP)</p>
            </div>
          </Widget>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
