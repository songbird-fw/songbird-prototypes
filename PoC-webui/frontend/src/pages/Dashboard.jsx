import React from 'react';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
  ArcElement,
} from 'chart.js';
import { Line, Bar, Doughnut } from 'react-chartjs-2';
import { Activity, ShieldAlert, Zap, Globe } from 'lucide-react';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
);

const Dashboard = () => {
  const lineData = {
    labels: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'],
    datasets: [
      {
        label: 'Blocked Attacks',
        data: [120, 190, 300, 250, 420, 310],
        borderColor: 'rgb(239, 68, 68)',
        backgroundColor: 'rgba(239, 68, 68, 0.5)',
        tension: 0.4,
      },
      {
        label: 'Allowed Traffic (GB)',
        data: [45, 52, 85, 92, 110, 88],
        borderColor: 'rgb(59, 130, 246)',
        backgroundColor: 'rgba(59, 130, 246, 0.5)',
        tension: 0.4,
      },
    ],
  };

  const barData = {
    labels: ['TCP', 'UDP', 'ICMP', 'HTTP', 'HTTPS', 'DNS'],
    datasets: [
      {
        label: 'Traffic Distribution',
        data: [65, 45, 12, 88, 120, 35],
        backgroundColor: 'rgba(99, 102, 241, 0.8)',
      },
    ],
  };

  const doughnutData = {
    labels: ['Safe', 'Suspicious', 'Malicious'],
    datasets: [
      {
        data: [85, 10, 5],
        backgroundColor: [
          'rgba(34, 197, 94, 0.8)',
          'rgba(234, 179, 8, 0.8)',
          'rgba(239, 68, 68, 0.8)',
        ],
      },
    ],
  };

  const stats = [
    { name: 'Total Requests', value: '1.2M', icon: <Activity className="text-blue-500" />, change: '+12%' },
    { name: 'Blocked Threats', value: '14,203', icon: <ShieldAlert className="text-red-500" />, change: '+5%' },
    { name: 'Avg Latency', value: '12ms', icon: <Zap className="text-yellow-500" />, change: '-2ms' },
    { name: 'Active Geo-Blocks', value: '42', icon: <Globe className="text-purple-500" />, change: '0' },
  ];

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Network Overview</h1>
        <p className="text-slate-500">Real-time firewall monitoring and statistics.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {stats.map((stat) => (
          <div key={stat.name} className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
            <div className="flex items-center justify-between">
              <div className="p-2 bg-slate-50 rounded-lg">{stat.icon}</div>
              <span className={`text-xs font-medium px-2 py-1 rounded-full ${
                stat.change.startsWith('+') ? 'bg-green-100 text-green-700' :
                stat.change === '0' ? 'bg-slate-100 text-slate-700' : 'bg-blue-100 text-blue-700'
              }`}>
                {stat.change}
              </span>
            </div>
            <div className="mt-4">
              <h3 className="text-sm font-medium text-slate-500">{stat.name}</h3>
              <p className="text-2xl font-bold text-slate-900">{stat.value}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
          <h3 className="text-lg font-semibold mb-6">Traffic Analysis</h3>
          <Line data={lineData} options={{ responsive: true, maintainAspectRatio: true }} />
        </div>
        <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
          <h3 className="text-lg font-semibold mb-6">Protocol Distribution</h3>
          <Bar data={barData} options={{ responsive: true, maintainAspectRatio: true }} />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm flex flex-col items-center">
          <h3 className="text-lg font-semibold mb-6 w-full text-left">Threat Assessment</h3>
          <div className="w-full max-w-[250px]">
            <Doughnut data={doughnutData} />
          </div>
        </div>
        <div className="lg:col-span-2 bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
          <h3 className="text-lg font-semibold mb-4">Recent Security Events</h3>
          <div className="space-y-4">
            {[
              { time: '14:22:01', event: 'DDoS mitigation triggered', source: '192.168.1.104', status: 'Blocked' },
              { time: '14:15:33', event: 'Port scan detected', source: '45.12.33.19', status: 'Blocked' },
              { time: '14:02:12', event: 'SQL Injection attempt', source: '10.0.4.55', status: 'Flagged' },
            ].map((event, i) => (
              <div key={i} className="flex items-center justify-between py-3 border-b border-slate-100 last:border-0">
                <div>
                  <p className="text-sm font-medium text-slate-900">{event.event}</p>
                  <p className="text-xs text-slate-500">{event.time} • Source: {event.source}</p>
                </div>
                <span className={`text-xs font-semibold px-2.5 py-0.5 rounded-full ${
                  event.status === 'Blocked' ? 'bg-red-100 text-red-700' : 'bg-yellow-100 text-yellow-700'
                }`}>
                  {event.status}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
